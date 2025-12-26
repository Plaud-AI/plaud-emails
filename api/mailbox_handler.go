package api

import (
	"errors"
	"net/http"

	"plaud-emails/data/dto"
	"plaud-emails/service/mindadvisor"

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
	LocalPart  string `json:"local_part" binding:"required"`
	Salutation string `json:"salutation" binding:"required"`
}

// CreateMailbox 创建专属邮箱
// POST /myplaud/mailbox/create
func (h *MailboxHandler) CreateMailbox(c *gin.Context) {
	var req CreateMailboxReq
	if err := c.ShouldBindJSON(&req); err != nil {
		FailResponse(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// 从鉴权中间件获取 user_id
	userID := GetUserID(c)
	if userID == "" {
		FailResponse(c, http.StatusUnauthorized, "unauthorized: missing user_id")
		return
	}

	user, err := h.svc.CreateMailbox(c.Request.Context(), userID, req.LocalPart, req.Salutation)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "create mailbox error: %v", err)

		// 根据错误类型返回不同的状态码
		switch {
		case errors.Is(err, mindadvisor.ErrMailboxConflict):
			FailResponse(c, http.StatusConflict, "mailbox already created with different local_part")
			return
		case errors.Is(err, mindadvisor.ErrEmailAlreadyExists):
			FailResponse(c, http.StatusConflict, "email address already taken")
			return
		case errors.Is(err, mindadvisor.ErrInvalidLocalPartLength),
			errors.Is(err, mindadvisor.ErrInvalidLocalPartChars),
			errors.Is(err, mindadvisor.ErrReservedWord),
			errors.Is(err, mindadvisor.ErrInvalidSalutation):
			FailResponse(c, http.StatusBadRequest, err.Error())
			return
		default:
			FailResponse(c, http.StatusInternalServerError, "create mailbox failed")
			return
		}
	}

	mailbox := dto.NewMailboxFromModel(user)
	SuccessResponse(c, mailbox)
}

// GetMailbox 获取用户的专属邮箱
// GET /myplaud/mailbox?user_id=xxx
func (h *MailboxHandler) GetMailbox(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		FailResponse(c, http.StatusBadRequest, "user_id is required")
		return
	}

	user, err := h.svc.GetMailbox(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "get mailbox error: %v", err)
		FailResponse(c, http.StatusInternalServerError, "get mailbox failed")
		return
	}

	// 未找到返回失败
	if user == nil {
		FailResponseWithStatus(c, http.StatusOK, -1, "not found")
		return
	}

	mailbox := dto.NewMailboxFromModel(user)
	SuccessResponse(c, mailbox)
}

// GetUserByEmail 根据专属邮箱查询 user_id
// GET /myplaud/user?email=xxx@myplaud
func (h *MailboxHandler) GetUserByEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		FailResponse(c, http.StatusBadRequest, "email is required")
		return
	}

	user, err := h.svc.GetUserByDedicatedEmail(c.Request.Context(), email)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "get user by email error: %v", err)
		FailResponse(c, http.StatusInternalServerError, "get user failed")
		return
	}

	// 未找到返回失败
	if user == nil {
		FailResponseWithStatus(c, http.StatusOK, -1, "not found")
		return
	}

	SuccessResponse(c, map[string]string{"user_id": user.UserID})
}

// LinkedEmailStatusResp 绑定邮箱状态响应
type LinkedEmailStatusResp struct {
	Linked bool `json:"linked"`
}

// GetLinkedEmailStatus 检查用户是否已绑定邮箱
// GET /v1/myplaud/linked-email/status
func (h *MailboxHandler) GetLinkedEmailStatus(c *gin.Context) {
	// 从中间件获取用户信息
	userID := GetUserID(c)
	if userID == "" {
		FailResponse(c, http.StatusUnauthorized, "unauthorized: missing user_id")
		return
	}

	linked, err := h.svc.HasLinkedEmail(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "check linked email status error: %v", err)
		FailResponse(c, http.StatusInternalServerError, "check linked email status failed")
		return
	}

	SuccessResponse(c, &LinkedEmailStatusResp{Linked: linked})
}
