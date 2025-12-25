package api

import (
	appconfig "plaud-emails/pkg/config"
	"plaud-emails/service/mindadvisor"
	usersvc "plaud-emails/service/user"

	scaffoldconfig "github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	dbpkg "github.com/Plaud-AI/plaud-go-scaffold/pkg/db"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/etcd"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/middleware"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/rdb"
)

// Services API服务依赖的服务
type Services interface {
	GetAppConfigGetter() scaffoldconfig.AppConfigGetter[*appconfig.AppConfig]
	GetRedisClient() *rdb.Client
	GetDBClient() *dbpkg.Client
	GetUserService() *usersvc.UserService
	GetMindAdvisorService() *mindadvisor.MindAdvisorService
	GetJwtAuther() *middleware.JWTAuthMiddleware
	GetServiceRegistry() *etcd.ServiceRegistry
}
