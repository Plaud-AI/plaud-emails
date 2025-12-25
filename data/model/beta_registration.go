package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// HelpWanted 多选项（JSON数组）
type HelpWanted []string

// Value 实现 driver.Valuer 接口
func (h HelpWanted) Value() (driver.Value, error) {
	return json.Marshal(h)
}

// Scan 实现 sql.Scanner 接口
func (h *HelpWanted) Scan(value any) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, h)
}

// BetaInviteRegistration 内测邀请登记表
// Table name: mind_advisor_beta_invite_registrations
type BetaInviteRegistration struct {
	ID                          uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID                      string     `gorm:"column:user_id;type:varchar(128);not null;uniqueIndex:uk_user_id" json:"user_id"`
	Email                       string     `gorm:"column:email;type:varchar(255);not null;uniqueIndex:uk_email" json:"email"`
	Role                        string     `gorm:"column:role;type:varchar(32);not null" json:"role"`
	RoleOther                   *string    `gorm:"column:role_other;type:varchar(255)" json:"role_other,omitempty"`
	Industry                    string     `gorm:"column:industry;type:varchar(32);not null" json:"industry"`
	IndustryOther               *string    `gorm:"column:industry_other;type:varchar(255)" json:"industry_other,omitempty"`
	MeetingsPerWeek             string     `gorm:"column:meetings_per_week;type:varchar(16);not null" json:"meetings_per_week"`
	PrimaryWorkingLanguage      string     `gorm:"column:primary_working_language;type:varchar(32);not null" json:"primary_working_language"`
	PrimaryWorkingLanguageOther *string    `gorm:"column:primary_working_language_other;type:varchar(255)" json:"primary_working_language_other,omitempty"`
	HelpWanted                  HelpWanted `gorm:"column:help_wanted;type:json;not null" json:"help_wanted"`
	LinkedinURL                 *string    `gorm:"column:linkedin_url;type:varchar(2048)" json:"linkedin_url,omitempty"`
	Extra                       *string    `gorm:"column:extra;type:json" json:"extra,omitempty"`
	Status                      int16      `gorm:"column:status;not null;default:1;index:idx_status" json:"status"`
	CreatedAt                   time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt                   time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (BetaInviteRegistration) TableName() string {
	return "mind_advisor_beta_invite_registrations"
}

// BetaInviteRegistration status constants
const (
	BetaRegistrationStatusActive      int16 = 1   // 正常
	BetaRegistrationStatusInactive    int16 = 0   // 停用
	BetaRegistrationStatusSoftDeleted int16 = 127 // 软删除
)

// Role 枚举值
const (
	RoleFounderCeoCxo   = "founder_ceo_cxo"
	RoleVpDirector      = "vp_director"
	RoleSalesLeader     = "sales_leader"
	RoleInvestor        = "investor"
	RoleDoctorConsultant = "doctor_consultant"
	RoleOther           = "other"
)

// Industry 枚举值
const (
	IndustryTechnology    = "technology"
	IndustryFinance       = "finance"
	IndustryHealthcare    = "healthcare"
	IndustryManufacturing = "manufacturing"
	IndustryOther         = "other"
)

// MeetingsPerWeek 枚举值
const (
	Meetings1To3   = "1_3"
	Meetings4To6   = "4_6"
	Meetings7To10  = "7_10"
	Meetings10Plus = "10_plus"
)

// PrimaryWorkingLanguage 枚举值
const (
	LanguageEnglish  = "english"
	LanguageJapanese = "japanese"
	LanguageChinese  = "chinese"
	LanguageOther    = "other"
)

// HelpWanted 枚举值
const (
	HelpPrepareMeetings  = "prepare_meetings"
	HelpCaptureDecisions = "capture_decisions"
	HelpSeePatterns      = "see_patterns"
	HelpDecisionStyle    = "decision_style"
)

// ValidRoles 有效的角色值
var ValidRoles = []string{
	RoleFounderCeoCxo, RoleVpDirector, RoleSalesLeader,
	RoleInvestor, RoleDoctorConsultant, RoleOther,
}

// ValidIndustries 有效的行业值
var ValidIndustries = []string{
	IndustryTechnology, IndustryFinance, IndustryHealthcare,
	IndustryManufacturing, IndustryOther,
}

// ValidMeetingsPerWeek 有效的会议频率值
var ValidMeetingsPerWeek = []string{
	Meetings1To3, Meetings4To6, Meetings7To10, Meetings10Plus,
}

// ValidLanguages 有效的语言值
var ValidLanguages = []string{
	LanguageEnglish, LanguageJapanese, LanguageChinese, LanguageOther,
}

// ValidHelpWanted 有效的帮助选项值
var ValidHelpWanted = []string{
	HelpPrepareMeetings, HelpCaptureDecisions, HelpSeePatterns, HelpDecisionStyle,
}
