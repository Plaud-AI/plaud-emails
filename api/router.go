package api

import (
	"context"
	"net/http"

	"plaud-emails/external/helloservice"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/ginutil"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/middleware"

	"github.com/gin-gonic/gin"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

var (
	publicRouter  *gin.Engine
	privateRouter *gin.Engine
)

func init() {
	publicRouter = gin.New()
	privateRouter = gin.New()
	publicRouter.Use(gin.Recovery())
	privateRouter.Use(gin.Recovery())
}

// InitRouter 初始化路由
func InitRouter(services Services) (public http.Handler, private http.Handler) {
	ginutil.SetGinMode()
	appConfigGetter := services.GetAppConfigGetter()
	appName := appConfigGetter.GetConfig().AppName

	publicRouter.Use(otelgin.Middleware(appName))
	privateRouter.Use(otelgin.Middleware(appName + "-private"))

	publicRouter.Use(middleware.RequestAuditMiddleware)
	privateRouter.Use(middleware.RequestAuditMiddleware)

	var helloClient helloservice.HelloServiceClient
	if services.GetServiceRegistry() != nil {
		conn, err := services.GetServiceRegistry().DialGRPC(context.Background(), helloservice.HelloService_ServiceDesc.ServiceName)
		if err != nil {
			logger.Errorf("dial hello service error: %v", err)
		}
		helloClient = helloservice.NewHelloServiceClient(conn)
	}
	demoHandler := NewDemoHandler(services.GetRedisClient())
	userHandler := NewUserHandler(services.GetUserService(), helloClient)
	mailboxHandler := NewMailboxHandler(services.GetMindAdvisorService())
	betaHandler := NewBetaHandler(services.GetMindAdvisorService())

	// public
	publicRouter.GET("/index", demoHandler.Index)
	publicRouter.GET("/health", demoHandler.Health)

	users := publicRouter.Group("/users")
	{
		users.POST("/add", userHandler.Add)
		users.GET("/get", userHandler.Get)
		users.PUT("/update_columns", userHandler.UpdateColumns)
		users.PUT("/update_user", userHandler.UpdateUser)
		users.DELETE("/del", userHandler.SoftDelete)
		users.DELETE("/del_force", userHandler.Delete)
		users.GET("/hello_rpc", userHandler.Hello)
		users.Use(services.GetJwtAuther().AuthJWT())
		users.DELETE("/del_need_auth", userHandler.Delete)
	}

	// myplaud - 心智幕僚邮箱相关接口
	myplaud := publicRouter.Group("/v1/myplaud")
	myplaud.Use(ReqIDMiddleware())
	{
		myplaud.POST("/mailbox", mailboxHandler.CreateMailbox)
		myplaud.GET("/mailbox", mailboxHandler.GetMailbox)
		myplaud.GET("/user", mailboxHandler.GetUserByEmail)
	}

	// myplaud beta - 内测邀请登记
	beta := publicRouter.Group("/v1/myplaud/beta")
	beta.Use(ReqIDMiddleware(), MockAuthMiddleware())
	{
		beta.POST("/registration", betaHandler.CreateBetaRegistration)
		beta.GET("/registration", betaHandler.GetBetaRegistration)
	}

	// private
	privateRouter.POST("/index", demoHandler.Index)
	return publicRouter, privateRouter
}
