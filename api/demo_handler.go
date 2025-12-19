package api

import (
	"net/http"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/common"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/rdb"

	"github.com/gin-gonic/gin"
)

// DemoHandler demo
type DemoHandler struct {
	redisClient *rdb.Client
}

// NewDemoHandler
func NewDemoHandler(rdb *rdb.Client) *DemoHandler {
	return &DemoHandler{
		redisClient: rdb,
	}
}

type HelloReq struct {
	Msg string `json:"msg" form:"msg"`
}

// Index index demo
func (p *DemoHandler) Index(c *gin.Context) {
	req := &HelloReq{}
	if err := c.ShouldBind(req); err != nil {
		common.JSONFailResponse(c, err.Error(), http.StatusBadRequest)
		return
	}
	common.JSONSuccessResponse(c, "", req)
}

type HealthResp struct {
	RedisStatus string `json:"redis"`
}

// Health health check
func (p *DemoHandler) Health(c *gin.Context) {
	if err := p.redisClient.GetClient().Ping(c.Request.Context()).Err(); err != nil {
		logger.Errorf("redis ping failed: %v", err)
		return
	}
	resp := HealthResp{
		RedisStatus: "ok",
	}
	common.JSONSuccessResponse(c, "", resp)
}
