package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 上下文 key
const (
	CtxKeyReqID = "reqid"
	CtxKeyAppID = "appid"
)

// APIResp 统一响应结构（带 reqid）
type APIResp struct {
	ReqID   string      `json:"reqid"`
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    any `json:"data,omitempty"`
}

// GetReqID 从上下文获取 reqid
func GetReqID(c *gin.Context) string {
	return c.GetString(CtxKeyReqID)
}

// GetAppID 从上下文获取 appid
func GetAppID(c *gin.Context) string {
	return c.GetString(CtxKeyAppID)
}

// ReqIDMiddleware 提取 reqid 和 appid 到上下文
func ReqIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.Query("reqid")
		appID := c.Query("appid")
		c.Set(CtxKeyReqID, reqID)
		c.Set(CtxKeyAppID, appID)
		c.Next()
	}
}

// SuccessResponse 返回成功响应
func SuccessResponse(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResp{
		ReqID:   GetReqID(c),
		Status:  200,
		Message: "success",
		Data:    data,
	})
}

// FailResponse 返回失败响应
func FailResponse(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, APIResp{
		ReqID:   GetReqID(c),
		Status:  httpCode,
		Message: message,
	})
}

// FailResponseWithStatus 返回失败响应（自定义业务状态码）
func FailResponseWithStatus(c *gin.Context, httpCode int, status int, message string) {
	c.JSON(httpCode, APIResp{
		ReqID:   GetReqID(c),
		Status:  status,
		Message: message,
	})
}
