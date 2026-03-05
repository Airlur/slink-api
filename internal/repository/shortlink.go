package repository

import (
	"context"
	"time"

	"slink-api/internal/dto"
	"slink-api/internal/model"
    
	"gorm.io/gorm"
)

type ShortlinkRepository interface {
	// ===== 基础 CRUD 接口方法 =====
	Create(ctx context.Context, shortlink *model.Shortlink) error
	Update(ctx context.Context, id uint, updates map[string]interface{}) error // 使用 map 更新，避免零值覆盖，更安全
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, options *dto.ListMyShortlinksRequest) ([]*model.Shortlink, int64, error)
	// IncrementClickCount 更新点击统计等信息
	IncrementClickCount(ctx context.Context, id uint) error
	// ===== 根据索引查询接口方法 =====
	// GetUniqueShortCode 根据 ShortCode 查询
	GetUniqueShortCode(ctx context.Context, shortCode string, preloadAssociations bool) (*model.Shortlink, error)
	// GetUniqueUserIdAndOriginalUrlMd5 根据 userId 查询 OriginalUrlMd5.
	GetByOriginalURLMd5(ctx context.Context, originalUrlMd5 string) (*model.Shortlink, error)
	// GetUniqueUserIdAndOriginalUrlMd5 根据 userId 查询 
	GetUniqueUserIdAndOriginalUrlMd5(ctx context.Context, userId int64, originalUrlMd5 string) (*model.Shortlink, error)
}

type shortlinkRepository struct {
	db *gorm.DB
}

func NewShortlinkRepository(db *gorm.DB) ShortlinkRepository {
	return &shortlinkRepository{db: db}
}

// ===== 基础 CRUD 接口方法实现 =====
func (r *shortlinkRepository) Create(ctx context.Context, shortlink *model.Shortlink) error {
	return r.db.WithContext(ctx).Create(shortlink).Error
}

func (r *shortlinkRepository) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	// return r.db.WithContext(ctx).Model(&model.Shortlink{}).Where("id = ?", id).Updates(updates).Error
	db := r.db.WithContext(ctx).Model(&model.Shortlink{}).Where("id = ?", id).Updates(updates)
	if db.Error != nil {
		return db.Error
	}
	// 检查是否有行受到影响，防止操作一个不存在的id
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// 软删除，同时更新短链接的状态
func (r *shortlinkRepository) Delete(ctx context.Context, id uint) error {
	// 构造需要更新的字段：status 设为 0（禁用），deleted_at 设为当前时间（软删除标记） 
	// 后期规范下使用常量定义状态
	updateData := map[string]interface{}{
	"status":     0,
	"deleted_at": time.Now(),
	}

 	// 执行 UPDATE 操作：仅更新未被软删除的数据（deleted_at IS NULL）
	db := r.db.WithContext(ctx).Model(&model.Shortlink{}).Where("id = ?", id).Updates(updateData)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *shortlinkRepository) ListByUserID(ctx context.Context, userID uint, options *dto.ListMyShortlinksRequest) ([]*model.Shortlink, int64, error) {
	var links []*model.Shortlink
	var total int64

	// 1. 构建基础查询
	db := r.db.WithContext(ctx).Model(&model.Shortlink{}).Where("shortlinks.user_id = ?", userID)

	// 2. 应用筛选条件
	if options.Tag != "" {
		// 使用JOIN来根据tag筛选
		db = db.Joins("JOIN tags ON tags.short_code = shortlinks.short_code").
			Where("tags.tag_name = ? AND tags.deleted_at IS NULL", options.Tag)
	}

	// 3. 在应用分页和排序前，先执行 COUNT 查询以获取总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 4. 应用排序
	switch options.SortBy {
	case "clicks_desc":
		db = db.Order("click_count DESC")
	case "clicks_asc":
		db = db.Order("click_count ASC")
	case "created_at_desc":
		db = db.Order("created_at DESC")
	case "created_at_asc":
		db = db.Order("created_at ASC")
	default: // 默认按创建时间倒序
		db = db.Order("created_at DESC")
	}

	// 5. 应用分页
	if options.Page > 0 && options.Limit > 0 {
		offset := (options.Page - 1) * options.Limit
		db = db.Offset(offset).Limit(options.Limit)
	}

	// 6. 执行最终查询，并使用 Preload 预加载关联的 Tags 和 Share
	err := db.Preload("Tags").Preload("Share").Find(&links).Error
	if err != nil {
		return nil, 0, err
	}

	return links, total, nil
}

// ===== 新增方法的实现 =====
func (r *shortlinkRepository) IncrementClickCount(ctx context.Context, id uint) error {
	// 使用 gorm.Expr 来执行原子更新操作 (UPDATE ... SET click_count = click_count + 1)
	// 这是并发安全的
	return r.db.WithContext(ctx).Model(&model.Shortlink{}).Where("id = ?", id).UpdateColumn("click_count", gorm.Expr("click_count + ?", 1)).Error
}

// ===== 根据索引查询接口方法实现 =====

func (r *shortlinkRepository) GetUniqueShortCode(ctx context.Context, shortCode string, preloadAssociations bool) (*model.Shortlink, error) {
    var m model.Shortlink
	
    db := r.db.WithContext(ctx)
	
	// 根据参数决定是否预加载
	if preloadAssociations {
		db = db.Preload("Tags").Preload("Share")
	}

	if err := db.Where("short_code = ?", shortCode).First(&m).Error; err != nil {
		return nil, err
	}
	
    return &m, nil
}

// TODO:查询的必须要是状态有效，没有过期的短链接，并且过期后，这个状态得变更下吧
func (r *shortlinkRepository) GetByOriginalURLMd5(ctx context.Context, originalUrlMd5 string) (*model.Shortlink, error) {
    var m model.Shortlink
    if err := r.db.WithContext(ctx).Where("original_url_md5 = ? AND status = 1 AND (expire_at IS NULL OR expire_at > NOW())", originalUrlMd5).First(&m).Error; err != nil {
        return nil, err
    }
    return &m, nil
}

func (r *shortlinkRepository) GetUniqueUserIdAndOriginalUrlMd5(ctx context.Context, userId int64, originalUrlMd5 string) (*model.Shortlink, error) {
    var m model.Shortlink
    if err := r.db.WithContext(ctx).Where("user_id = ? AND original_url_md5 = ? AND status = 1 AND (expire_at IS NULL OR expire_at > NOW())", userId, originalUrlMd5).First(&m).Error; err != nil {
        return nil, err
    }
    return &m, nil
}
