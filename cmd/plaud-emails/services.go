package main

import (
	"context"

	"plaud-emails/external/helloservice"
	appconfig "plaud-emails/pkg/config"
	"plaud-emails/service/mindadvisor"
	"plaud-emails/service/rpc/server"
	"plaud-emails/service/user"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/app"

	"google.golang.org/grpc"
)

type Services struct {
	*app.Services[*appconfig.AppConfig]
	UserService        *user.UserService
	MindAdvisorService *mindadvisor.MindAdvisorService
}

func (p *Services) GetUserService() *user.UserService {
	return p.UserService
}

func (p *Services) GetMindAdvisorService() *mindadvisor.MindAdvisorService {
	return p.MindAdvisorService
}

// BuildBizServices 构建业务服务
func BuildBizServices(ctx context.Context, services *app.Services[*appconfig.AppConfig]) (*Services, error) {
	userService, err := user.New(services.DBClient.GetDB(), services.Snowflake)
	if err != nil {
		return nil, err
	}

	mindAdvisorService := mindadvisor.New(services.DBClient.GetDB())

	return &Services{
		Services:           services,
		UserService:        userService,
		MindAdvisorService: mindAdvisorService,
	}, nil
}

// InitGRPCServices 初始化 gRPC 服务, 返回注册函数
func InitGRPCServices(ctx context.Context, services *app.Services[*appconfig.AppConfig]) (app.GRPCServiceRegFunc, error) {
	helloServiceServer := server.NewHelloServiceServer()
	return func(s *grpc.Server) error {
		helloservice.RegisterHelloServiceServer(s, helloServiceServer)
		return nil
	}, nil
}
