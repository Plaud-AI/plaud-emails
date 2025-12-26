package api

import (
	"errors"
	"net/http"

	datamodel "plaud-emails/data/model"
	"plaud-emails/service/mindadvisor"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"

	"github.com/gin-gonic/gin"
)

// BetaHandler 内测登记处理器
type BetaHandler struct {
	svc *mindadvisor.MindAdvisorService
}

// NewBetaHandler 创建 BetaHandler
func NewBetaHandler(svc *mindadvisor.MindAdvisorService) *BetaHandler {
	return &BetaHandler{svc: svc}
}

// CreateBetaRegistrationReq 创建内测登记请求（请求体直接是问卷数组）
type CreateBetaRegistrationReq []datamodel.QuestionnaireItem

// BetaRegistrationResp 内测登记响应
type BetaRegistrationResp struct {
	ID        uint64 `json:"id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// CreateBetaRegistration 创建内测邀请登记
// POST /v1/myplaud/beta/registration
func (h *BetaHandler) CreateBetaRegistration(c *gin.Context) {
	var req CreateBetaRegistrationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		FailResponse(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// 从中间件获取用户信息
	userID := GetUserID(c)
	email := GetUserEmail(c)

	input := &mindadvisor.BetaRegistrationInput{
		Questionnaire: req,
	}

	reg, err := h.svc.CreateBetaRegistration(c.Request.Context(), userID, email, input)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "create beta registration error: %v", err)

		// 根据错误类型返回不同的状态码
		switch {
		case errors.Is(err, mindadvisor.ErrUserAlreadyRegistered):
			FailResponse(c, http.StatusConflict, "user already registered")
			return
		case errors.Is(err, mindadvisor.ErrEmailAlreadyRegistered):
			FailResponse(c, http.StatusConflict, "email already registered")
			return
		case errors.Is(err, mindadvisor.ErrEmptyQuestionnaire):
			FailResponse(c, http.StatusBadRequest, err.Error())
			return
		default:
			FailResponse(c, http.StatusInternalServerError, "create registration failed")
			return
		}
	}

	resp := &BetaRegistrationResp{
		ID:        reg.ID,
		UserID:    reg.UserID,
		Email:     reg.Email,
		Status:    "registered",
		CreatedAt: reg.CreatedAt.UnixMilli(),
	}

	SuccessResponse(c, resp)
}

// GetBetaRegistration 获取内测登记信息
// GET /v1/myplaud/beta/registration
func (h *BetaHandler) GetBetaRegistration(c *gin.Context) {
	// 从中间件获取用户信息
	userID := GetUserID(c)

	reg, err := h.svc.GetBetaRegistration(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "get beta registration error: %v", err)
		FailResponse(c, http.StatusInternalServerError, "get registration failed")
		return
	}

	if reg == nil {
		SuccessResponse(c, nil)
		return
	}

	resp := &BetaRegistrationResp{
		ID:        reg.ID,
		UserID:    reg.UserID,
		Email:     reg.Email,
		Status:    "registered",
		CreatedAt: reg.CreatedAt.UnixMilli(),
	}

	SuccessResponse(c, resp)
}

// BetaRegistrationStatusResp 内测登记状态响应
type BetaRegistrationStatusResp struct {
	Registered bool `json:"registered"`
}

// GetBetaRegistrationStatus 检查用户是否已登记内测
// GET /v1/myplaud/beta/registration/status
func (h *BetaHandler) GetBetaRegistrationStatus(c *gin.Context) {
	// 从中间件获取用户信息
	userID := GetUserID(c)

	registered, err := h.svc.IsBetaRegistered(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorfCtx(c.Request.Context(), "check beta registration status error: %v", err)
		FailResponse(c, http.StatusInternalServerError, "check registration status failed")
		return
	}

	SuccessResponse(c, &BetaRegistrationStatusResp{Registered: registered})
}
