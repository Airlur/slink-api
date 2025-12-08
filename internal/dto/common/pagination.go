package common

// PaginationRequest 定义了通用的分页请求参数
// 通过嵌入此结构体，任何列表查询DTO都可以快速获得分页能力
type PaginationRequest struct {
	Page   int `form:"page"`       // 页码，从1开始
	Limit  int `form:"limit"`      // 每页数量
	SortBy string `form:"sort_by"` // 排序顺序
}

// PaginationResponse 定义了通用的分页响应信息
type PaginationResponse struct {
	Total int64 `json:"total"`    // 数据总条数
	Page  int   `json:"page"`     // 当前页码
	Limit int   `json:"limit"`    // 每页数量
}

// PaginatedData 定义了通用的分页数据响应结构
// 它使用Go 1.18+的泛型 T 来代表任意类型的数据列表，实现了完全的复用
type PaginatedData[T any] struct {
	Data       []T               `json:"data"`			// 数据列表
	Pagination PaginationResponse `json:"pagination"`	// 分页信息
}