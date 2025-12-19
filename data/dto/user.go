package dto

import datamodel "plaud-emails/data/model"

// User DTO for external API layers
type User struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// NewUserFromModel converts model.User to dto.User
func NewUserFromModel(m *datamodel.User) *User {
	if m == nil {
		return nil
	}
	return &User{
		ID:      m.ID,
		Name:    m.Name,
		Address: m.Address,
	}
}
