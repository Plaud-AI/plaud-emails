package model

import "time"

const (
	StatusValid   = 1
	StatusInvalid = 0
	DelDeleted    = 1
	DelNormal     = 0
)

type BaseModel struct {
	// 创建时间
	Ct int64 `gorm:"column:ct" json:"ct"`
	// 更新时间
	Ut int64 `gorm:"column:ut" json:"ut"`
	// 版本号
	Ver int32 `gorm:"column:ver" json:"ver"`
	// 状态
	Status int8 `gorm:"column:status" json:"status"`
	// 删除状态
	Del int8 `gorm:"column:del" json:"del"`
}

// OnAdd 添加时设置创建时间, 设置状态为1
func (p *BaseModel) OnAdd() {
	p.Ct = time.Now().UnixMilli()
	p.Status = StatusValid
}

// OnUpdate 更新时设置更新时间
func (p *BaseModel) OnUpdate() {
	p.Ut = time.Now().UnixMilli()
}

// IsDeleted 是否已删除
func (p *BaseModel) IsDeleted() bool {
	return p.Del == DelDeleted
}

// IsValid 是否有效
func (p *BaseModel) IsValid() bool {
	return p.Status == StatusValid && p.Del == DelNormal
}

func (p *BaseModel) GetVersion() int32 {
	return p.Ver
}

func (p *BaseModel) SetVersion(v int32) {
	p.Ver = v
}

// BaseIDModel 整型ID的模型
type BaseIDModel struct {
	// 主键ID
	ID int64 `gorm:"column:id;primaryKey" json:"id"`
	BaseModel
}

func (p *BaseIDModel) GetID() any {
	return p.ID
}

func (p *BaseIDModel) GetIntID() int64 {
	return p.ID
}

// BaseStrIDModel 字符串ID的模型
type BaseStrIDModel struct {
	ID string `gorm:"column:id;primaryKey;type:varchar(64);not null" json:"id"`
	BaseModel
}

func (p *BaseStrIDModel) GetID() any {
	return p.ID
}

func (p *BaseStrIDModel) GetStrID() string {
	return p.ID
}
