package dto

import (
{{if .HasTimeField}}
	"time"
{{end}}
    "short-link/internal/dto/common" 
)

// {{.StructName}}Response
// @Description 响应给客户端的 {{.StructName}} 信息，屏蔽了敏感字段
type {{.StructName}}Response struct {
{{- range .Columns}}
    {{- $fieldLine := printf "%s %s `json:\"%s\"`" (pad .Name $.MaxNameLen) (pad .Type $.MaxTypeLen) .JsonTag}}
    {{pad $fieldLine $.MaxFieldDefLen}} // {{.Comment}}
{{- end}}
    {{- $idLine := printf "%s %s `json:\"id\"`" (pad "ID" $.MaxNameLen) (pad "uint" $.MaxTypeLen) }}
    {{pad $idLine $.MaxFieldDefLen}}
    {{- $createdAtLine := printf "%s %s `json:\"created_at\"`" (pad "CreatedAt" $.MaxNameLen) (pad "time.Time" $.MaxTypeLen) }}
    {{pad $createdAtLine $.MaxFieldDefLen}}
    {{- $updatedAtLine := printf "%s %s `json:\"updated_at\"`" (pad "UpdatedAt" $.MaxNameLen) (pad "time.Time" $.MaxTypeLen) }}
    {{pad $updatedAtLine $.MaxFieldDefLen}}
}

// Create{{.StructName}}Request
// @Description ...
type Create{{.StructName}}Request struct {
{{- range .Columns}}
    {{- $fieldLine := printf "%s %s `json:\"%s\"`" (pad .Name $.MaxNameLen) (pad .Type $.MaxTypeLen) .JsonTag}}
    {{pad $fieldLine $.MaxFieldDefLen}} // {{.Comment}}
{{- end}}
}


// Update{{.StructName}}Request
// @Description ...

type Update{{.StructName}}Request struct {
    // 模板将默认生成指针类型的字段以支持部分更新
    // TODO: 你的生成器需要判断哪些字段是允许更新的，并在这里生成
{{- range .Columns}}
    {{- $fieldLine := printf "%s %s `json:\"%s\"`" (pad .Name $.MaxNameLen) (pad .Type $.MaxTypeLen) .JsonTag}}
    {{pad $fieldLine $.MaxFieldDefLen}} // {{.Comment}}
{{- end}}
}

// List{{.StructName}}Request 定义了列表查询的参数
type List{{.StructName}}Request struct {
	common.PaginationRequest // 嵌入公共分页请求
	// TODO: 你的生成器可以根据元数据，在这里动态生成可供筛选和排序的字段
	// 例如:
	// Tag    string `form:"tag"`
	// SortBy string `form:"sort_by"`
}
