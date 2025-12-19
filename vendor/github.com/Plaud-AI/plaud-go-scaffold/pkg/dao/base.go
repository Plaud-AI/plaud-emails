package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/model"

	"gorm.io/gorm"
)

type onAdd interface {
	OnAdd()
}
type onUpdate interface {
	OnUpdate()
}

type hasVersion interface {
	GetVersion() int32
	SetVersion(version int32)
}

type hasID interface {
	GetID() any
}

type hasIntID interface {
	GetIntID() int64
}

type hasStrID interface {
	GetStrID() string
}

var _ onAdd = (*model.BaseModel)(nil)
var _ onUpdate = (*model.BaseModel)(nil)
var _ hasID = (*model.BaseIDModel)(nil)
var _ hasID = (*model.BaseStrIDModel)(nil)
var _ hasIntID = (*model.BaseIDModel)(nil)
var _ hasStrID = (*model.BaseStrIDModel)(nil)
var _ hasVersion = (*model.BaseModel)(nil)

// BaseDao is a generic DAO with common CRUD and query helpers.
// T should be a GORM model struct type.
type BaseDao[T any, IDType IDConstraint] struct {
	db *gorm.DB
}

// GetDB returns the underlying GORM DB instance.
func (p *BaseDao[T, IDType]) GetDB() *gorm.DB {
	return p.db
}

// NewBaseDao constructs a BaseDao with the provided DB.
func NewBaseDao[T any, IDType IDConstraint](db *gorm.DB) BaseDao[T, IDType] {
	return BaseDao[T, IDType]{db: db}
}

// ExecTx runs the given function inside a transaction. It is suitable for composing
// multiple database operations across different DAOs/models atomically.
//
// Usage:
//
//	err := accountDao.ExecTx(ctx, func(tx *gorm.DB) error {
//		accDao := NewBaseDao[model.Account, int64](tx)
//		usrDao := NewBaseDao[model.User, int64](tx)
//		if err := accDao.Add(ctx, &acc); err != nil { return err }
//		if err := usrDao.Add(ctx, &usr); err != nil { return err }
//		return nil
//	})
func (p *BaseDao[T, IDType]) ExecTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// Add inserts the given entity.
// Notes:
// - If T implements onAdd (e.g., embeds model.BaseModel), OnAdd will be invoked to set default fields like ct/status.
// - This method does not set ut; use Update/UpdateColumns for updates.
func (p *BaseDao[T, IDType]) Add(ctx context.Context, entity *T) error {
	if entity, ok := any(entity).(onAdd); ok {
		entity.OnAdd()
	}
	return p.db.WithContext(ctx).Create(entity).Error
}

// Get returns one row by id.
// Behavior:
// - When the row is not found, it returns (nil, nil).
func (p *BaseDao[T, IDType]) Get(ctx context.Context, id IDType) (*T, error) {
	var out T
	if err := p.db.WithContext(ctx).Where("id = ?", id).Take(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// Update updates the given entity by its primary key.
// Behavior and constraints:
//   - If T implements onUpdate, OnUpdate will be invoked (commonly sets ut).
//   - Uses Select("*") to persist zero-values. Ensure the entity is loaded from DB first to avoid unintentionally overwriting fields
//     (e.g., ct/id should not be modified). Prefer UpdateColumns for partial updates.
//   - Returns true if any row was updated; false if no rows matched.
func (p *BaseDao[T, IDType]) Update(ctx context.Context, entity *T) (updated bool, err error) {
	if e, ok := any(entity).(onUpdate); ok {
		e.OnUpdate()
	}
	tx := p.db.WithContext(ctx).Model(entity).Select("*").Updates(entity)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

// UpdateSelect updates only the specified fields on the entity by its primary key.
// Behavior and constraints:
//   - If T implements onUpdate, OnUpdate will be invoked (commonly sets ut). Ensure you include "ut" in fields if you want it persisted.
//   - Uses Select(fields...) to update only the provided columns; zero-values of selected fields are persisted.
//   - If no fields are provided, it falls back to Update (Select("*")).
//   - Returns true if any row was updated; false if no rows matched.
func (p *BaseDao[T, IDType]) UpdateSelect(ctx context.Context, entity *T, fields ...string) (updated bool, err error) {
	if e, ok := any(entity).(onUpdate); ok {
		e.OnUpdate()
	}
	tx := p.db.WithContext(ctx).Model(entity)
	if len(fields) == 0 {
		tx = tx.Select("*")
	} else {
		needUt := true
		for _, f := range fields {
			if f == "ut" {
				needUt = false
				break
			}
		}
		if needUt {
			fields = append(fields, "ut")
		}
		tx = tx.Select(fields)
	}
	tx = tx.Updates(entity)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

// UpdateColumns updates specific columns by id.
// Behavior and constraints:
// - Immutable/sensitive columns are ignored: id, ct, ver.
// - If ut is not provided, it will be auto-set to current time.
// - Returns true if any row was updated; returns (false, nil) for empty updates.
// - Does not invoke model hooks.
func (p *BaseDao[T, IDType]) UpdateColumns(ctx context.Context, id IDType, updates map[string]any) (updated bool, err error) {
	if len(updates) == 0 {
		return false, nil
	}
	// filter out immutable/sensitive columns
	filtered := make(map[string]any, len(updates)+1)
	for k, v := range updates {
		// disallow updating id/ct/ver directly via this API
		if k == "id" || k == "ct" || k == "ver" {
			continue
		}
		filtered[k] = v
	}
	// maintain update time if column not explicitly provided
	if _, ok := filtered["ut"]; !ok {
		filtered["ut"] = time.Now().UnixMilli()
	}
	tx := p.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(filtered)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

// UpdateColumnsByVer updates specific columns by id with optimistic locking.
// Behavior and constraints:
// - Matches by (id, ver==currVersion). On success, ver is incremented atomically in DB (ver = ver + 1).
// - Ignores immutable columns: id, ct, ver (ver is controlled by the method).
// - If ut is not provided, it will be auto-set to current time.
// - Returns true if the update succeeded; returns false if version is stale or row not found.
// - Does not invoke model hooks.
func (p *BaseDao[T, IDType]) UpdateColumnsByVer(ctx context.Context, id IDType, currVersion int32, updates map[string]any) (updated bool, err error) {
	if updates == nil {
		updates = map[string]any{}
	}
	// filter out immutable/sensitive columns except version which is controlled below
	filtered := make(map[string]any, len(updates)+2)
	for k, v := range updates {
		if k == "id" || k == "ct" || k == "ver" {
			continue
		}
		filtered[k] = v
	}
	if _, ok := filtered["ut"]; !ok {
		filtered["ut"] = time.Now().UnixMilli()
	}
	// bump version atomically: ver = ver + 1
	filtered["ver"] = gorm.Expr("ver + ?", 1)

	tx := p.db.WithContext(ctx).Model(new(T)).
		Where("id = ? AND ver = ?", id, currVersion).
		Updates(filtered)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

// UpdateByVer updates the whole entity using optimistic locking based on version.
// Behavior and constraints:
// - T must implement hasID and hasVersion (e.g., embed model.BaseModel).
// - Reads currVersion from entity, sets entity version to currVersion+1 before update.
// - If T implements onUpdate, OnUpdate will be invoked (commonly sets ut).
// - Uses Select("*") so zero-values will be persisted. Ensure entity is loaded from DB to avoid overwriting unintended fields.
// - Returns true if updated; false when version is stale or row not found. On failure, entity version is restored to original.
func (p *BaseDao[T, IDType]) UpdateByVer(ctx context.Context, entity *T) (updated bool, err error) {
	var currVersion int32
	var entityVer hasVersion
	var entityID hasID
	if e, ok := any(entity).(hasVersion); ok {
		currVersion = e.GetVersion()
		e.SetVersion(currVersion + 1)
		entityVer = e
	} else {
		return false, errors.New("entity does not implement hasVersion")
	}
	if idObj, ok := any(entity).(hasID); ok {
		entityID = idObj
	} else {
		return false, errors.New("entity does not implement hasID")
	}

	if e, ok := any(entity).(onUpdate); ok {
		e.OnUpdate()
	}

	// First update entity fields when (id, version) matches; select all fields to include zero-values
	tx := p.db.WithContext(ctx).Model(entity).Select("*").Where("id = ? AND ver = ?", entityID.GetID(), currVersion).Updates(entity)
	if tx.Error != nil {
		entityVer.SetVersion(currVersion)
		return false, tx.Error
	}
	if tx.RowsAffected == 0 {
		entityVer.SetVersion(currVersion)
		return false, nil
	}
	return true, nil
}

// UpdateSelectByVer updates only specified fields using optimistic locking.
// Behavior and constraints:
// - T must implement hasID and hasVersion.
// - Reads currVersion from entity, sets entity version to currVersion+1 before update.
// - If T implements onUpdate, OnUpdate will be invoked. Ensure you include "ut" in fields to persist it.
// - Uses Select(fields...) so only selected fields are updated.
// - Returns true if updated; false when version is stale or row not found. On failure, entity version is restored to original.
func (p *BaseDao[T, IDType]) UpdateSelectByVer(ctx context.Context, entity *T, fields ...string) (updated bool, err error) {
	var currVersion int32
	var entityVer hasVersion
	var entityID hasID
	if e, ok := any(entity).(hasVersion); ok {
		currVersion = e.GetVersion()
		e.SetVersion(currVersion + 1)
		entityVer = e
	} else {
		return false, errors.New("entity does not implement hasVersion")
	}
	if idObj, ok := any(entity).(hasID); ok {
		entityID = idObj
	} else {
		return false, errors.New("entity does not implement hasID")
	}

	if e, ok := any(entity).(onUpdate); ok {
		e.OnUpdate()
	}

	// ensure version column is persisted alongside selected fields
	needVer := true
	for _, f := range fields {
		if f == "ver" {
			needVer = false
			break
		}
	}
	if needVer {
		fields = append(fields, "ver")
	}

	tx := p.db.WithContext(ctx).Model(entity)
	if len(fields) == 0 {
		tx = tx.Select("*")
	} else {
		tx = tx.Select(fields)
	}
	tx = tx.Where("id = ? AND ver = ?", entityID.GetID(), currVersion).Updates(entity)
	if tx.Error != nil {
		entityVer.SetVersion(currVersion)
		return false, tx.Error
	}
	if tx.RowsAffected == 0 {
		entityVer.SetVersion(currVersion)
		return false, nil
	}
	return true, nil
}

// Delete hard deletes by id.
// Behavior:
// - Returns true if any row was deleted; returns false (no error) if the id does not exist.
func (p *BaseDao[T, IDType]) Delete(ctx context.Context, id IDType) (deleted bool, err error) {
	tx := p.db.WithContext(ctx).Where("id = ?", id).Delete(new(T))
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

// SoftDelete marks a row as deleted (del = 1) and updates ut.
// Behavior:
// - Returns true if any row was updated; returns false (no error) if the id does not exist.
// - Idempotent: re-calling on an already soft-deleted row will still succeed with update of ut.
func (p *BaseDao[T, IDType]) SoftDelete(ctx context.Context, id IDType) (updated bool, err error) {
	var params = map[string]any{
		"del": 1,
		"ut":  time.Now().UnixMilli(),
	}
	tx := p.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(params)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

// First returns the first row matching where, first record ordered by primary key
// Behavior:
// - If includeDeleted is false, an additional condition "del = 0" is applied.
// - When no row matches, returns (nil, nil).
func (p *BaseDao[T, IDType]) First(ctx context.Context, includeDeleted bool, where string, args ...any) (*T, error) {
	var out T
	q := p.db.WithContext(ctx)
	if where != "" {
		q = q.Where(where, args...)
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if err := q.First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// Take returns the first row matching where, returned by the database in no specified order
// Behavior:
// - If includeDeleted is false, an additional condition "del = 0" is applied.
// - When no row matches, returns (nil, nil).
func (p *BaseDao[T, IDType]) Take(ctx context.Context, includeDeleted bool, where string, args ...any) (*T, error) {
	var out T
	q := p.db.WithContext(ctx)
	if where != "" {
		q = q.Where(where, args...)
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if err := q.Take(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// First returns the first row matching where, first record ordered by primary key
// Behavior:
// - If includeDeleted is false, an additional condition "del = 0" is applied.
// - When no row matches, returns (nil, nil).
func (p *BaseDao[T, IDType]) FirstFields(ctx context.Context, includeDeleted bool, fields any, where string, args ...any) (*T, error) {
	var out T
	q := p.db.WithContext(ctx)
	if fields != nil {
		q = q.Select(fields)
	}
	if where != "" {
		q = q.Where(where, args...)
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if err := q.First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// First returns the first row matching where, returned by the database in no specified order
// Behavior:
// - If includeDeleted is false, an additional condition "del = 0" is applied.
// - When no row matches, returns (nil, nil).
func (p *BaseDao[T, IDType]) TakeFields(ctx context.Context, includeDeleted bool, fields any, where string, args ...any) (*T, error) {
	var out T
	q := p.db.WithContext(ctx)
	if fields != nil {
		q = q.Select(fields)
	}
	if where != "" {
		q = q.Where(where, args...)
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if err := q.Take(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// Query wraps GORM Query with optional del filtering and where clause.
// Behavior:
// - fields can be string or []string, e.g., "id,name" or []string{"id", "name"}.
// - If includeDeleted is false, an additional condition "del = 0" is applied.
// - Unselected fields in the result will be zero-values.
func (p *BaseDao[T, IDType]) Query(ctx context.Context, includeDeleted bool, fields any, where string, args ...any) ([]*T, error) {
	var list []*T
	q := p.db.WithContext(ctx).Model(new(T))
	if fields != nil {
		q = q.Select(fields)
	}
	if where != "" {
		q = q.Where(where, args...)
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Count returns the count of rows matching where.
// Behavior:
// - If includeDeleted is false, an additional condition "del = 0" is applied.
func (p *BaseDao[T, IDType]) Count(ctx context.Context, includeDeleted bool, where string, args ...any) (int64, error) {
	var cnt int64
	q := p.db.WithContext(ctx).Model(new(T))
	if where != "" {
		q = q.Where(where, args...)
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if err := q.Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

// QueryByCursor performs cursor pagination by id, only works for int64 id.
// Behavior and usage notes:
// - Ascending (desc=false): returns rows with id > lastID ordered by id ASC.
// - Descending (desc=true): returns rows with id < lastID ordered by id DESC.
// - Next cursor is typically the id of the last element in the returned slice.
// - If includeDeleted is false, an additional condition "del = 0" is applied.
// - Ensure the provided where condition preserves monotonicity by id across pages to avoid duplicates/omissions.
func (p *BaseDao[T, IDType]) QueryByCursor(ctx context.Context, includeDeleted bool, lastID int64, limit int, desc bool, where string, args ...any) ([]*T, error) {
	var entity T
	if _, ok := any(&entity).(hasIntID); !ok {
		return nil, fmt.Errorf("entity %T does not implement hasIntID", entity)
	}
	var list []*T
	q := p.db.WithContext(ctx).Model(new(T))
	if where != "" {
		q = q.Where(where, args...)
	}
	if lastID > 0 {
		if desc {
			q = q.Where("id < ?", lastID)
		} else {
			q = q.Where("id > ?", lastID)
		}
	}
	if !includeDeleted {
		q = q.Where("del = 0")
	}
	if desc {
		q = q.Order("id DESC")
	} else {
		q = q.Order("id ASC")
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
