package repository

import (
	"context"

	"slink-api/internal/model"
    
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ShareRepository interface {
	// GetByShortCode retrieves a share record by its short_code.
	GetByShortCode(ctx context.Context, shortCode string) (*model.Share, error)
	// Upsert creates a new share record or updates an existing one.
	Upsert(ctx context.Context, share *model.Share) error
	// ===== 基础 CRUD 接口方法 =====
	Create(ctx context.Context, share *model.Share) error
	Update(ctx context.Context, id uint, updates map[string]interface{}) error // 使用 map 更新，避免零值覆盖，更安全
	Delete(ctx context.Context, id uint) error
	GetByID(ctx context.Context, id uint) (*model.Share, error)
	List(ctx context.Context, offset, limit int) ([]*model.Share, error)
	// ===== 根据索引查询接口方法 =====
	// GetUniqueShortCode finds a record by ShortCode.
	GetUniqueShortCode(ctx context.Context, shortCode string) (*model.Share, error)
}

type shareRepository struct {
	db *gorm.DB
}

func NewShareRepository(db *gorm.DB) ShareRepository {
	return &shareRepository{db: db}
}

func (r *shareRepository) GetByShortCode(ctx context.Context, shortCode string) (*model.Share, error) {
	var m model.Share
	if err := r.db.WithContext(ctx).Where("short_code = ?", shortCode).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *shareRepository) Upsert(ctx context.Context, share *model.Share) error {
	// gorm.io/gorm/clause 提供了 OnConflict 子句，可以优雅地实现 Upsert
	// 当 short_code 冲突时，更新指定的字段
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "short_code"}},
		DoUpdates: clause.AssignmentColumns([]string{"share_title", "share_desc", "share_image"}),
	}).Create(share).Error
}

// ===== 基础 CRUD 接口方法实现 =====
func (r *shareRepository) Create(ctx context.Context, share *model.Share) error {
	return r.db.WithContext(ctx).Create(share).Error
}

func (r *shareRepository) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Share{}).Where("id = ?", id).Updates(updates).Error
}

func (r *shareRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Share{}).Error
}

func (r *shareRepository) GetByID(ctx context.Context, id uint) (*model.Share, error) {
	var m model.Share
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *shareRepository) List(ctx context.Context, offset, limit int) ([]*model.Share, error) {
	var items []*model.Share
	if err := r.db.WithContext(ctx).Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ===== 根据索引查询接口方法实现 =====

func (r *shareRepository) GetUniqueShortCode(ctx context.Context, shortCode string) (*model.Share, error) {
    var m model.Share
    // 使用 snakeCase 函数将 Go 字段名 (PascalCase) 转回数据库列名 (snake_case)
    if err := r.db.WithContext(ctx).Where("short_code = ?", shortCode).First(&m).Error; err != nil {
        return nil, err
    }
    return &m, nil
}
