package user

import (
	"context"
	"errors"

	"plaud-emails/dao"

	datamodel "plaud-emails/data/model"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/snowflake"
	"github.com/Plaud-AI/plaud-go-scaffold/pkg/svc"

	"gorm.io/gorm"
)

// UserService 用户服务
type UserService struct {
	svc.BaseService
	userDao   *dao.UserDao
	generator *snowflake.Generator
}

func New(db *gorm.DB, idCreater *snowflake.GeneratorCreater) (*UserService, error) {
	generator, err := idCreater.Create()
	if err != nil {
		return nil, err
	}
	return &UserService{generator: generator, userDao: dao.NewUserDao(db)}, nil
}

func (p *UserService) AddUser(ctx context.Context, name string, address string) (*datamodel.User, error) {
	if p.generator == nil {
		return nil, errors.New("snowflake generator is nil")
	}
	id := p.generator.NextID()
	u := &datamodel.User{
		Name:    name,
		Address: address,
	}

	u.ID = id
	u.OnAdd()

	if err := p.userDao.Add(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (p *UserService) GetUser(ctx context.Context, id int64) (*datamodel.User, error) {
	return p.userDao.Get(ctx, id)
}

func (p *UserService) SoftDeleteUser(ctx context.Context, id int64) (ok bool, err error) {
	return p.userDao.SoftDelete(ctx, id)
}

func (p *UserService) DeleteUser(ctx context.Context, id int64) (ok bool, err error) {
	return p.userDao.Delete(ctx, id)
}

func (p *UserService) UpdateUserColumns(ctx context.Context, id int64, name string, address string) (bool, error) {
	updates := map[string]any{}
	updates["name"] = name
	updates["address"] = address
	if len(updates) == 0 {
		return false, nil
	}
	return p.userDao.UpdateColumns(ctx, id, updates)
}

func (p *UserService) UpdateUser(ctx context.Context, user *datamodel.User) (bool, error) {
	return p.userDao.Update(ctx, user)
}

// Init 初始化服务
func (p *UserService) Init(ctx context.Context) error {
	if p.IsInited() {
		return nil
	}

	if p.userDao == nil {
		return errors.New("user dao is nil")
	}
	if p.generator == nil {
		return errors.New("snowflake generator is nil")
	}
	p.SetInited(true)
	return nil
}

func (p *UserService) Start(ctx context.Context) error {
	if p.IsStarted() {
		return nil
	}

	logger.Infof("start user service")
	p.SetStarted(true)
	return nil
}

func (p *UserService) Stop(ctx context.Context) error {
	if p.IsStopped() {
		return nil
	}
	defer p.SetStopped(true)

	logger.Infof("stop user service")
	return nil
}
