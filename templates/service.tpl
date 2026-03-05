package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"slink-api/internal/dto"
	"slink-api/internal/dto/common" // 引入公共分页DTO
	"slink-api/internal/model"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/constant"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/redis"
	"slink-api/internal/pkg/response"
	"slink-api/internal/repository"
	
	"github.com/go-sql-driver/mysql" // 导入mysql驱动以识别错误
	goRedis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// {{.StructName}}Service 定义了 {{.StructName}} 模块的服务层接口
type {{.StructName}}Service interface {
	// --- 核心 CRUD 方法 ---
	Create(ctx context.Context, user *jwt.UserInfo, req *dto.Create{{.StructName}}Request) (*dto.{{.StructName}}Response, error)
	Update(ctx context.Context, user *jwt.UserInfo, key string, req *dto.Update{{.StructName}}Request) error
	Delete(ctx context.Context, user *jwt.UserInfo, key string) error
	Get(ctx context.Context, user *jwt.UserInfo, key string) (*dto.{{.StructName}}Response, error)
	List(ctx context.Context, user *jwt.UserInfo, req *dto.List{{.StructName}}Request) (*common.PaginatedData[*dto.{{.StructName}}Response], error)

	// --- 【可选】基于索引的查询方法 ---
	{{range $indexName, $columns := .Indexes}}
	GetBy{{$indexName}}(ctx context.Context, user *jwt.UserInfo, {{range $i, $col := $columns}}{{lowerCamel $col}} {{getTypeByColName $col}}{{if not (last $i $columns)}}, {{end}}{{end}}) (*dto.{{$.StructName}}Response, error)
	{{- end}}
}

type {{.StructName | lower}}Service struct {
	db             *gorm.DB
	{{.StructName | lower}}Repo repository.{{.StructName}}Repository
	// TODO: 如果需要与其他模块交互，在这里注入其他的 Repository, 下方  NewXXService 的形参和 return 的参数也需要注入
}

func New{{.StructName}}Service(db *gorm.DB, {{.StructName | lower}}Repo repository.{{.StructName}}Repository) {{.StructName}}Service {
	return &{{.StructName | lower}}Service{
		db:             db,
		{{.StructName | lower}}Repo: {{.StructName | lower}}Repo,
	}
}

// getAndCheckOwnership 是一个内部辅助函数，用于获取资源并校验所有权
// key: 资源的业务标识符 (例如 short_code)
// preload: 是否需要预加载关联数据 (Tags, Share 等)
func (s *{{.StructName | lower}}Service) getAndCheckOwnership(ctx context.Context, user *jwt.UserInfo, key string, preload bool) (*model.{{.StructName}}, error) {
	// 调用 Repository 获取资源
	// 注意：这里的 GetByKey 方法需要在 Repository 中实现，它通常是 GetBy[UniqueKey] 的别名
	m, err := s.{{.StructName | lower}}Repo.GetByKey(ctx, key, preload)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.{{.StructName}}NotFound, "{{.StructName}} 不存在")
		}
		return nil, bizErrors.New(response.InternalError, "数据库查询失败")
	}

	// 权限校验：确保操作者是该资源的所有者
	// TODO: 请确保你的 model.{{.StructName}} 结构体中有一个名为 UserId 的字段
	if m.UserId != int64(user.ID) {
		return nil, bizErrors.New(response.Forbidden, "无权操作此资源")
	}

	return m, nil
}

// Create 创建一个新的 {{.StructName}}
func (s *{{.StructName | lower}}Service) Create(ctx context.Context, user *jwt.UserInfo, req *dto.Create{{.StructName}}Request) (*dto.{{.StructName}}Response, error) {
	// 1. DTO 到 Model 的转换
	newRecord := &model.{{.StructName}}{
	{{- range .Columns}}
		{{.Name}}: req.{{.Name}},
	{{- end}}
		// 示例：填充用户ID
		UserId: int64(user.ID),
	}

	// 2. 直接调用 repo.Create()，并对返回的错误进行精确处理
	if err := s.{{.StructName | lower}}Repo.Create(ctx, newRecord); err != nil {
		// 检查是否是MySQL的唯一键冲突错误 (错误码 1062)
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			// 将底层数据库错误直接向上传递，由Handler层进行具体的业务翻译
			return nil, err 
		}

		// 对于所有其他无法识别的错误，记录日志并返回通用内部错误
		logger.Error("创建{{.StructName}}失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "创建失败")
	}

	// 3. 成功创建，将Model转换为DTO并返回
	return convert{{.StructName}}ToDTO(newRecord), nil
}

// Update 更新一个 {{.StructName}}
func (s *{{.StructName | lower}}Service) Update(ctx context.Context, user *jwt.UserInfo, key string, req *dto.Update{{.StructName}}Request) error {
	// 1. 获取资源并校验所有权
	m, err := s.getAndCheckOwnership(ctx, user, key, false) // 更新操作通常不需要预加载
	if err != nil {
		return err
	}

	// 2. 动态构建需要更新的字段
	updates := make(map[string]interface{})
	// TODO: 根据你的 Update...Request DTO 中的指针字段，在这里添加更新逻辑
	// 示例:
	// if req.FieldName != nil {
	//	 updates["field_name"] = *req.FieldName
	// }

	// 3. 如果没有字段需要更新，直接返回
	if len(updates) == 0 {
		return nil
	}
	
	// 4. 执行数据库更新
	if err := s.{{.StructName | lower}}Repo.Update(ctx, m.ID, updates); err != nil {
		return bizErrors.New(response.InternalError, "更新失败")
	}

	// 5. 【缓存逻辑】更新成功后，必须删除缓存以保证数据一致性
	// 如果你的模块不需要缓存，可以注释或删除以下代码块
	cacheKey := fmt.Sprintf("cache:{{.ModuleName}}:%s", key)
	if err := redis.Del(ctx, cacheKey); err != nil {
		// 缓存删除失败是严重问题，需要记录日志，但不应影响主流程
		logger.Error("删除缓存失败", "error", err, "key", cacheKey)
	}

	return nil
}

// Delete 删除一个 {{.StructName}}
func (s *{{.StructName | lower}}Service) Delete(ctx context.Context, user *jwt.UserInfo, key string) error {
	m, err := s.getAndCheckOwnership(ctx, user, key, false)
	if err != nil {
		return err
	}

	if err := s.{{.StructName | lower}}Repo.Delete(ctx, m.ID); err != nil {
		return bizErrors.New(response.InternalError, "删除失败")
	}

	// 【缓存逻辑】删除成功后，也必须删除缓存
	cacheKey := fmt.Sprintf("cache:{{.ModuleName}}:%s", key)
	if err := redis.Del(ctx, cacheKey); err != nil {
		logger.Error("删除缓存失败", "error", err, "key", cacheKey)
	}

	return nil
}


// Get 获取单个 {{.StructName}} 的详情
func (s *{{.StructName | lower}}Service) Get(ctx context.Context, user *jwt.UserInfo, key string) (*dto.{{.StructName}}Response, error) {
	// --- 默认生成的缓存逻辑 ---
	// 说明：默认启用缓存。如果你的模块不需要缓存，可以安全地删除下面的所有缓存相关代码，
	// 只保留 s.getAndCheckOwnership 调用和 DTO 转换部分。
	cacheKey := fmt.Sprintf("cache:{{.ModuleName}}:%s", key)

	// 1. 查缓存
	cachedData, err := redis.Get(ctx, cacheKey)
	if err != nil && err != goRedis.Nil {
		logger.Error("查询Redis缓存失败", "error", err, "key", cacheKey)
	}
	if err == nil { // 缓存命中
		if cachedData == constant.NullCacheValue { // 命中空值缓存
			return nil, bizErrors.New(response.{{.StructName}}NotFound, "{{.StructName}} 不存在")
		}
		var result dto.{{.StructName}}Response
		// TODO: cachedData 可能是 JSON，需要反序列化
		// json.Unmarshal([]byte(cachedData), &result)
		return &result, nil
	}

	// 2. 缓存未命中，进入防击穿逻辑
	lockKey := fmt.Sprintf("lock:{{.ModuleName}}:%s", key)
	lockTTL := 10 * time.Second // 锁的过期时间，防止死锁

	// 3.1 尝试获取分布式锁
	locked, lockErr := redis.SetNX(ctx, lockKey, "1", lockTTL)
	if lockErr != nil {
		logger.Error("尝试获取分布式锁失败", "error", lockErr, "key", lockKey)
	}

	if locked { // 获取锁成功
		// 3.2 获取锁成功：由我来查询数据库并回写缓存
		defer redis.Del(ctx, lockKey) // 确保执行完毕后释放锁
		
		// 3.2.1 缓存未命中 (Cache Miss)，查询数据库并预加载
		m, dbErr := s.getAndCheckOwnership(ctx, user, key, true)
		if dbErr != nil {
			// 数据库也查不到，是缓存穿透的典型场景
			if dbErr == gorm.ErrRecordNotFound {
				// 防穿透：如果数据库不存在，缓存一个空值，并设置较短的过期时间
				nullCacheTTL := time.Duration(config.GlobalConfig.Cache.NullCacheExpirationSeconds) * time.Second
				redis.Set(ctx, cacheKey, constant.NullCacheValue, nullCacheTTL)
				return nil, dbErr
			}
			// 其他数据库错误
			return nil, bizErrors.New(response.InternalError, "数据库查询失败")
		}

		resultDTO := convert{{.StructName}}ToDTO(m)

		// 回写缓存（增加随机抖动，防止缓存雪崩）
		baseTTL := time.Duration(config.GlobalConfig.Cache.DefaultExpirationMinutes) * time.Minute
		jitter := time.Duration(rand.Intn(config.GlobalConfig.Cache.RandomJitterSeconds)) * time.Second
		// TODO: 这里应该将 resultDTO 序列化为 JSON 字符串再存入
		// jsonData, _ := json.Marshal(resultDTO)
		redis.Set(ctx, cacheKey, "...", baseTTL+jitter)

		return resultDTO, nil
	} else { // 获取锁失败
		time.Sleep(50 * time.Millisecond) // 短暂等待
		return s.Get(ctx, user, key)      // 重试（此时大概率已命中缓存）
	}
}

// List 获取 {{.StructName}} 列表
func (s *{{.StructName | lower}}Service) List(ctx context.Context, user *jwt.UserInfo, req *dto.List{{.StructName}}Request) (*common.PaginatedData[*dto.{{.StructName}}Response], error) {
	// 分页参数的默认值和最大值保护
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 { // 设置一个最大值，防止恶意请求
		req.Limit = 100
	}

	// 调用Repository层获取数据
	records, total, err := s.{{.StructName | lower}}Repo.List(ctx, user.ID, req)
	if err != nil {
		logger.Error("获取{{.StructName}}列表失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "获取{{.StructName}}列表失败")
	}

	// 将 model 列表转换为 DTO 列表
	dtoList := make([]*dto.{{.StructName}}Response, 0, len(records))
	for _, record := range records {
		dtoList = append(dtoList, convert{{.StructName}}ToDTO(record))
	}

	return &common.PaginatedData[*dto.{{.StructName}}Response]{
		Data: dtoList,
		Pagination: common.PaginationResponse{
			Total: total,
			Page:  req.Page,
			Limit: req.Limit,
		},
	}, nil
}

// convert{{.StructName}}ToDTO 将 model 转换为 DTO
func convert{{.StructName}}ToDTO(m *model.{{.StructName}}) *dto.{{.StructName}}Response {
	// 这是一个基础转换器，你可能需要根据业务需求进行修改，保留需要的字段
	// 例如，拼接完整的URL、处理关联数据等
	return &dto.{{.StructName}}Response{
		// ... DTO 字段赋值 ...
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		{{- range .Columns}}
		{{.Name}}: m.{{.Name}},
		{{- end}}
	}
}


// ========== 【可选】为每个索引生成对应的 Service 方法 ==========
{{range $indexName, $columns := .Indexes}}
func (s *{{$.StructName | lower}}Service) GetBy{{$indexName}}(ctx context.Context, user *jwt.UserInfo, {{range $i, $col := $columns}}{{$col | lowerCamel}} {{getTypeByColName $col}}{{if not (last $i $columns)}}, {{end}}{{end}}) (*dto.{{$.StructName}}Response, error) {
 	// TODO: 如果需要，请在此处添加针对 user 的权限校验
 	m, err := s.{{$.StructName | lower}}Repo.GetBy{{$indexName}}(ctx, {{range $i, $col := $columns}}{{lowerCamel $col}}{{if not (last $i $columns)}}, {{end}}{{end}})
 	if err != nil {
 		if err == gorm.ErrRecordNotFound {
 			return nil, bizErrors.New(response.{{$.StructName}}NotFound, "{{$.StructName}} not found")
 		}
 		return nil, bizErrors.New(response.InternalError, "database error")
 	}
 	return convert{{$.StructName}}ToDTO(m), nil
}
{{end}}