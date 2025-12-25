package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"plaud-emails/api"
	appconfig "plaud-emails/pkg/config"

	appsvc "github.com/Plaud-AI/plaud-go-scaffold/pkg/app"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/etcd"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/svc"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/telemetry"
)

func main() {
	opts := appsvc.ParseFlags("plaud-emails")

	appConfigGetter, err := appsvc.InitConfig[*appconfig.AppConfig](opts)

	if err != nil {
		logger.FatalAndExit("init config fail, err:%v", err)
	}
	var conf = appConfigGetter.GetConfig()

	// Initialize enhanced observability with metrics and logging
	shutdownObservability, err := telemetry.InitObservability(conf.AppName, conf.Observability, appConfigGetter.GetEnv())
	if err != nil {
		logger.Errorf("init observability failed: %v", err)
	}
	defer func() {
		if shutdownObservability != nil {
			shutdownObservability()
		}
	}()

	services, err := appsvc.BuildServices(context.Background(), appConfigGetter, opts)
	if err != nil {
		if services != nil {
			if err := svc.StopAll(context.Background(), services); err != nil {
				logger.Errorf("stop services fail, err:%v", err)
			}
		}
		logger.FatalAndExit("build services fail, err:%v", err)
	}

	allServices, err := BuildBizServices(context.Background(), services)
	if err != nil {
		if allServices != nil {
			if err := svc.StopAll(context.Background(), allServices); err != nil {
				logger.Errorf("stop services fail, err:%v", err)
			}
		}
		logger.FatalAndExit("build services fail, err:%v", err)
	}

	stopOnce := sync.Once{}
	stopAllServices := func() {
		stopOnce.Do(func() {
			if err := svc.StopAll(context.Background(), allServices); err != nil {
				logger.Errorf("stop services fail, err:%v", err)
			}
		})
	}
	defer stopAllServices()

	exitNow := func(msg string, args ...interface{}) {
		stopAllServices()
		logger.FatalAndExit(msg, args...)
	}

	// 初始化服务, 如果服务实现了实现Initalble接口
	if err := svc.InitAll(context.Background(), allServices); err != nil {
		exitNow("init services fail, err:%v", err)
	}

	// 启动服务, 如果服务实现了实现Startable接口
	if err := svc.StartAll(context.Background(), allServices); err != nil {
		exitNow("start services fail, err:%v", err)
	}

	publicMux, privateMux := api.InitRouter(allServices)
	errChan := make(chan error, 1)

	grpcServiceRegFunc, err := InitGRPCServices(context.Background(), services)
	if err != nil {
		exitNow("build grpc services fail, err:%v", err)
	}

	grpcServer, err := appsvc.StartGRPCServer(conf.GRPC.IP, conf.GRPC.Port, grpcServiceRegFunc, errChan)
	if err != nil {
		exitNow("start grpc server fail, err:%v", err)
	}

	if grpcServer != nil && allServices.ServiceRegistry != nil {
		serverInfo := etcd.ServiceInfo{
			Addr: conf.GRPC.IP,
			Port: conf.GRPC.Port,
		}
		for k := range grpcServer.GetServiceInfo() {
			if err := services.ServiceRegistry.Register(context.Background(), k, serverInfo); err != nil {
				exitNow("register service fail, err:%v", err)
			}
		}
	}

	publicServer := appsvc.StartHTTPServer("public", conf.Public.IP, conf.Public.Port, publicMux, errChan)
	privateServer := appsvc.StartHTTPServer("private", conf.Private.IP, conf.Private.Port, privateMux, errChan)

	if grpcServer == nil && publicServer == nil && privateServer == nil {
		exitNow("all servers are disabled")
	}

	logger.Infof("all services started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待关闭信号或错误
	select {
	case <-quit:
		logger.Infof("received shutdown signal, shutting down server...")
	case err := <-errChan:
		logger.Infof("server error: %v", err)
	}

	wg := sync.WaitGroup{}

	//关闭网络服务，不再接收请求
	if publicServer != nil {
		wg.Add(1)
		go func() { defer wg.Done(); _ = appsvc.ShutdownHTTPServer("public", publicServer) }()
	}
	if privateServer != nil {
		wg.Add(1)
		go func() { defer wg.Done(); _ = appsvc.ShutdownHTTPServer("private", privateServer) }()
	}
	if grpcServer != nil {
		wg.Add(1)
		go func() { defer wg.Done(); appsvc.ShutdownRPCServer(grpcServer) }()
	}

	wg.Wait()

	// 强制关闭
	if publicServer != nil {
		_ = appsvc.ForceCloseHTTPServer("public", publicServer)
	}
	if privateServer != nil {
		_ = appsvc.ForceCloseHTTPServer("private", privateServer)
	}
	if grpcServer != nil {
		grpcServer.Stop()
	}

	stopAllServices()

	logger.Infof("server shutdown completed")
}
