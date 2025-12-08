package dto

import (
    "time"
)


// AddTagRequest 定义了添加标签的请求体
type AddTagRequest struct {
	// 标签名，必填，长度限制1-30
	TagName string `json:"tagName" binding:"required,min=1,max=30"`
}

// RemoveTagRequest 定义了移除标签的请求体
type RemoveTagRequest struct {
	TagName string `json:"tagName" binding:"required"`
}

// TagListResponse 定义了用户标签列表的响应体
type TagListResponse struct {
	Tags []string `json:"tags"`
}

// TagResponse
// @Description 响应给客户端的 Tag 信息，屏蔽了敏感字段
type TagResponse struct {
    ShortCode  string     `json:"shortCode"` // 关联短码
    UserId     int64      `json:"userId"`    // 关联用户ID
    TagName    string     `json:"tagName"`   // 标签名称（如“活动推广”）
    ID         uint       `json:"id"`       
    CreatedAt  time.Time  `json:"created_at"`
    UpdatedAt  time.Time  `json:"updated_at"`
}

// CreateTagRequest
// @Description ...
type CreateTagRequest struct {
    ShortCode  string     `json:"shortCode"` // 关联短码
    UserId     int64      `json:"userId"`    // 关联用户ID
    TagName    string     `json:"tagName"`   // 标签名称（如“活动推广”）
}

// UpdateTagRequest
// @Description ...
type UpdateTagRequest struct {
    ShortCode  string     `json:"shortCode"` // 关联短码
    UserId     int64      `json:"userId"`    // 关联用户ID
    TagName    string     `json:"tagName"`   // 标签名称（如“活动推广”）
}
