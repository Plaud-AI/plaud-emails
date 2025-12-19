package mindadvisor

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"plaud-emails/dao"
	datamodel "plaud-emails/data/model"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/svc"

	"gorm.io/gorm"
)

const (
	// EmailDomain 邮箱域名
	EmailDomain = "@myplaud"

	// LocalPart 限制
	LocalPartMinLen = 4
	LocalPartMaxLen = 20
)

// 保留词列表
var reservedWords = []string{"admin", "support", "root", "plaud", "system", "help", "info", "mail", "postmaster", "webmaster"}

// 合法字符正则：仅允许字母、数字、点
var localPartRegex = regexp.MustCompile(`^[a-z0-9.]+$`)

// 错误定义
var (
	ErrInvalidLocalPartLength   = errors.New("local_part length must be between 4 and 20 characters")
	ErrInvalidLocalPartChars    = errors.New("local_part can only contain lowercase letters, numbers, and dots")
	ErrReservedWord             = errors.New("local_part contains reserved word")
	ErrEmailAlreadyExists       = errors.New("email already exists")
	ErrUserAlreadyHasMailbox    = errors.New("user already has mailbox")
	ErrMailboxConflict          = errors.New("mailbox already created with different local_part")
	ErrInvalidSalutation        = errors.New("salutation must be Mr or Mrs")
)

// MindAdvisorService 心智幕僚服务
type MindAdvisorService struct {
	svc.BaseService
	userDao *dao.MindAdvisorUserDao
	db      *gorm.DB
}

// New 创建 MindAdvisorService
func New(db *gorm.DB) *MindAdvisorService {
	return &MindAdvisorService{
		userDao: dao.NewMindAdvisorUserDao(db),
		db:      db,
	}
}

// CreateMailbox 创建专属邮箱
func (s *MindAdvisorService) CreateMailbox(ctx context.Context, userID, localPart, salutation string) (*datamodel.MindAdvisorUser, error) {
	// 输入校验
	if err := s.validateLocalPart(localPart); err != nil {
		return nil, err
	}

	if err := s.validateSalutation(salutation); err != nil {
		return nil, err
	}

	dedicatedEmail := localPart + EmailDomain

	var result *datamodel.MindAdvisorUser

	// 事务处理
	err := s.userDao.ExecTx(ctx, func(tx *gorm.DB) error {
		txDao := dao.NewMindAdvisorUserDao(tx)

		// 检查用户是否已创建邮箱
		existing, err := txDao.GetByUserID(ctx, userID)
		if err != nil {
			return err
		}

		if existing != nil {
			// 用户已存在邮箱
			if existing.DedicatedEmail != "" {
				// 如果已创建的邮箱与请求的不同，返回冲突错误
				if existing.DedicatedEmail != dedicatedEmail {
					return ErrMailboxConflict
				}
				// 相同则直接返回已有的
				result = existing
				return nil
			}
		}

		// 检查邮箱地址是否被占用
		emailExists, err := txDao.GetByDedicatedEmail(ctx, dedicatedEmail)
		if err != nil {
			return err
		}
		if emailExists != nil {
			return ErrEmailAlreadyExists
		}

		// 创建新记录
		newUser := &datamodel.MindAdvisorUser{
			UserID:         userID,
			DedicatedEmail: dedicatedEmail,
			Config: &datamodel.MindAdvisorUserConfig{
				Salutation: salutation,
			},
			Status: datamodel.MindAdvisorStatusActive,
		}

		if err := txDao.Create(ctx, newUser); err != nil {
			// 检查是否是唯一约束冲突
			if strings.Contains(err.Error(), "Duplicate entry") {
				if strings.Contains(err.Error(), "uk_dedicated_email") {
					return ErrEmailAlreadyExists
				}
				if strings.Contains(err.Error(), "uk_user_id") {
					return ErrUserAlreadyHasMailbox
				}
			}
			return err
		}

		result = newUser
		return nil
	})

	if err != nil {
		logger.ErrorfCtx(ctx, "create mailbox error: %v", err)
		return nil, err
	}

	return result, nil
}

// GetMailbox 获取用户的专属邮箱
func (s *MindAdvisorService) GetMailbox(ctx context.Context, userID string) (*datamodel.MindAdvisorUser, error) {
	user, err := s.userDao.GetByUserID(ctx, userID)
	if err != nil {
		logger.ErrorfCtx(ctx, "get mailbox error: %v", err)
		return nil, err
	}
	return user, nil
}

// validateLocalPart 校验 local_part
func (s *MindAdvisorService) validateLocalPart(localPart string) error {
	// 长度校验
	if len(localPart) < LocalPartMinLen || len(localPart) > LocalPartMaxLen {
		return ErrInvalidLocalPartLength
	}

	// 转小写后校验字符
	lower := strings.ToLower(localPart)
	if !localPartRegex.MatchString(lower) {
		return ErrInvalidLocalPartChars
	}

	// 敏感词过滤
	for _, word := range reservedWords {
		if strings.Contains(lower, word) {
			return ErrReservedWord
		}
	}

	return nil
}

// validateSalutation 校验称呼
func (s *MindAdvisorService) validateSalutation(salutation string) error {
	if salutation != "Mr" && salutation != "Mrs" {
		return ErrInvalidSalutation
	}
	return nil
}

// Init 初始化服务
func (s *MindAdvisorService) Init(ctx context.Context) error {
	if s.IsInited() {
		return nil
	}
	s.SetInited(true)
	return nil
}

// Start 启动服务
func (s *MindAdvisorService) Start(ctx context.Context) error {
	if s.IsStarted() {
		return nil
	}
	logger.Infof("start mind advisor service")
	s.SetStarted(true)
	return nil
}

// Stop 停止服务
func (s *MindAdvisorService) Stop(ctx context.Context) error {
	if s.IsStopped() {
		return nil
	}
	defer s.SetStopped(true)
	logger.Infof("stop mind advisor service")
	return nil
}
