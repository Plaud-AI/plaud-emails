package middleware

import (
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/common"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/gin-gonic/gin"
)

// RegionAuther 区域认证器
type RegionAuther interface {
	AuthRegion(region string) error
}

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	authConfigGetter getConfigFunc
	regionAuther     RegionAuther
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(authConfigGetter getConfigFunc, regionAuther RegionAuther) *AuthMiddleware {
	return &AuthMiddleware{
		authConfigGetter: authConfigGetter,
		regionAuther:     regionAuther,
	}
}

// AuthAPIKey 认证API Key
func (p *AuthMiddleware) AuthAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p.authenticateAPIKey(c) {
			c.Next()
		} else {
			p.unauthorized(c)
		}
	}
}

// authenticateAPIKey API Key认证
func (p *AuthMiddleware) authenticateAPIKey(c *gin.Context) bool {
	if p.authConfigGetter == nil {
		return false
	}

	authConfig := p.authConfigGetter()
	if authConfig == nil {
		return false
	}

	ip := c.ClientIP()
	if authConfig.IsWhitelist(ip) {
		logger.Debugf("ip %s is in whitelist", ip)
		return true
	}

	// 从Header获取API Key
	apiKey := c.GetHeader(HeaderAPIKey)
	if apiKey == "" {
		return false
	}

	return authConfig.IsAPIKey(apiKey)
}

// unauthorized 返回未授权响应
func (p *AuthMiddleware) unauthorized(c *gin.Context) {
	common.JSONFailResponse(c, "Unauthorized", common.CodeUnauthorized)
	c.Abort()
}
