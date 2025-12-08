package dto

import (
    "time"
)

// GetShareInfoResponse 定义了获取分享信息的响应体
type GetShareInfoResponse struct {
	ShortCode  string `json:"shortCode"`  // 关联短码
	ShareTitle string `json:"shareTitle"` // 分享标题
	ShareDesc  string `json:"shareDesc"`  // 分享描述
	ShareImage string `json:"shareImage"` // 分享封面图URL
}

// UpdateShareInfoRequest 定义了更新分享信息的请求体
// 所有字段都是可选的，并带有校验规则
type UpdateShareInfoRequest struct {
	ShareTitle *string `json:"shareTitle" binding:"omitempty,max=50"` // 分享标题
	ShareDesc  *string `json:"shareDesc" binding:"omitempty,max=100"` // 分享描述
	ShareImage *string `json:"shareImage" binding:"omitempty,url"`    // 分享封面图URL
}

// 模板生成的 DTO 代码
// ShareResponse
// @Description 响应给客户端的 Share 信息，屏蔽了敏感字段
type ShareResponse struct {
    ShortCode   string     `json:"shortCode"`  // 关联短码
    ShareTitle  string     `json:"shareTitle"` // 分享标题
    ShareDesc   string     `json:"shareDesc"`  // 分享描述
    ShareImage  string     `json:"shareImage"` // 分享封面图URL
    ID          uint       `json:"id"`        
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateShareRequest
// @Description ...
type CreateShareRequest struct {
    ShortCode   string     `json:"shortCode"`  // 关联短码
    ShareTitle  string     `json:"shareTitle"` // 分享标题
    ShareDesc   string     `json:"shareDesc"`  // 分享描述
    ShareImage  string     `json:"shareImage"` // 分享封面图URL
}

// UpdateShareRequest
// @Description ...
type UpdateShareRequest struct {
    ShortCode   string     `json:"shortCode"`  // 关联短码
    ShareTitle  string     `json:"shareTitle"` // 分享标题
    ShareDesc   string     `json:"shareDesc"`  // 分享描述
    ShareImage  string     `json:"shareImage"` // 分享封面图URL
}
