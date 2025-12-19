package ginutil

import (
	"github.com/Plaud-AI/plaud-library-go/env"
	"github.com/gin-gonic/gin"
)

// SetGinMode 根据环境设置Gin模式
func SetGinMode() {
	if env.GetEnv() == env.DevelopEnv {
		gin.SetMode(gin.DebugMode)
	} else if env.GetEnv() == env.TestEnv {
		gin.SetMode(gin.TestMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
}
