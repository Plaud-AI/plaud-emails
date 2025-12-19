package middleware

import (
	"net/http"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/common"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/gin-gonic/gin"
)

// RequestAuditMiddleware 记录请求耗时与异常响应
// 规则：
// - 处理时间超过5s
// - HTTP 响应码为 500
// - GetResponseStatus 不为 0
func RequestAuditMiddleware(c *gin.Context) {
	const (
		slowDuration = 5 * time.Second
	)

	startAt := time.Now()
	c.Next()
	duration := time.Since(startAt)

	statusCode := c.Writer.Status()
	needLog := false
	logLevelError := false

	var reasonFields []logger.Field

	if duration > slowDuration {
		needLog = true
		reasonFields = append(reasonFields, logger.Field{Name: "slow_request", Value: duration.Milliseconds()})
	}
	if statusCode == http.StatusInternalServerError { // 500
		needLog = true
		logLevelError = true
	} else if common.GetResponseStatus(c) != 0 {
		needLog = true
	}

	if needLog {
		ctx := c.Request.Context()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		reqFields := map[string]any{
			"method":    method,
			"path":      path,
			"client_ip": c.ClientIP(),
			"params":    c.Params,
		}
		if userID, ok := c.Get(common.KeyUserID); ok {
			reqFields["user_id"] = userID
		}
		if query := c.Request.URL.RawQuery; query != "" {
			reqFields["query"] = query
		}

		allFields := append(([]logger.Field)(nil),
			logger.Field{Name: "request", Value: reqFields},
			logger.Field{Name: "http_status", Value: statusCode},
			logger.Field{Name: "biz_status", Value: common.GetResponseStatus(c)},
		)
		allFields = append(allFields, reasonFields...)

		l := logger.WithFields(allFields)
		if logLevelError {
			l.ErrorfCtx(ctx, "request audit")
		} else {
			l.WarnfCtx(ctx, "request audit")
		}
	}
}
