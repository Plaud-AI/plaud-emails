package dao

import (
	datamodel "plaud-emails/data/model"

	basedao "github.com/Plaud-AI/plaud-go-scaffold/pkg/dao"

	"gorm.io/gorm"
)

type UserDao struct {
	basedao.BaseDao[datamodel.User, int64]
}

func NewUserDao(db *gorm.DB) *UserDao {
	return &UserDao{basedao.NewBaseDao[datamodel.User, int64](db)}
}
