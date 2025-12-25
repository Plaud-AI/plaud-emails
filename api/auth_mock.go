package api

import (
	"context"
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
// TODO: 替换为真实的鉴权服务实现
type AuthService interface {
	// ValidateToken 验证 token 并返回用户信息
	ValidateToken(ctx context.Context, token string) (*AuthUserInfo, error)
}

// MockAuthService Mock 鉴权服务
type MockAuthService struct{}

// NewMockAuthService 创建 MockAuthService
func NewMockAuthService() *MockAuthService {
	return &MockAuthService{}
}

// ValidateToken Mock 实现：从 token 中解析用户信息
// TODO: 替换为调用真实的鉴权接口
func (s *MockAuthService) ValidateToken(ctx context.Context, token string) (*AuthUserInfo, error) {
	// Mock: 这里应该调用真实的鉴权服务接口
	// 例如: resp, err := http.Get("http://auth-service/validate?token=" + token)
	// 当前 Mock 实现：假设 token 格式为 "user_id:email"
	// 实际生产环境应该调用真实的鉴权 API

	// 模拟调用外部鉴权接口
	// TODO: 替换为真实的 API 调用
	// userInfo, err := callAuthAPI(ctx, token)
	// if err != nil {
	//     return nil, err
	// }
	// return userInfo, nil

	// Mock: 暂时返回 nil，表示需要从 Header 获取
	return nil, nil
}

// 默认的鉴权服务实例
var defaultAuthService AuthService = NewMockAuthService()

// SetAuthService 设置鉴权服务（用于依赖注入）
func SetAuthService(svc AuthService) {
	defaultAuthService = svc
}

// MockAuthMiddleware Mock 鉴权中间件
// TODO: 替换为真实的鉴权逻辑
// 当前 Mock 实现：从 Header Authorization 获取 token，调用鉴权服务验证
func MockAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从 Authorization Header 获取 token
		token := c.GetHeader("Authorization")

		var userID, userEmail string

		if token != "" {
			// 调用鉴权服务验证 token
			userInfo, err := defaultAuthService.ValidateToken(c.Request.Context(), token)
			if err != nil {
				FailResponse(c, http.StatusUnauthorized, "unauthorized: invalid token")
				c.Abort()
				return
			}
			if userInfo != nil {
				userID = userInfo.UserID
				userEmail = userInfo.Email
			}
		}

		// Mock 兜底：如果鉴权服务没有返回用户信息，从 Header 获取（仅用于测试）
		if userID == "" {
			userID = c.GetHeader("X-User-ID")
		}
		if userEmail == "" {
			userEmail = c.GetHeader("X-User-Email")
		}

		// 必须有用户信息
		if userID == "" || userEmail == "" {
			FailResponse(c, http.StatusUnauthorized, "unauthorized: missing user info")
			c.Abort()
			return
		}

		c.Set(CtxKeyUserID, userID)
		c.Set(CtxKeyUserEmail, userEmail)
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
