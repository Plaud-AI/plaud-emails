package api

import (
	"errors"
	"net/http"

	"plaud-emails/data/dto"
	"plaud-emails/service/mindadvisor"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/common"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/gin-gonic/gin"
)

// MailboxHandler 邮箱处理器
type MailboxHandler struct {
	svc *mindadvisor.MindAdvisorService
}

// NewMailboxHandler 创建 MailboxHandler
func NewMailboxHandler(svc *mindadvisor.MindAdvisorService) *MailboxHandler {
	return &MailboxHandler{svc: svc}
}

// CreateMailboxReq 创建邮箱请求
type CreateMailboxReq struct {
	UserID     string `json:"user_id" binding:"required"`
	LocalPart  string `json:"local_part" binding:"required"`
	Salutation string `json:"salutation" binding:"required"`
}

// CreateMailbox 创建专属邮箱
// POST /myplaud/mailbox
func (h *MailboxHandler) CreateMailbox(c *gin.Context) {
	var req CreateMailboxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.NewFailResp("invalid request: "+err.Error(), -1))
		return
	}

	userID := req.UserID

	user, err := h.svc.CreateMailbox(c.Request.Context(), userID, req.LocalPart, req.Salutation)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "create mailbox error: %v", err)

		// 根据错误类型返回不同的状态码
		switch {
		case errors.Is(err, mindadvisor.ErrMailboxConflict):
			c.JSON(http.StatusConflict, common.NewFailResp("mailbox already created with different local_part", -1))
			return
		case errors.Is(err, mindadvisor.ErrEmailAlreadyExists):
			c.JSON(http.StatusConflict, common.NewFailResp("email address already taken", -1))
			return
		case errors.Is(err, mindadvisor.ErrInvalidLocalPartLength),
			errors.Is(err, mindadvisor.ErrInvalidLocalPartChars),
			errors.Is(err, mindadvisor.ErrReservedWord),
			errors.Is(err, mindadvisor.ErrInvalidSalutation):
			c.JSON(http.StatusBadRequest, common.NewFailResp(err.Error(), -1))
			return
		default:
			c.JSON(http.StatusInternalServerError, common.NewFailResp("create mailbox failed", -1))
			return
		}
	}

	mailbox := dto.NewMailboxFromModel(user)
	c.JSON(http.StatusOK, common.NewSuccessResp("mailbox", mailbox))
}

// GetMailbox 获取用户的专属邮箱
// GET /myplaud/mailbox?user_id=xxx
func (h *MailboxHandler) GetMailbox(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, common.NewFailResp("user_id is required", -1))
		return
	}

	user, err := h.svc.GetMailbox(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "get mailbox error: %v", err)
		c.JSON(http.StatusInternalServerError, common.NewFailResp("get mailbox failed", -1))
		return
	}

	// 未创建返回 null
	if user == nil {
		c.JSON(http.StatusOK, common.NewSuccessResp("mailbox", nil))
		return
	}

	mailbox := dto.NewMailboxFromModel(user)
	c.JSON(http.StatusOK, common.NewSuccessResp("mailbox", mailbox))
}

// GetUserByEmail 根据专属邮箱查询 user_id
// GET /myplaud/user?email=xxx@myplaud
func (h *MailboxHandler) GetUserByEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, common.NewFailResp("email is required", -1))
		return
	}

	user, err := h.svc.GetUserByDedicatedEmail(c.Request.Context(), email)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "get user by email error: %v", err)
		c.JSON(http.StatusInternalServerError, common.NewFailResp("get user failed", -1))
		return
	}

	// 未找到返回失败
	if user == nil {
		common.JSONFailResponse(c, "not found", -1)
		return
	}

	common.JSONSuccessResponse(c, "user_id", user.UserID)
}
