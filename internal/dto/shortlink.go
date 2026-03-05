package dto

import (
	"time"

	"slink-api/internal/dto/common"
)

// GuestCreateShortlinkRequest 游客创建短链接的请求体
type GuestCreateShortlinkRequest struct {
	OriginalUrl string `json:"originalUrl" binding:"required"` // 只允许提供原始URL，且必须是合法的URL
}

// UserCreateShortlinkRequest 用户创建短链接的请求体
type UserCreateShortlinkRequest struct {
	OriginalUrl string `json:"originalUrl" binding:"required"`
	ShortCode *string `json:"shortCode,omitempty" binding:"omitempty,alphanum,min=6,max=8"` // omitempty: 可选; alphanum: 只能是字母和数字; min/max: 长度限制
    ExpiresIn *string `json:"expiresIn,omitempty" binding:"omitempty,oneof=1h 24h 7d 30d 90d 1y never"`
}

// ShortlinkResponse
// @Description 响应给客户端的 Shortlink 信息，屏蔽了敏感字段
type ShortlinkResponse struct {
    ShortUrl        string      `json:"shortUrl"`                 // 完整的短链接URL
    ShortCode       string      `json:"shortCode"`                // 短码（Base62，区分大小写）
    OriginalUrl     string      `json:"originalUrl"`              // 原始长链接（URLEncode编码）
    OriginalUrlMd5  string      `json:"originalUrlMd5,omitempty"` // 原始长链接的MD5摘要
    UserId          int64       `json:"userId,omitempty"`         // 关联用户ID（管理员为0）
    ExpireAt        *time.Time  `json:"expireAt"`                 // 过期时间（NULL=永久）
    LastWarnAt      *time.Time  `json:"lastWarnAt,omitempty"`     // 最近一次失效预警发送时间
    Status          int         `json:"status,omitempty"`         // 状态（1=有效，0=失效）
    ClickCount      int64       `json:"clickCount,omitempty"`     // 点击量统计
    IsHot           int         `json:"isHot,omitempty"`          // 是否为热点短码（1=是，日访问≥1000）
    IsCustom        int         `json:"isCustom,omitempty"`       // 是否为自定义短码（1=是）
    ID              uint        `json:"id,omitempty"`            
    CreatedAt       *time.Time  `json:"created_at,omitempty"`    
    UpdatedAt       *time.Time  `json:"updated_at,omitempty"`    
}

// UpdateShortlinkRequest
// @Description ...
type UpdateShortlinkRequest struct {
    OriginalUrl     *string     `json:"originalUrl" binding:"omitempty,url"`          // 原始长链接（URLEncode编码）
    Status          *int        `json:"status" binding:"omitempty,oneof=0 1"`         // 状态（1=有效，0=失效）
	ExpiresIn       *string     `json:"expiresIn,omitempty" binding:"omitempty,oneof=7d 30d 90d 1y never"`
}

// UpdateShortlinkStatusRequest 定义了更新短链接状态的请求体
type UpdateShortlinkStatusRequest struct {
	// 使用指针和 binding:"required" 确保该字段必须被传入
	// oneof=0 1 限制了状态值只能是 0 (失效) 或 1 (有效)
	Status *int `json:"status" binding:"required,oneof=0 1"`
}

// ExtendShortlinkExpirationRequest 定义了延长短链接有效期的请求体
type ExtendShortlinkExpirationRequest struct {
	// 限制了只能传入指定的有效期字符串
	ExpiresIn string `json:"expiresIn" binding:"required,oneof=7d 30d 90d 1y never"`
}

// --- 用于查询列表的请求DTO ---
type ListMyShortlinksRequest struct {
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
	Tag    string `form:"tag"`
	SortBy string `form:"sort_by"`
}

// --- 用于响应的DTO ---

// ShortlinkDetailResponse 用于详情和列表项的响应
type ShortlinkDetailResponse struct {
	ShortUrl    string     `json:"shortUrl"`
	ShortCode   string     `json:"shortCode"`
	OriginalUrl string     `json:"originalUrl"`
	ExpireAt    *time.Time `json:"expireAt"`
	ClickCount  int64      `json:"clickCount"`
	Status      int        `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	Tags        []string   `json:"tags"`      // 关联的标签名列表
	Share       *ShareInfo `json:"share"`     // 关联的分享信息
}

// ShareInfo DTO
type ShareInfo struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Image string `json:"image"`
}

// ShortlinkListResponse 用于 Swagger 文档生成分页响应
type ShortlinkListResponse struct {
	Data       []ShortlinkDetailResponse `json:"data"`
	Pagination common.PaginationResponse `json:"pagination"`
}
