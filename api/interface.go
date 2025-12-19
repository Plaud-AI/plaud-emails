package api

import (
	usersvc "plaud-emails/service/user"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	dbpkg "github.com/Plaud-AI/plaud-go-scaffold/pkg/db"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/etcd"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/middleware"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/rdb"
)

// Services API服务依赖的服务
type Services interface {
	GetAppConfigGetter() config.AppConfigGetter[*config.AppConfig]
	GetRedisClient() *rdb.Client
	GetDBClient() *dbpkg.Client
	GetUserService() *usersvc.UserService
	GetJwtAuther() *middleware.JWTAuthMiddleware
	GetServiceRegistry() *etcd.ServiceRegistry
}
