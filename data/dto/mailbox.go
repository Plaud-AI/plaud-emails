package dto

import datamodel "plaud-emails/data/model"

// MailboxConfig 邮箱配置
type MailboxConfig struct {
	Salutation string `json:"salutation"`
}

// Mailbox 邮箱 DTO
type Mailbox struct {
	DedicatedEmail string         `json:"dedicated_email"`
	LocalPart      string         `json:"local_part"`
	Status         string         `json:"status"`
	Config         *MailboxConfig `json:"config"`
}

// MailboxResponse 邮箱响应
type MailboxResponse struct {
	Mailbox *Mailbox `json:"mailbox"`
}

// NewMailboxFromModel 从 Model 转换为 DTO
func NewMailboxFromModel(m *datamodel.MindAdvisorUser) *Mailbox {
	if m == nil {
		return nil
	}

	var config *MailboxConfig
	if m.Config != nil {
		config = &MailboxConfig{
			Salutation: m.Config.Salutation,
		}
	}

	// 从 dedicated_email 中提取 local_part
	localPart := extractLocalPart(m.DedicatedEmail)

	return &Mailbox{
		DedicatedEmail: m.DedicatedEmail,
		LocalPart:      localPart,
		Status:         "EMAIL_CREATED",
		Config:         config,
	}
}

// extractLocalPart 从完整邮箱地址中提取 local_part
func extractLocalPart(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}
