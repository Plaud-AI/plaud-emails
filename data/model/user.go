package model

import scaffoldmodel "github.com/Plaud-AI/plaud-go-scaffold/pkg/model"

// User GORM model
// Table name: users
// Fields: id,name,address,ct,ut,ver,status,del
// Primary key: id (int64)
type User struct {
	scaffoldmodel.BaseIDModel
	Name    string `gorm:"column:name;type:varchar(255)" json:"name"`
	Address string `gorm:"column:address;type:varchar(255)" json:"address"`
}

func (User) TableName() string { return "users" }
