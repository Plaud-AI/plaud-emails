package api

import (
	"errors"
	"net/http"

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

// CreateBetaRegistrationReq 创建内测登记请求
type CreateBetaRegistrationReq struct {
	Role                        string   `json:"role" binding:"required"`
	RoleOther                   string   `json:"role_other,omitempty"`
	Industry                    string   `json:"industry" binding:"required"`
	IndustryOther               string   `json:"industry_other,omitempty"`
	MeetingsPerWeek             string   `json:"meetings_per_week" binding:"required"`
	PrimaryWorkingLanguage      string   `json:"primary_working_language" binding:"required"`
	PrimaryWorkingLanguageOther string   `json:"primary_working_language_other,omitempty"`
	HelpWanted                  []string `json:"help_wanted" binding:"required"`
	LinkedinURL                 string   `json:"linkedin_url,omitempty"`
}

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
		Role:                        req.Role,
		RoleOther:                   req.RoleOther,
		Industry:                    req.Industry,
		IndustryOther:               req.IndustryOther,
		MeetingsPerWeek:             req.MeetingsPerWeek,
		PrimaryWorkingLanguage:      req.PrimaryWorkingLanguage,
		PrimaryWorkingLanguageOther: req.PrimaryWorkingLanguageOther,
		HelpWanted:                  req.HelpWanted,
		LinkedinURL:                 req.LinkedinURL,
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
		case errors.Is(err, mindadvisor.ErrInvalidRole),
			errors.Is(err, mindadvisor.ErrInvalidIndustry),
			errors.Is(err, mindadvisor.ErrInvalidMeetingsPerWeek),
			errors.Is(err, mindadvisor.ErrInvalidLanguage),
			errors.Is(err, mindadvisor.ErrInvalidHelpWanted),
			errors.Is(err, mindadvisor.ErrEmptyHelpWanted):
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
