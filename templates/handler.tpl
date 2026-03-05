package v1

import (
	"errors"
	
	"slink-api/internal/dto"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/response"
	"slink-api/internal/pkg/validator"
	"slink-api/internal/service"

	"github.com/gin-gonic/gin"
)

type {{.StructName}}Handler struct {
	svc service.{{.StructName}}Service
}

func New{{.StructName}}Handler(svc service.{{.StructName}}Service) *{{.StructName}}Handler {
	return &{{.StructName}}Handler{svc: svc}
}

// handle{{.StructName}}ServiceError 是本模块专属的通用错误处理器。
// 你可以根据每个模块的特定错误（如MySQL唯一键冲突）进行定制化修改。
func handle{{.StructName}}ServiceError(c *gin.Context, err error) {
	// 尝试从错误链中解析出我们自定义的业务错误
	var bizErr *bizErrors.Error
	if errors.As(err, &bizErr) {
		// 如果Service层返回的错误中包含了具体的消息，我们优先使用它
		// 否则，Fail函数会从errorMap中查找默认消息
		response.Fail(c, bizErr.Code, bizErr.Message)
		return
	}

	// 对于所有其他未知错误，记录日志并返回通用的内部错误
	logger.Error("未处理的服务层错误", "error", err)
	response.Fail(c, response.InternalError, "") // message留空，Fail函数会自动填充
}

// === 接口实现 ===

// Create 创建一个新的 {{.StructName}} 资源
func (h *{{.StructName}}Handler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	// 默认获取用户信息，用于Service层进行业务处理和权限判断
	userInfo := jwt.GetUserInfo(c)

	var req dto.Create{{.StructName}}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	result, err := h.svc.Create(ctx, userInfo, &req)
	if err != nil {
		handleShortlinksServiceError(c, err)
		return
	}
	response.Ok(c, result, "创建成功")
}

// Update 更新一个已存在的 {{.StructName}} 资源
func (h *{{.StructName}}Handler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	// 统一使用字符串类型的 key 作为资源标识符
	key := c.Param("key")
	userInfo := jwt.GetUserInfo(c)

	var req dto.Update{{.StructName}}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Update(ctx, userInfo, key, &req); err != nil {
		handle{{.StructName}}ServiceError(c, err)
		return
	}

	response.Ok(c, nil, "更新成功")
}

// Delete 删除一个 {{.StructName}} 资源
func (h *{{.StructName}}Handler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	key := c.Param("key")
	userInfo := jwt.GetUserInfo(c)

	if err := h.svc.Delete(ctx, userInfo, key); err != nil {
		handle{{.StructName}}ServiceError(c, err)
		return
	}
	response.Ok(c, nil, "删除成功")
}

// Get 获取单个 {{.StructName}} 资源的详细信息
func (h *{{.StructName}}Handler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	key := c.Param("key")
	userInfo := jwt.GetUserInfo(c)

	result, err := h.svc.Get(ctx, userInfo, key)
	if err != nil {
		handle{{.StructName}}ServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取成功")
}

// List 获取 {{.StructName}} 资源列表，支持分页、筛选和排序
func (h *{{.StructName}}Handler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)

	var req dto.List{{.StructName}}Request
	// 使用 ShouldBindQuery 来绑定 URL 查询参数 (e.g., ?page=1&limit=10)
	if err := c.ShouldBindQuery(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}
	
	result, err := h.svc.List(ctx, userInfo, &req)
	if err != nil {
		handle{{.StructName}}ServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取成功")
}

// ========== 【可选】为每个索引生成对应的 Handler 方法 ==========
// 这些方法提供了基于特定索引的快速查询能力。
// 如果某个模块不需要这些接口，可以直接在生成后的文件中删除。
{{range $indexName, $columns := .Indexes}}
func (h *{{$.StructName}}Handler) GetBy{{$indexName}}(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c) // 同样默认获取用户信息

 	// 1. 从 URL 路径中解析参数
 	{{- range $i, $col := $columns}}
 	// ... (此处的参数解析逻辑与旧模板保持一致，但可以根据需要进一步强化)
	{{- end}}

 	// 2. 调用 Service
 	result, err := h.svc.GetBy{{$indexName}}(ctx, userInfo, {{range $i, $col := $columns}}{{lowerCamel $col}}{{if not (last $i $columns)}}, {{end}}{{end}})
	if err != nil {
		handle{{$.StructName}}ServiceError(c, err)
		return
	}
 	// 3. 成功响应
	response.Ok(c, result, "success")
}
{{end}}