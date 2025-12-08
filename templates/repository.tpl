package repository

import (
	"context"
	
	"short-link/internal/dto"
	"short-link/internal/model"
	
	"gorm.io/gorm"
)

// {{.StructName}}Repository 定义了数据访问接口
type {{.StructName}}Repository interface {
	// --- 核心 CRUD 方法 ---
	Create(ctx context.Context, model *model.{{.StructName}}) error
	Update(ctx context.Context, id uint, updates map[string]interface{}) error
	Delete(ctx context.Context, id uint) error

	// --- 核心查询方法 ---
	GetByID(ctx context.Context, id uint) (*model.{{.StructName}}, error)
	// GetByKey 是一个通用的、基于业务唯一键的查询方法
	GetByKey(ctx context.Context, key string, preload bool) (*model.{{.StructName}}, error)
	// List 是一个通用的、支持分页/排序/筛选的列表查询方法
	List(ctx context.Context, userID uint, options *dto.List{{.StructName}}Request) ([]*model.{{.StructName}}, int64, error)
	
	// --- 【可选】基于其他索引的查询方法 ---
	{{range $indexName, $columns := .Indexes}}
	GetBy{{$indexName}}(ctx context.Context, {{range $i, $col := $columns}}{{lowerCamel $col}} {{getTypeByColName $col}}{{if not (last $i $columns)}}, {{end}}{{end}}) (*model.{{$.StructName}}, error)
	{{- end}}
}

type {{.StructName | lower}}Repository struct {
	db *gorm.DB
}

func New{{.StructName}}Repository(db *gorm.DB) {{.StructName}}Repository {
	return &{{.StructName | lower}}Repository{db: db}
}


// Create 实现了创建记录的数据库操作
func (r *{{.StructName | lower}}Repository) Create(ctx context.Context, model *model.{{.StructName}}) error {
	return r.db.WithContext(ctx).Create(model).Error
}


// Update 实现了更新记录的数据库操作，并包含行数检查
func (r *{{.StructName | lower}}Repository) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	db := r.db.WithContext(ctx).Model(&model.{{.StructName}}{}).Where("id = ?", id).Updates(updates)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 实现了软删除记录的数据库操作，并包含行数检查
func (r *{{.StructName | lower}}Repository) Delete(ctx context.Context, id uint) error {
	// 模板假设你的模型嵌入了 gorm.Model，GORM会处理软删除
	db := r.db.WithContext(ctx).Delete(&model.{{.StructName}}{}, id)
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil

	// 【注意】如果是有状态或其他字段在软删除时需要一并更新的，把上面的代码删除或注释，打开下面注释的代码。不需要，则删除下方注释代码。
	// updateData := map[string]interface{}{
	// 	"status":     model.{{.StructName}}StatusCancellation, // 状态的常量
	// 	"deleted_at": time.Now(), // 记得导入 "time" 包
	// }

	// db := r.db.WithContext(ctx).Model(&model.{{.StructName}}{}).Where("id = ?", id).Updates(updateData)
	// if db.Error != nil {
	// 	return db.Error
	// }
	// if db.RowsAffected == 0 {
	// 	return gorm.ErrRecordNotFound
	// }
	// return nil
}

// GetByID 通过主键ID获取单条记录
func (r *{{.StructName | lower}}Repository) GetByID(ctx context.Context, id uint) (*model.{{.StructName}}, error) {
	var m model.{{.StructName}}
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}


// GetByKey 通过业务唯一键获取单条记录，支持预加载
func (r *{{.StructName | lower}}Repository) GetByKey(ctx context.Context, key string, preload bool) (*model.{{.StructName}}, error) {
	var m model.{{.StructName}}
	db := r.db.WithContext(ctx)

	// 如果需要，预加载关联数据
	if preload {
		// TODO: 你的生成器需要知道哪些是关联字段，并在这里生成 .Preload("...")
		// 示例如下：
		// db = db.Preload("Tags").Preload("Share")
	}

	// TODO: 你的生成器需要知道哪个字段是业务主键 (key)
	keyColumn := "short_code" // 这是一个示例，需要替换
	if err := db.Where(keyColumn+" = ?", key).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// List 实现了强大的列表查询功能
func (r *{{.StructName | lower}}Repository) List(ctx context.Context, userID uint, options *dto.List{{.StructName}}Request) ([]*model.{{.StructName}}, int64, error) {
	var records []*model.{{.StructName}}
	var total int64

	// 1. 构建基础查询，默认包含用户ID隔离
	db := r.db.WithContext(ctx).Model(&model.{{.StructName}}{}).Where("user_id = ?", userID)

	// 2. 【扩展点】应用筛选条件
	// TODO: 你的生成器可以根据元数据，在这里动态生成 WHERE 或 JOIN 子句
	// 示例 (来自shortlink_repository.go):
	// if options.Tag != "" {
	// 	db = db.Joins("JOIN tags ON ...").Where("tags.tag_name = ?", options.Tag)
	// }

	// 3. 在分页和排序前，执行 COUNT 查询
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 4. 应用排序
	// TODO: 你的生成器可以根据元数据，动态生成排序逻辑
	// 示例 (来自shortlink_repository.go):
	// switch options.SortBy {
	// case "clicks_desc":
	// 	db = db.Order("click_count DESC")
	// default:
	// 	db = db.Order("created_at DESC")
	// }
	db = db.Order("created_at DESC") // 提供一个默认排序

	// 5. 应用分页
	if options.Page > 0 && options.Limit > 0 {
		offset := (options.Page - 1) * options.Limit
		db = db.Offset(offset).Limit(options.Limit)
	}

	// 6. 执行最终查询，并预加载关联数据
	// TODO: 你的生成器需要知道哪些是关联字段
	// db = db.Preload("Tags").Preload("Share")
	if err := db.Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}


// ========== 【可选】为每个索引生成对应的 Repository 方法 ==========
{{range $indexName, $columns := .Indexes}}
func (r *{{$.StructName | lower}}Repository) GetBy{{$indexName}}(ctx context.Context, {{range $i, $col := $columns}}{{$col | lowerCamel}} {{getTypeByColName $col}}{{if not (last $i $columns)}}, {{end}}{{end}}) (*model.{{$.StructName}}, error) {
	var m model.{{$.StructName}}
	if err := r.db.WithContext(ctx).Where("{{range $i, $col := $columns}}{{snake $col}} = ?{{if not (last $i $columns)}} AND {{end}}{{end}}", {{range $i, $col := $columns}}{{lowerCamel $col}}{{if not (last $i $columns)}}, {{end}}{{end}}).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}
{{end}}