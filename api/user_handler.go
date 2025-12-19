package api

import (
	"net/http"
	"strconv"

	"plaud-emails/data/dto"
	"plaud-emails/data/model"
	"plaud-emails/external/helloservice"
	usersvc "plaud-emails/service/user"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/common"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	svc         *usersvc.UserService
	helloClient helloservice.HelloServiceClient
}

func NewUserHandler(svc *usersvc.UserService, helloClient helloservice.HelloServiceClient) *UserHandler {
	return &UserHandler{svc: svc, helloClient: helloClient}
}

type addUserReq struct {
	Name    string `json:"name" binding:"required"`
	Address string `json:"address" binding:"required"`
}

type updateUserReq struct {
	ID      int64  `json:"id" binding:"required"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (p *UserHandler) Add(c *gin.Context) {
	var req addUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid request", -1))
		return
	}

	u, err := p.svc.AddUser(c, req.Name, req.Address)
	if err != nil {
		logger.Errorf("create user error: %v", err)
		common.JSONFailResponse(c, "create failed", -1)
		return
	}
	common.JSONSuccessResponse(c, "user", dto.NewUserFromModel(u))
}

func (p *UserHandler) Get(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, common.NewFailResp("missing id", -1))
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid id", -1))
		return
	}

	u, err := p.svc.GetUser(c, id)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "get user error: %v", err)
		common.JSONFailResponse(c, "not found", -1)
		return
	}
	if !u.IsValid() {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid user", -1))
		return
	}
	common.JSONSuccessResponse(c, "user", dto.NewUserFromModel(u))
}

func (p *UserHandler) UpdateColumns(c *gin.Context) {
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid request", -1))
		return
	}
	ok, err := p.svc.UpdateUserColumns(c, req.ID, req.Name, req.Address)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "update user error: %v", err)
		common.JSONFailResponse(c, "update failed", -1)
		return
	}
	common.JSONSuccessResponse(c, "updated", ok)
}

func (p *UserHandler) UpdateUser(c *gin.Context) {
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid request", -1))
		return
	}
	u := &model.User{}
	u.ID = req.ID
	u.Name = req.Name
	u.Address = req.Address
	ok, err := p.svc.UpdateUser(c, u)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "update user error: %v", err)
		common.JSONFailResponse(c, "update failed", -1)
		return
	}
	common.JSONSuccessResponse(c, "updated", ok)
}

func (p *UserHandler) SoftDelete(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, common.NewFailResp("missing id", -1))
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid id", -1))
		return
	}
	ok, err := p.svc.SoftDeleteUser(c, id)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "delete user error: %v", err)
		common.JSONFailResponse(c, "delete failed", -1)
		return
	}
	common.JSONSuccessResponse(c, "user", ok)
}

func (p *UserHandler) Delete(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, common.NewFailResp("missing id", -1))
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid id", -1))
		return
	}
	ok, err := p.svc.DeleteUser(c, id)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "delete user error: %v", err)
		common.JSONFailResponse(c, "delete failed", -1)
		return
	}
	common.JSONSuccessResponse(c, "user", ok)
}

func (p *UserHandler) Hello(c *gin.Context) {
	if p.helloClient == nil {
		common.JSONFailResponse(c, "hello client not found", -1)
		return
	}
	msg := c.PostForm("msg")
	req := &helloservice.SayHelloRequest{
		Msg: "Hello, World!",
	}
	if msg != "" {
		req.Msg = msg
	}
	resp, err := p.helloClient.SayHello(c, req)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "hello error: %v", err)
		common.JSONFailResponse(c, "hello failed", -1)
		return
	}
	common.JSONSuccessResponse(c, "hello", resp)
}
