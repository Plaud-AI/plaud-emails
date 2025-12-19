package app

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config/vars"
	netpkg "github.com/Plaud-AI/plaud-go-scaffold/pkg/net"

	"google.golang.org/grpc"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/aws"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	dbpkg "github.com/Plaud-AI/plaud-go-scaffold/pkg/db"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/etcd"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/middleware"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/rdb"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/snowflake"

	"github.com/Plaud-AI/plaud-library-go/env"

	_ "go.uber.org/automaxprocs"
)

func init() {
	logger.Infof("cpu num: %d, max procs: %d", runtime.NumCPU(), runtime.GOMAXPROCS(-1))
}

// Options 控制应用初始化的可配置项
type Options struct {
	AppName       string
	AppConfigName string
	ConfigMode    string // local | aws
	ServiceAddr   string

	PublicIP    string
	PublicPort  int
	GRPCIP      string
	GRPCPort    int
	PrivateIP   string
	PrivatePort int

	Environment string
}

// Services 提供对外服务的依赖集合
type Services[T config.ConfigConstraint] struct {
	AppConfigGetter config.AppConfigGetter[T]
	JwtAuther       *middleware.JWTAuthMiddleware
	RedisClient     *rdb.Client
	DBClient        *dbpkg.Client
	SecretsManager  *aws.SecretsManager
	ServiceRegistry *etcd.ServiceRegistry
	Snowflake       *snowflake.GeneratorCreater

	appName     string
	serviceAddr string
	serverID    string
}

func (p *Services[T]) GetAppConfigGetter() config.AppConfigGetter[T] { return p.AppConfigGetter }
func (p *Services[T]) GetJwtAuther() *middleware.JWTAuthMiddleware   { return p.JwtAuther }
func (p *Services[T]) GetRedisClient() *rdb.Client                   { return p.RedisClient }
func (p *Services[T]) GetDBClient() *dbpkg.Client                    { return p.DBClient }
func (p *Services[T]) GetServiceRegistry() *etcd.ServiceRegistry     { return p.ServiceRegistry }

// ParseFlags 解析通用命令行参数到 Options，并调用 flag.Parse()
func ParseFlags(defaultAppName string) Options {
	var opts Options
	flag.StringVar(&opts.AppName, "app_name", defaultAppName, "app name")
	flag.StringVar(&opts.AppConfigName, "app_config_name", "", "app config name")
	flag.StringVar(&opts.ConfigMode, "config_mode", "local", "config mode: local or aws")
	flag.StringVar(&opts.PublicIP, "public_ip", "", "public server ip")
	flag.IntVar(&opts.PublicPort, "public_port", 0, "public server port")
	flag.StringVar(&opts.GRPCIP, "grpc_ip", "", "grpc server ip")
	flag.IntVar(&opts.GRPCPort, "grpc_port", 0, "grpc server port")
	flag.StringVar(&opts.PrivateIP, "private_ip", "", "private server ip")
	flag.IntVar(&opts.PrivatePort, "private_port", 0, "private server port")
	flag.StringVar(&opts.Environment, "env", "dev", "env config")
	flag.StringVar(&opts.ServiceAddr, "service_addr", "", "expose service addr")
	flag.Parse()
	return opts
}

type onConfigChangedFunc[T config.Configurer] func(config T)

func (p onConfigChangedFunc[T]) OnConfigChanged(config T) {
	p(config)
}

// InitConfig 初始化应用配置（包含 logger 初始化与 IP/Port 覆盖），便于各个 CMD 复用
func InitConfig[T config.ConfigConstraint](opts Options) (config.AppConfigGetter[T], error) {
	var appConfigGetter config.AppConfigGetter[T]
	envName := opts.Environment
	if envName == "" {
		envName = env.GetEnv()
	} else {
		if err := env.SetEnv(envName); err != nil {
			return nil, fmt.Errorf("set env to %s fail, err:%w", envName, err)
		}
	}

	var setWithOpts = func(conf T) {
		if c, ok := any(conf).(config.PublicServerAddressGetter); ok {
			c.GetPublicServerAddress().Assign(opts.PublicIP, opts.PublicPort)
		}
		if c, ok := any(conf).(config.GRPCServerAddressGetter); ok {
			c.GetGRPCServerAddress().Assign(opts.GRPCIP, opts.GRPCPort)
		}
		if c, ok := any(conf).(config.PrivateServerAddressGetter); ok {
			c.GetPrivateServerAddress().Assign(opts.PrivateIP, opts.PrivatePort)
		}
		if opts.AppName != "" {
			conf.SetAppName(opts.AppName)
		}
		if c, ok := any(conf).(config.AWSAppNameSetter); ok {
			c.SetAwsAppName(aws.GetAWSAppConfigName())
		}
	}

	var configVarResolver = vars.NewConfigVarResolver()

	switch opts.ConfigMode {
	case "aws":
		appConfigApplicationName := aws.GetAWSAppConfigName()
		if appConfigApplicationName == "" {
			return nil, fmt.Errorf("aws AppConfig application name is empty")
		}
		configName := opts.AppConfigName
		if configName == "" {
			configName = fmt.Sprintf("%s-%s.yaml", appConfigApplicationName, envName)
		}
		appConfigLoader := aws.NewAppConfigLoader[T](
			appConfigApplicationName,
			opts.AppName,
			envName,
			configName,
		)
		appConfigLoader.SetConfigVarResolver(configVarResolver)
		logger.Infof("init app config from aws app config: %s, env: %s, config name: %s", appConfigApplicationName, envName, configName)
		if err := appConfigLoader.Init(); err != nil {
			return nil, fmt.Errorf("init app config fail, err:%v", err)
		}
		appConfigGetter = appConfigLoader

		//如果重新加载了，设置配置
		var onConfigChanged onConfigChangedFunc[T] = func(newConf T) {
			var zero T
			if newConf == zero {
				return
			}
			setWithOpts(newConf)
		}
		appConfigLoader.AddConfigListener(onConfigChanged)

	case "local":
		configName := opts.AppConfigName
		if configName == "" {
			configName = fmt.Sprintf("%s.yaml", envName)
		}
		configFile := filepath.Join("conf", configName)
		appConfigLoader := config.NewLocalConfigLoader[T](opts.AppName, envName, configFile)
		appConfigLoader.SetConfigVarResolver(configVarResolver)
		logger.Infof("init app config from local config: %s, env: %s, config name: %s", opts.AppName, envName, configFile)

		if err := appConfigLoader.Init(); err != nil {
			return nil, fmt.Errorf("init app config form %s fail, err:%v", configFile, err)
		}
		appConfigGetter = appConfigLoader

	default:
		return nil, fmt.Errorf("invalid config mode: %s", opts.ConfigMode)
	}

	conf := appConfigGetter.GetConfig()
	setWithOpts(conf)

	// 初始化logger
	if conf.GetLoggerConfig() != nil {
		loggerConfig := conf.GetLoggerConfig()
		initLoggerConfig(envName, loggerConfig)
		if err := logger.InitLogger(conf.GetLoggerConfig()); err != nil {
			return nil, fmt.Errorf("init logger fail, err:%v", err)
		}
	}
	logger.Infof("appName: %s, env: %s, configPath: %s, loadMode: %s, awsAppName: %s", conf.GetAppName(), conf.GetEnv(), conf.GetConfigPath(), conf.GetLoadMode(), conf.GetAwsAppName())
	return appConfigGetter, nil
}

// BuildServices 初始化通用依赖（Redis/DB/Snowflake/JWT/ETCD）供各 CMD 共享
func BuildServices[T config.ConfigConstraint](ctx context.Context, appConfigGetter config.AppConfigGetter[T], opts Options) (*Services[T], error) {
	var err error
	conf := appConfigGetter.GetConfig()

	serverID := ""
	if opts.ServiceAddr != "" {
		serverID = opts.ServiceAddr
	} else {
		serverID = conf.GetServerAddrs()
		if serverID == "" {
			serverID = netpkg.GetHostName()
		}
	}

	serverID = fmt.Sprintf("%s@%s-%d", conf.GetAppName(), serverID, os.Getpid())
	logger.Infof("serverID: %s", serverID)

	var redisClient *rdb.Client
	if c, ok := any(conf).(config.RedisConfigGetter); ok {
		if c.GetRedisConfig() != nil {
			if redisClient, err = rdb.NewClient(c.GetRedisConfig()); err != nil {
				return nil, fmt.Errorf("init redis fail, err:%v", err)
			}
		}
	}

	var jwtRedisClient *rdb.Client
	if c, ok := any(conf).(config.JWTRedisConfigGetter); ok {
		if c.GetJWTRedisConfig() != nil {
			if jwtRedisClient, err = rdb.NewClient(c.GetJWTRedisConfig()); err != nil {
				return nil, fmt.Errorf("init jwt redis fail, err:%v", err)
			}
		}
	}

	var registry *etcd.ServiceRegistry
	if c, ok := any(conf).(config.ETCDConfigGetter); ok {
		if c.GetETCDConfig() != nil && c.GetETCDConfig().Enable {
			registry, err = etcd.NewServiceRegistry(c.GetETCDConfig(), serverID, opts.ServiceAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to create etcd registry: %v", err)
			}
		}
	}

	var dbClient *dbpkg.Client
	if c, ok := any(conf).(config.DBConfigGetter); ok {
		if c.GetDBConfig() != nil {
			if dbClient, err = dbpkg.NewClient(c.GetDBConfig()); err != nil {
				return nil, fmt.Errorf("init db fail, err:%v", err)
			}
		}
	}

	var snowflakeCreater *snowflake.GeneratorCreater
	if c, ok := any(conf).(config.SnowflakeConfigGetter); ok {
		if c.GetSnowflakeConfig() != nil {
			snowflakeCreater = snowflake.NewCreater(c.GetSnowflakeConfig(), registry)
		}
	}

	var jwtAuther *middleware.JWTAuthMiddleware
	if _, ok := any(conf).(config.AuthConfigGetter); ok && jwtRedisClient != nil {
		// AuthConfig可能会变化，需要从appConfigGetter中获取最新的Config
		jwtAuther = middleware.NewJWTAuthMiddleware(func() *config.AuthConfig {
			c := any(appConfigGetter.GetConfig()).(config.AuthConfigGetter)
			return c.GetAuthConfig()
		}, jwtRedisClient)
	}

	var secretsManager *aws.SecretsManager
	if c, ok := any(conf).(config.KeySecretsConfigGetter); ok {
		if c.GetKeySecretsConfig() != nil {
			secretsManager, err = aws.NewSecretsManager()
			if err != nil {
				return nil, fmt.Errorf("init secrets manager fail, err:%v", err)
			}
		}
	}

	services := &Services[T]{
		RedisClient:     redisClient,
		DBClient:        dbClient,
		AppConfigGetter: appConfigGetter,
		JwtAuther:       jwtAuther,
		ServiceRegistry: registry,
		Snowflake:       snowflakeCreater,
		appName:         appConfigGetter.GetAppName(),
		serverID:        serverID,
		serviceAddr:     opts.ServiceAddr,
		SecretsManager:  secretsManager,
	}

	return services, nil
}

// StartHTTPServer 启动 HTTP 服务器（可复用）
func StartHTTPServer(name, ip string, port int, handler http.Handler, errChan chan<- error) *http.Server {
	if ip == "" || port == 0 {
		logger.Warnf("%s http server is disabled", name)
		return nil
	}
	if handler == nil {
		logger.Warnf("%s http server handler is nil", name)
		return nil
	}
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", ip, port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Infof("%s server started at http://%s:%d", name, ip, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("%s server start failed: %v", name, err)
		}
	}()

	return server
}

// ShutdownHTTPServer 优雅停止 HTTP 服务
func ShutdownHTTPServer(name string, server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	logger.Infof("shutting down %s HTTP server...", name)
	return server.Shutdown(ctx)
}

// ForceCloseHTTPServer 强制关闭 HTTP 服务
func ForceCloseHTTPServer(name string, server *http.Server) error {
	if err := server.Close(); err != nil {
		logger.Errorf("%s server force close failed: %v", name, err)
		return err
	}
	return nil
}

type GRPCServiceRegFunc func(*grpc.Server) error

// StartGRPCServer 启动 gRPC 服务器，注册回调由调用方提供以便复用
func StartGRPCServer(ip string, port int, registerFunc GRPCServiceRegFunc, errChan chan<- error) (svr *grpc.Server, err error) {
	if ip == "" || port == 0 {
		logger.Warnf("gRPC server is disabled")
		return nil, nil
	}

	grpcServer := grpc.NewServer()
	if registerFunc != nil {
		if err := registerFunc(grpcServer); err != nil {
			logger.Errorf("register gRPC services failed: %v", err)
			return nil, err
		}
	}

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
		if err != nil {
			errChan <- fmt.Errorf("failed to listen for gRPC: %v", err)
			return
		}
		logger.Infof("gRPC server started at %s:%d", ip, port)
		if err := grpcServer.Serve(lis); err != nil {
			errChan <- fmt.Errorf("gRPC server failed: %v", err)
		}
	}()

	return grpcServer, nil
}

// ShutdownRPCServer 优雅停止 gRPC 服务
func ShutdownRPCServer(server *grpc.Server) {
	logger.Infof("shutting down gRPC server...")
	server.GracefulStop()
}

// initLoggerConfig 初始化logger级别
func initLoggerConfig(envName string, logConfig *config.LogConfig) {
	if logConfig.Level == "" {
		if envName == env.DevelopEnv {
			logConfig.Level = logger.Debug.Name()
		} else {
			logConfig.Level = logger.Info.Name()
		}
	}
	if logConfig.Env == "" {
		logConfig.Env = envName
	}
}
