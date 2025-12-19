package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// MindAdvisorUserConfig 心智幕僚用户配置
type MindAdvisorUserConfig struct {
	Salutation string `json:"salutation"` // Mr 或 Mrs
}

// Value 实现 driver.Valuer 接口
func (c MindAdvisorUserConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan 实现 sql.Scanner 接口
func (c *MindAdvisorUserConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, c)
}

// MindAdvisorUser 心智幕僚用户表
// Table name: users_mind_advisor
type MindAdvisorUser struct {
	ID             uint64                 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID         string                 `gorm:"column:user_id;type:varchar(128);not null;uniqueIndex:uk_user_id" json:"user_id"`
	DedicatedEmail string                 `gorm:"column:dedicated_email;type:varchar(255);not null;uniqueIndex:uk_dedicated_email" json:"dedicated_email"`
	Config         *MindAdvisorUserConfig `gorm:"column:config;type:json" json:"config"`
	Status         int16                  `gorm:"column:status;not null;default:1;index:idx_status" json:"status"`
	CreatedAt      time.Time              `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time              `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (MindAdvisorUser) TableName() string { return "users_mind_advisor" }

// MindAdvisorUser status constants
const (
	MindAdvisorStatusActive      int16 = 1   // 正常
	MindAdvisorStatusInactive    int16 = 0   // 停用
	MindAdvisorStatusSoftDeleted int16 = 127 // 软删除
)

// IsActive 是否有效
func (m *MindAdvisorUser) IsActive() bool {
	return m.Status == MindAdvisorStatusActive
}
