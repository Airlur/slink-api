package repository

import (
	"context"

	"slink-api/internal/model"
    
	"gorm.io/gorm"
)

type TagRepository interface {
	// Create 为短链接添加一个标签
	Create(ctx context.Context, tag *model.Tag) error
	// Delete 软删除一个标签关联
	Delete(ctx context.Context, userID uint, shortCode, tagName string) error
	// ListUniqueTagsByUserID 获取一个用户的所有唯一标签名
	ListUniqueTagsByUserID(ctx context.Context, userID uint) ([]string, error)
	// FindByUK 通过唯一键（用户ID, 短码, 标签名）查找记录，包括已软删除的
	FindByUK(ctx context.Context, userID uint, shortCode, tagName string) (*model.Tag, error)
	// Undelete 恢复一个已软删除的标签（通过更新 deleted_at 为 NULL）
	Undelete(ctx context.Context, id uint) error
}

type tagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) TagRepository {
	return &tagRepository{db: db}
}

func (r *tagRepository) Create(ctx context.Context, tag *model.Tag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}

func (r *tagRepository) Delete(ctx context.Context, userID uint, shortCode, tagName string) error {
	// 使用软删除
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND short_code = ? AND tag_name = ?", userID, shortCode, tagName).
		Delete(&model.Tag{})
		
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // 表示没有找到要删除的记录
	}
	return nil
}

func (r *tagRepository) ListUniqueTagsByUserID(ctx context.Context, userID uint) ([]string, error) {
	var tags []string
	// 使用 Distinct 和 Pluck 来高效地获取唯一的标签名列表
	err := r.db.WithContext(ctx).Model(&model.Tag{}).
		Where("user_id = ?", userID).
		Distinct().
		Pluck("tag_name", &tags).Error
	return tags, err
}


// FindByUK 通过唯一键（用户ID, 短码, 标签名）查找记录，包括已软删除的
func (r *tagRepository) FindByUK(ctx context.Context, userID uint, shortCode, tagName string) (*model.Tag, error) {
	var m model.Tag
	// GORM 的 Unscoped() 方法可以查询到被软删除的记录
	err := r.db.WithContext(ctx).Unscoped().
		Where("user_id = ? AND short_code = ? AND tag_name = ?", userID, shortCode, tagName).
		First(&m).Error
	return &m, err
}

// Undelete 恢复一个已软删除的标签（通过更新 deleted_at 为 NULL）
func (r *tagRepository) Undelete(ctx context.Context, id uint) error {
	// 使用 Unscoped() 确保能操作到已软删除的记录
	return r.db.WithContext(ctx).Model(&model.Tag{}).Unscoped().
		Where("id = ?", id).
		Update("deleted_at", nil).Error
}