package repository

import (
	"context"
	"time"

	"short-link/internal/dto"
	"short-link/internal/model"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, id uint, updates map[string]interface{}) error // 使用 map 更新，避免零值覆盖
	Delete(ctx context.Context, id uint) error
	// unscoped = false 查询的是活跃用户；unscoped = true 查询的是状态为注销，有删除时间的用户
	FindOne(ctx context.Context, conditions *model.User, unscoped bool) (*model.User, error)
	List(ctx context.Context, options *dto.ListUsersRequest) ([]*model.User, int64, error) 
	UpdateUnscoped(ctx context.Context, id uint, updates map[string]interface{}) error
}



type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create 创建用户
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// Update 更新用户信息
func (r *userRepository) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	db := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 用户注销账号，软删除并更新状态
func (r *userRepository) Delete(ctx context.Context, id uint) error {
	// 构造需要更新的字段
	updateData := map[string]interface{}{
		"status":     model.UserStatusCancellation, // status为“已注销”
		"deleted_at": time.Now(), // eleted_at 设为当前时间以触发软删除
	}

	db := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updateData)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FindOne 是一个通用的、基于条件查询单条记录的方法
func (r *userRepository) FindOne(ctx context.Context, conditions *model.User, unscoped bool) (*model.User, error) {
	var user model.User
	db := r.db.WithContext(ctx)

	// 根据参数决定是否查询软删除的记录
	if unscoped {
		db = db.Unscoped()
	}

	// GORM会根据 conditions 中非零值的字段自动构建 WHERE 查询
	if err := db.Where(conditions).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// List 返回 User 列表
func (r *userRepository) List(ctx context.Context, options *dto.ListUsersRequest) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	// 1. 构建基础查询
	db := r.db.WithContext(ctx).Model(&model.User{})

	// 对于管理员，需要查询包含已删除的记录
	db = db.Unscoped()

	// 2. 【核心】应用动态筛选条件
	if options.Username != nil && *options.Username != "" {
		db = db.Where("username LIKE ?", "%"+*options.Username+"%")
	}
	if options.Status != nil {
		db = db.Where("status = ?", *options.Status)
	}

	// 3. 在分页和排序前，先执行 COUNT 查询
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 4. 【核心】应用动态排序
	if options.SortBy != "" {
		switch options.SortBy {
		case "created_at_desc":
			db = db.Order("created_at DESC")
		case "created_at_asc":
			db = db.Order("created_at ASC")
		case "last_login_at_desc":
			db = db.Order("last_login_at DESC")
		case "last_login_at_asc":
			db = db.Order("last_login_at ASC")
		default:
			// 对于未知的排序参数，可以忽略或使用默认排序
			db = db.Order("created_at DESC")
		}
	} else {
		// 默认排序
		db = db.Order("created_at DESC")
	}

	// 5. 应用分页
	// 注意：options里的Page和Limit已经在Service层处理过默认值和最大值
	offset := (options.Page - 1) * options.Limit
	db = db.Offset(offset).Limit(options.Limit)

	// 6. 执行最终查询
	if err := db.Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// UpdateUnscoped 更新用户（包括已软删除的）
func (r *userRepository) UpdateUnscoped(ctx context.Context, id uint, updates map[string]interface{}) error {
	db := r.db.WithContext(ctx).Model(&model.User{}).Unscoped().Where("id = ?", id).Updates(updates)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}