package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 上下文 key
const (
	CtxKeyUserID    = "user_id"
	CtxKeyUserEmail = "user_email"
)

// AuthUserInfo 鉴权后的用户信息
type AuthUserInfo struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// AuthService 鉴权服务接口
type AuthService interface {
	// ValidateToken 验证 token 并返回用户信息
	ValidateToken(ctx context.Context, token string) (*AuthUserInfo, error)
}

// 默认的鉴权服务实例
var defaultAuthService AuthService

// ErrAuthServiceNotConfigured 鉴权服务未配置错误
var ErrAuthServiceNotConfigured = errors.New("auth service not configured, please set PLAUD_API_URL")

// SetAuthService 设置鉴权服务（用于依赖注入）
func SetAuthService(svc AuthService) {
	defaultAuthService = svc
}

// BetaAuthMiddleware Beta 路由鉴权中间件
// 从 Header Authorization 获取 token，调用 PlaudAuthService 验证
func BetaAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查鉴权服务是否已配置
		if defaultAuthService == nil {
			FailResponse(c, http.StatusInternalServerError, ErrAuthServiceNotConfigured.Error())
			c.Abort()
			return
		}

		token := c.GetHeader("Authorization")

		// 调用鉴权服务验证 token
		userInfo, err := defaultAuthService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			FailResponse(c, http.StatusUnauthorized, "unauthorized: "+err.Error())
			c.Abort()
			return
		}

		if userInfo == nil {
			FailResponse(c, http.StatusUnauthorized, "unauthorized: invalid token")
			c.Abort()
			return
		}

		// 必须有用户信息
		if userInfo.UserID == "" {
			FailResponse(c, http.StatusUnauthorized, "unauthorized: missing user_id")
			c.Abort()
			return
		}
		if userInfo.Email == "" {
			FailResponse(c, http.StatusUnauthorized, "unauthorized: missing email")
			c.Abort()
			return
		}

		c.Set(CtxKeyUserID, userInfo.UserID)
		c.Set(CtxKeyUserEmail, userInfo.Email)
		c.Next()
	}
}

// GetUserID 从上下文获取 user_id
func GetUserID(c *gin.Context) string {
	return c.GetString(CtxKeyUserID)
}

// GetUserEmail 从上下文获取 user_email
func GetUserEmail(c *gin.Context) string {
	return c.GetString(CtxKeyUserEmail)
}
