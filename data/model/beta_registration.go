package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// QuestionnaireItem 问卷单项
type QuestionnaireItem struct {
	Key   string   `json:"key"`
	Value []string `json:"value"`
}

// Questionnaire 问卷信息 JSON 类型（数组格式）
type Questionnaire []QuestionnaireItem

// Value 实现 driver.Valuer 接口
func (q Questionnaire) Value() (driver.Value, error) {
	if q == nil {
		return nil, nil
	}
	return json.Marshal(q)
}

// Scan 实现 sql.Scanner 接口
func (q *Questionnaire) Scan(value any) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, q)
}

// BetaRegistrationExtra 扩展信息 JSON 类型
type BetaRegistrationExtra map[string]any

// Value 实现 driver.Valuer 接口
func (e BetaRegistrationExtra) Value() (driver.Value, error) {
	if e == nil {
		return nil, nil
	}
	return json.Marshal(e)
}

// Scan 实现 sql.Scanner 接口
func (e *BetaRegistrationExtra) Scan(value any) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, e)
}

// BetaInviteRegistration 内测邀请登记表
// Table name: mind_advisor_beta_invite_registrations
type BetaInviteRegistration struct {
	ID            uint64                  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID        string                  `gorm:"column:user_id;type:varchar(128);not null;uniqueIndex:uk_user_id" json:"user_id"`
	Email         string                  `gorm:"column:email;type:varchar(255);not null;uniqueIndex:uk_email" json:"email"`
	Questionnaire Questionnaire           `gorm:"column:questionnaire;type:json;not null" json:"questionnaire"`
	Extra         *BetaRegistrationExtra  `gorm:"column:extra;type:json" json:"extra,omitempty"`
	Status        int16                   `gorm:"column:status;not null;default:1;index:idx_status" json:"status"`
	CreatedAt     time.Time               `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time               `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
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
