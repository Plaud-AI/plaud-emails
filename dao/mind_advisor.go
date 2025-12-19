package dao

import (
	"context"
	"errors"

	datamodel "plaud-emails/data/model"

	"gorm.io/gorm"
)

// MindAdvisorUserDao 心智幕僚用户 DAO
type MindAdvisorUserDao struct {
	db *gorm.DB
}

// NewMindAdvisorUserDao 创建 MindAdvisorUserDao
func NewMindAdvisorUserDao(db *gorm.DB) *MindAdvisorUserDao {
	return &MindAdvisorUserDao{db: db}
}

// GetDB 获取数据库连接
func (d *MindAdvisorUserDao) GetDB() *gorm.DB {
	return d.db
}

// Create 创建心智幕僚用户
func (d *MindAdvisorUserDao) Create(ctx context.Context, user *datamodel.MindAdvisorUser) error {
	return d.db.WithContext(ctx).Create(user).Error
}

// GetByUserID 根据 user_id 查询
func (d *MindAdvisorUserDao) GetByUserID(ctx context.Context, userID string) (*datamodel.MindAdvisorUser, error) {
	var user datamodel.MindAdvisorUser
	err := d.db.WithContext(ctx).Where("user_id = ?", userID).Take(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByDedicatedEmail 根据 dedicated_email 查询
func (d *MindAdvisorUserDao) GetByDedicatedEmail(ctx context.Context, email string) (*datamodel.MindAdvisorUser, error) {
	var user datamodel.MindAdvisorUser
	err := d.db.WithContext(ctx).Where("dedicated_email = ?", email).Take(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Update 更新心智幕僚用户
func (d *MindAdvisorUserDao) Update(ctx context.Context, user *datamodel.MindAdvisorUser) error {
	return d.db.WithContext(ctx).Save(user).Error
}

// ExecTx 执行事务
func (d *MindAdvisorUserDao) ExecTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return d.db.WithContext(ctx).Transaction(fn)
}

// MindAdvisorLinkedEmailDao 心智幕僚绑定邮箱 DAO
type MindAdvisorLinkedEmailDao struct {
	db *gorm.DB
}

// NewMindAdvisorLinkedEmailDao 创建 MindAdvisorLinkedEmailDao
func NewMindAdvisorLinkedEmailDao(db *gorm.DB) *MindAdvisorLinkedEmailDao {
	return &MindAdvisorLinkedEmailDao{db: db}
}

// Create 创建绑定邮箱
func (d *MindAdvisorLinkedEmailDao) Create(ctx context.Context, email *datamodel.MindAdvisorLinkedEmail) error {
	return d.db.WithContext(ctx).Create(email).Error
}

// GetByUserIDAndEmail 根据 user_id 和 email 查询
func (d *MindAdvisorLinkedEmailDao) GetByUserIDAndEmail(ctx context.Context, userID, email string) (*datamodel.MindAdvisorLinkedEmail, error) {
	var linkedEmail datamodel.MindAdvisorLinkedEmail
	err := d.db.WithContext(ctx).Where("user_id = ? AND email = ?", userID, email).Take(&linkedEmail).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &linkedEmail, nil
}

// ListByUserID 根据 user_id 查询所有绑定邮箱
func (d *MindAdvisorLinkedEmailDao) ListByUserID(ctx context.Context, userID string) ([]*datamodel.MindAdvisorLinkedEmail, error) {
	var emails []*datamodel.MindAdvisorLinkedEmail
	err := d.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, datamodel.MindAdvisorStatusActive).Find(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}
