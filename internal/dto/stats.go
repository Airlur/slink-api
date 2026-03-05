package dto

import (
	"slink-api/internal/dto/common"
	"slink-api/internal/model"
)

// AccessLogListResponse 用于 Swagger 文档生成分页响应
type AccessLogListResponse struct {
	Data       []*model.AccessLog        `json:"data"`
	Pagination common.PaginationResponse `json:"pagination"`
}

// OverviewStatsResponse 概览统计响应体
type OverviewStatsResponse struct {
	TotalClicks int64  `json:"totalClicks"` // 短链接历史总点击量
	ClicksToday int64  `json:"clicksToday"` // 今日点击量
	TopRegion   string `json:"topRegion"`   // 点击量最高的地区
	TopSource   string `json:"topSource"`   // 点击量最高的来源
}

// GetTrendRequest 定义了获取点击趋势的查询参数
type GetTrendRequest struct {
	Period string `form:"period"` // 时间周期，如 7d、30d，留空则使用 start/end
	Start  string `form:"start"`  // 自定义开始日期 (格式: YYYY-MM-DD)
	End    string `form:"end"`    // 自定义结束日期 (格式: YYYY-MM-DD)
}

// GetCitiesStatsRequest 获取城市统计的查询参数
type GetCitiesStatsRequest struct {
	Province string `form:"province"`  // 按省份筛选
}

// TrendStatsResponse 单个短链的趋势响应
type TrendStatsResponse struct {
	Date   string `json:"date"`   // 日期 （YYYY-MM-DD）
	Clicks int64  `json:"clicks"` // 当日点击量
}

// UserOverviewStatsResponse 用户级聚合统计
type UserOverviewStatsResponse struct {
	TotalLinks  int64   `json:"totalLinks"`
	TotalClicks int64   `json:"totalClicks"`
	ClicksToday int64   `json:"clicksToday"`
	GrowthRate  float64 `json:"growthRate"`
}

// UserTrendRequest 用户聚合趋势查询参数
type UserTrendRequest struct {
	Granularity string `form:"granularity" binding:"omitempty,oneof=day hour"`
	StartDate   string `form:"start_date"`
	EndDate     string `form:"end_date"`
}

// UserTrendPoint 用户聚合趋势的数据点
type UserTrendPoint struct {
	Time  string `json:"time"`
	Count int64  `json:"count"`
}

// UserTrendResponse 用户聚合趋势响应
type UserTrendResponse struct {
	Trend []*UserTrendPoint `json:"trend"`
}

// RegionStatsResponse 地域统计响应体
type RegionStatsResponse struct {
	Name  string `json:"name"`  // 地域名称
	Value int64  `json:"value"` // 对应的点击量
}

// DeviceStatsResponse 设备统计响应体
type DeviceStatsResponse struct {
	Name  string `json:"name"`  // 设备/系统/浏览器名称
	Value int64  `json:"value"` // 对应的点击量
}

// SourceStatsResponse 来源统计响应体
type SourceStatsResponse struct {
	Name  string `json:"name"`  // 来源渠道名称
	Value int64  `json:"value"` // 对应的点击量
}

// GlobalStatsResponse 管理端全局统计响应体
type GlobalStatsResponse struct {
	TotalShortlinks int64          `json:"totalShortlinks"` // 平台短链接总数
	TotalClicks     int64          `json:"totalClicks"`     // 平台总点击量
	ActiveUsers     int64          `json:"activeUsers"`     // 近30天活跃用户数
	TopLinks        []*TopLinkInfo `json:"topLinks"`        // Top 5 热门短链接
}

// TopLinkInfo 热门短链概要信息
type TopLinkInfo struct {
	ShortCode   string `json:"shortCode"`   // 短码
	OriginalUrl string `json:"originalUrl"` // 原始链接（可选）
	ClickCount  int64  `json:"clickCount"`  // 点击量
}
