package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/common"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/config"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/rdb"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
)

type getConfigFunc func() *config.AuthConfig

// JWTAuthMiddleware 认证中间件
type JWTAuthMiddleware struct {
	authConfigGetter getConfigFunc
	redisClient      *rdb.Client
}

// NewJWTAuthMiddleware 创建认证中间件
func NewJWTAuthMiddleware(authConfigGetter getConfigFunc, redisClient *rdb.Client) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		authConfigGetter: authConfigGetter,
		redisClient:      redisClient,
	}
}

// unauthorized 返回未授权响应
func (p *JWTAuthMiddleware) unauthorized(c *gin.Context) {
	common.JSONFailResponse(c, "Unauthorized", http.StatusUnauthorized)
	c.Abort()
}

// AuthJWT JWT认证中间件
func (p *JWTAuthMiddleware) AuthJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := p.GetCurrentUser(c)
		if err != nil {
			logger.Errorf("JWT authentication failed: %v", err)
			p.unauthorized(c)
			return
		}

		// 将用户ID设置到上下文中，供后续处理器使用
		c.Set(common.KeyUserID, userID)
		c.Next()
	}
}

// GetCurrentUser 获取当前用户，基于token在redis中的验证, 返回用户ID，如果验证失败返回错误
func (p *JWTAuthMiddleware) GetCurrentUser(c *gin.Context) (string, error) {
	var accessToken string
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// 支持 Authorization: Bearer <token> 或 bearer <token>（不区分大小写）
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return "", fmt.Errorf("invalid authorization header format")
		}
		accessToken = parts[1]
	} else {
		accessToken = c.Query("access_token")
	}
	if accessToken == "" {
		return "", fmt.Errorf("authorization header or access_token is empty")
	}

	// 获取JWT payload
	payload, err := p.getJWTPayload(accessToken)
	if err != nil {
		logger.Errorf("failed to get JWT payload: %v", err)
		return "", fmt.Errorf("token invalid or expired")
	}

	// 检查user_id是否存在
	userID, ok := payload["sub"].(string)
	if !ok || userID == "" {
		logger.Errorf("JWT payload validation failed, payload: %v", payload)
		return "", fmt.Errorf("token invalid or expired")
	}

	// 利用Redis记录access_token，并允许多端登录
	rdID := fmt.Sprintf("Plaud-AI-access-%s", userID)
	rdJSONStr, err := p.redisClient.GetClient().Get(c.Request.Context(), rdID).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("failed to get access_token from redis: %v", err)
		}
		return "", fmt.Errorf("token invalid or expired")
	}

	// 解析Redis中的JSON数据
	var rdJSON map[string]interface{}
	if err := json.Unmarshal([]byte(rdJSONStr), &rdJSON); err != nil {
		logger.Errorf("failed to unmarshal redis json: %v", err)
		return "", fmt.Errorf("token invalid or expired")
	}

	// 验证token的有效性
	if _, exists := rdJSON[accessToken]; !exists {
		logger.Errorf("%s access_token not in redis", userID)
		return "", fmt.Errorf("token invalid or expired")
	}

	return userID, nil
}

// getJWTPayload 从JWT token中获取payload
func (p *JWTAuthMiddleware) getJWTPayload(tokenString string) (map[string]interface{}, error) {
	if p.authConfigGetter == nil {
		return nil, fmt.Errorf("auth config is nil")
	}

	authConfig := p.authConfigGetter()
	if authConfig == nil {
		return nil, fmt.Errorf("auth config is nil")
	}

	secret := authConfig.JWT.Secret
	if secret == "" {
		return nil, fmt.Errorf("JWT secret is empty")
	}

	// 解析JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
