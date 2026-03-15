package dto

import (
	"slink-api/internal/dto/common"
	"slink-api/internal/model"
)

type AccessLogListResponse struct {
	Data       []*model.AccessLog        `json:"data"`
	Pagination common.PaginationResponse `json:"pagination"`
}

type OverviewStatsResponse struct {
	TotalClicks int64  `json:"totalClicks"`
	ClicksToday int64  `json:"clicksToday"`
	TopRegion   string `json:"topRegion"`
	TopSource   string `json:"topSource"`
}

type StatsRangeRequest struct {
	Period      string `form:"period"`
	Range       string `form:"range"`
	Granularity string `form:"granularity" binding:"omitempty,oneof=day hour auto"`
	StartDate   string `form:"start_date"`
	EndDate     string `form:"end_date"`
	Start       string `form:"start"`
	End         string `form:"end"`
}

func (r StatsRangeRequest) RequestedPeriod() string {
	if r.Period != "" {
		return r.Period
	}
	return r.Range
}

func (r StatsRangeRequest) RequestedStart() string {
	if r.StartDate != "" {
		return r.StartDate
	}
	return r.Start
}

func (r StatsRangeRequest) RequestedEnd() string {
	if r.EndDate != "" {
		return r.EndDate
	}
	return r.End
}

func (r StatsRangeRequest) HasExplicitRange() bool {
	return r.RequestedPeriod() != "" || r.RequestedStart() != "" || r.RequestedEnd() != ""
}

type GetTrendRequest struct {
	StatsRangeRequest
}

type GetProvincesStatsRequest struct {
	StatsRangeRequest
}

type GetCitiesStatsRequest struct {
	Province string `form:"province"`
	StatsRangeRequest
}

type GetDevicesStatsRequest struct {
	Dimension string `form:"dimension" binding:"omitempty,oneof=device_type os os_version browser"`
	StatsRangeRequest
}

type GetSourcesStatsRequest struct {
	StatsRangeRequest
}

type GetLogsStatsRequest struct {
	common.PaginationRequest
	StatsRangeRequest
}

type TrendStatsResponse struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type UserOverviewStatsResponse struct {
	TotalLinks  int64   `json:"totalLinks"`
	TotalClicks int64   `json:"totalClicks"`
	ClicksToday int64   `json:"clicksToday"`
	GrowthRate  float64 `json:"growthRate"`
}

type UserTrendRequest struct {
	StatsRangeRequest
}

type UserTopLinksRequest struct {
	Limit int `form:"limit"`
	StatsRangeRequest
}

type UserTrendPoint struct {
	Time  string `json:"time"`
	Count int64  `json:"count"`
}

type UserTrendResponse struct {
	Trend []*UserTrendPoint `json:"trend"`
}

type RegionStatsResponse struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type DeviceStatsResponse struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type SourceStatsResponse struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type GlobalStatsResponse struct {
	TotalShortlinks int64          `json:"totalShortlinks"`
	TotalClicks     int64          `json:"totalClicks"`
	ActiveUsers     int64          `json:"activeUsers"`
	TopLinks        []*TopLinkInfo `json:"topLinks"`
}

type TopLinkInfo struct {
	ShortCode   string `json:"shortCode"`
	OriginalUrl string `json:"originalUrl"`
	ClickCount  int64  `json:"clickCount"`
}
