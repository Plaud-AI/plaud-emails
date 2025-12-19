package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// LinkedEmailExtra 绑定邮箱扩展信息
type LinkedEmailExtra struct {
	// 预留字段
}

// Value 实现 driver.Valuer 接口
func (e LinkedEmailExtra) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan 实现 sql.Scanner 接口
func (e *LinkedEmailExtra) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, e)
}

// MindAdvisorLinkedEmail 心智幕僚用户绑定外部邮箱表
// Table name: mind_advisor_linked_emails
type MindAdvisorLinkedEmail struct {
	ID         uint64            `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     string            `gorm:"column:user_id;type:varchar(128);not null;uniqueIndex:uk_user_email,priority:1;index:idx_user_id" json:"user_id"`
	Email      string            `gorm:"column:email;type:varchar(255);not null;uniqueIndex:uk_user_email,priority:2;index:idx_email" json:"email"`
	Source     string            `gorm:"column:source;type:varchar(32);not null" json:"source"`
	Verified   bool              `gorm:"column:verified;not null;default:0" json:"verified"`
	VerifiedAt *time.Time        `gorm:"column:verified_at" json:"verified_at"`
	SyncStatus *string           `gorm:"column:sync_status;type:varchar(32)" json:"sync_status"`
	LastSyncAt *time.Time        `gorm:"column:last_sync_at" json:"last_sync_at"`
	Extra      *LinkedEmailExtra `gorm:"column:extra;type:json" json:"extra"`
	Status     int16             `gorm:"column:status;not null;default:1" json:"status"`
	CreatedAt  time.Time         `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time         `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (MindAdvisorLinkedEmail) TableName() string { return "mind_advisor_linked_emails" }

// LinkedEmail source constants
const (
	LinkedEmailSourceForwarding = "forwarding"
	LinkedEmailSourceGmail      = "gmail"
	LinkedEmailSourceOutlook    = "outlook"
	LinkedEmailSourceIMAP       = "imap"
	LinkedEmailSourceManual     = "manual"
)

// IsActive 是否有效
func (m *MindAdvisorLinkedEmail) IsActive() bool {
	return m.Status == MindAdvisorStatusActive
}
