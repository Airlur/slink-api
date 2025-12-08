package dto

// OverviewStatsResponse 概览统计响应体
type OverviewStatsResponse struct {
	TotalClicks int64  `json:"totalClicks"` // 短链接历史总点击量
	ClicksToday int64  `json:"clicksToday"` // 今日点击量
	TopRegion   string `json:"topRegion"`   // 点击量最高的地域（省份/国家）
	TopSource   string `json:"topSource"`   // 点击量最高的来源（暂未实现）
}

// GetTrendRequest 定义了获取点击趋势的查询参数
type GetTrendRequest struct {
	// 时间周期，例如 "7d", "30d"。如果为空，则使用 start 和 end
	Period string `form:"period"`
	// 自定义开始日期 (格式: YYYY-MM-DD)
	Start string `form:"start"`
	// 自定义结束日期 (格式: YYYY-MM-DD)
	End string `form:"end"`
}

// GetCitiesStatsRequest 定义了获取市级数据的查询参数
type GetCitiesStatsRequest struct {
	Province string `form:"province"` // 按省份筛选
}


// TrendStatsResponse 点击趋势响应体
type TrendStatsResponse struct {
	Date  string `json:"date"`  // 日期 (YYYY-MM-DD)
	Clicks int64  `json:"clicks"` // 当日点击量
}


// RegionStatsResponse 地域统计响应体（可用于省份和城市）
type RegionStatsResponse struct {
	Name  string `json:"name"`  // 地域名称（如 "广东省" 或 "深圳市"）
	Value int64  `json:"value"` // 对应的点击量
}

// DeviceStatsResponse 用于设备统计的响应体
type DeviceStatsResponse struct {
	Name  string `json:"name"`  // 设备/系统/浏览器名称
	Value int64  `json:"value"` // 对应的点击量
}

// SourceStatsResponse 用于来源统计的响应体
type SourceStatsResponse struct {
	Name  string `json:"name"`  // 来源渠道名称
	Value int64  `json:"value"` // 对应的点击量
}

// GlobalStatsResponse 定义了管理员全局统计的响应体
type GlobalStatsResponse struct {
	TotalShortlinks int64        `json:"totalShortlinks"` // 平台短链接总数
	TotalClicks     int64        `json:"totalClicks"`     // 平台总点击量
	ActiveUsers     int64        `json:"activeUsers"`     // 近30天活跃用户数
	TopLinks        []*TopLinkInfo `json:"topLinks"`        // Top 5 热门短链接
}

// TopLinkInfo 热门短链接的简化信息
type TopLinkInfo struct {
	ShortCode   string `json:"shortCode"`   // 短码
	OriginalUrl string `json:"originalUrl"` // 原始链接（可选）
	ClickCount  int64  `json:"clickCount"`  // 点击量
}