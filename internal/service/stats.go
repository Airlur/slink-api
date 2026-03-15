package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"slink-api/internal/dto"
	"slink-api/internal/dto/common"
	"slink-api/internal/model"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/response"
	"slink-api/internal/repository"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type StatsService interface {
	GetOverview(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.OverviewStatsResponse, error)
	GetTrend(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetTrendRequest) ([]*dto.TrendStatsResponse, error)
	GetProvinces(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetProvincesStatsRequest) ([]*dto.RegionStatsResponse, error)
	GetCities(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetCitiesStatsRequest) ([]*dto.RegionStatsResponse, error)
	GetDevices(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetDevicesStatsRequest) ([]*dto.DeviceStatsResponse, error)
	GetSources(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetSourcesStatsRequest) ([]*dto.SourceStatsResponse, error)
	GetLogs(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetLogsStatsRequest) (*common.PaginatedData[*model.AccessLog], error)
	GetUserOverview(ctx context.Context, user *jwt.UserInfo) (*dto.UserOverviewStatsResponse, error)
	GetUserTrend(ctx context.Context, user *jwt.UserInfo, req *dto.UserTrendRequest) (*dto.UserTrendResponse, error)
	GetUserRegions(ctx context.Context, user *jwt.UserInfo, req *dto.GetProvincesStatsRequest) ([]*dto.RegionStatsResponse, error)
	GetUserCities(ctx context.Context, user *jwt.UserInfo, req *dto.GetCitiesStatsRequest) ([]*dto.RegionStatsResponse, error)
	GetUserDevices(ctx context.Context, user *jwt.UserInfo, req *dto.GetDevicesStatsRequest) ([]*dto.DeviceStatsResponse, error)
	GetUserSources(ctx context.Context, user *jwt.UserInfo, req *dto.GetSourcesStatsRequest) ([]*dto.SourceStatsResponse, error)
	GetUserTopLinks(ctx context.Context, user *jwt.UserInfo, req *dto.UserTopLinksRequest) ([]*dto.TopLinkInfo, error)
	GetUserDashboardActions(ctx context.Context, user *jwt.UserInfo, req *dto.DashboardActionsRequest) (*dto.DashboardActionsResponse, error)
	GetUserMap(ctx context.Context, user *jwt.UserInfo, req *dto.MapStatsRequest) (*dto.MapStatsResponse, error)
	GetUserSourceTrend(ctx context.Context, user *jwt.UserInfo, req *dto.SourceTrendRequest) (*dto.SourceTrendResponse, error)
	GetUserTagPerformance(ctx context.Context, user *jwt.UserInfo, req *dto.TagPerformanceRequest) ([]*dto.TagPerformanceItem, error)
	GetMap(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.MapStatsRequest) (*dto.MapStatsResponse, error)
	GetCompare(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetTrendRequest) (*dto.ShortlinkCompareResponse, error)
	GetGlobalStats(ctx context.Context) (*dto.GlobalStatsResponse, error)
}

type statsService struct {
	shortlinkRepo repository.ShortlinkRepository
	statsRepo     repository.StatsRepository
	logRepo       repository.LogRepository
}

func NewStatsService(slRepo repository.ShortlinkRepository, stRepo repository.StatsRepository, logRepo repository.LogRepository) StatsService {
	return &statsService{
		shortlinkRepo: slRepo,
		statsRepo:     stRepo,
		logRepo:       logRepo,
	}
}

func (s *statsService) checkShortlinkOwnership(ctx context.Context, user *jwt.UserInfo, shortCode string) (*model.Shortlink, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	sl, err := s.shortlinkRepo.GetStatsMetaByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bizErrors.New(response.ShortlinkNotFound, "shortlink not found")
		}
		logger.Error("failed to load shortlink for stats", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "failed to load stats")
	}
	if sl.UserId != int64(user.ID) {
		logger.Info("stats access denied", "shortlinkUserID", sl.UserId, "currentUserID", user.ID, "shortCode", shortCode)
		return nil, bizErrors.New(response.Forbidden, "forbidden")
	}
	return sl, nil
}

func (s *statsService) GetOverview(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.OverviewStatsResponse, error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	var (
		eg       errgroup.Group
		overview dto.OverviewStatsResponse
	)

	eg.Go(func() error {
		overview.TotalClicks = sl.ClickCount
		return nil
	})

	eg.Go(func() error {
		clicks, err := s.statsRepo.GetClicksByDate(ctx, shortCode, time.Now().Format("2006-01-02"))
		if err != nil {
			logger.Error("failed to load clicksToday", "error", err, "shortCode", shortCode)
			return err
		}
		overview.ClicksToday = clicks
		return nil
	})

	eg.Go(func() error {
		topRegion, err := s.statsRepo.GetTopRegion(ctx, shortCode)
		if err != nil {
			logger.Error("failed to load topRegion", "error", err, "shortCode", shortCode)
			return err
		}
		overview.TopRegion = topRegion
		return nil
	})

	eg.Go(func() error {
		startTime := dayStart(sl.CreatedAt)
		topSource, err := s.statsRepo.GetTopSource(ctx, shortCode, startTime, time.Now())
		if err != nil {
			logger.Error("failed to load topSource", "error", err, "shortCode", shortCode)
			return err
		}
		overview.TopSource = topSource
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, bizErrors.New(response.InternalError, "failed to load stats overview")
	}
	return &overview, nil
}

func (s *statsService) GetTrend(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetTrendRequest) ([]*dto.TrendStatsResponse, error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	if req == nil {
		req = &dto.GetTrendRequest{}
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "7d")
	if err != nil {
		return nil, err
	}

	var raw []*dto.TrendStatsResponse
	switch resolved.Granularity {
	case "hour":
		startTime := resolved.Start
		if startTime.Before(sl.CreatedAt) {
			startTime = sl.CreatedAt
		}
		raw, err = s.statsRepo.GetTrendByHour(ctx, shortCode, startTime, resolved.End)
	default:
		startDate, endDate := normalizeDateRange(resolved.Start, resolved.End)
		raw, err = s.statsRepo.GetTrend(ctx, shortCode, startDate, endDate)
	}
	if err != nil {
		logger.Error("failed to load shortlink trend", "error", err, "shortCode", shortCode, "granularity", resolved.Granularity)
		return nil, bizErrors.New(response.InternalError, "failed to load trend")
	}

	return fillShortlinkTrendData(resolved.Start, resolved.End, resolved.Granularity, raw), nil
}

func (s *statsService) GetProvinces(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetProvincesStatsRequest) ([]*dto.RegionStatsResponse, error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	rangeReq := dto.StatsRangeRequest{}
	if req != nil {
		rangeReq = req.StatsRangeRequest
	}
	startDate, endDate, err := s.resolveAggregateRange(sl, rangeReq)
	if err != nil {
		return nil, err
	}

	provinces, err := s.statsRepo.GetProvinces(ctx, shortCode, startDate, endDate)
	if err != nil {
		logger.Error("failed to load provinces", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "failed to load provinces")
	}
	return provinces, nil
}

func (s *statsService) GetCities(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetCitiesStatsRequest) ([]*dto.RegionStatsResponse, error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	if req == nil {
		req = &dto.GetCitiesStatsRequest{}
	}

	startDate, endDate, err := s.resolveAggregateRange(sl, req.StatsRangeRequest)
	if err != nil {
		return nil, err
	}

	cities, err := s.statsRepo.GetCities(ctx, shortCode, req.Province, startDate, endDate)
	if err != nil {
		logger.Error("failed to load cities", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "failed to load cities")
	}
	return cities, nil
}

func (s *statsService) GetDevices(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetDevicesStatsRequest) ([]*dto.DeviceStatsResponse, error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	if req == nil {
		req = &dto.GetDevicesStatsRequest{}
	}

	startDate, endDate, err := s.resolveAggregateRange(sl, req.StatsRangeRequest)
	if err != nil {
		return nil, err
	}

	devices, err := s.statsRepo.GetDevices(ctx, shortCode, req.Dimension, startDate, endDate)
	if err != nil {
		logger.Error("failed to load devices", "error", err, "shortCode", shortCode, "dimension", req.Dimension)
		return nil, bizErrors.New(response.InternalError, "failed to load devices")
	}
	return devices, nil
}

func (s *statsService) GetSources(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetSourcesStatsRequest) ([]*dto.SourceStatsResponse, error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	rangeReq := dto.StatsRangeRequest{}
	if req != nil {
		rangeReq = req.StatsRangeRequest
	}
	startTime, endTime, err := s.resolveLogRange(sl, rangeReq)
	if err != nil {
		return nil, err
	}

	sources, err := s.statsRepo.GetSources(ctx, shortCode, startTime, endTime)
	if err != nil {
		logger.Error("failed to load sources", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "failed to load sources")
	}
	return sources, nil
}

func (s *statsService) GetLogs(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetLogsStatsRequest) (*common.PaginatedData[*model.AccessLog], error) {
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}

	if req == nil {
		req = &dto.GetLogsStatsRequest{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	startTime, endTime, err := s.resolveLogRange(sl, req.StatsRangeRequest)
	if err != nil {
		return nil, err
	}

	logs, total, err := s.logRepo.ListLogs(ctx, shortCode, startTime, endTime, req.Page, req.Limit)
	if err != nil {
		logger.Error("failed to load logs", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "failed to load logs")
	}

	return &common.PaginatedData[*model.AccessLog]{
		Data: logs,
		Pagination: common.PaginationResponse{
			Total: total,
			Page:  req.Page,
			Limit: req.Limit,
		},
	}, nil
}

func (s *statsService) GetUserOverview(ctx context.Context, user *jwt.UserInfo) (*dto.UserOverviewStatsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}

	totalLinks, err := s.statsRepo.GetUserTotalLinks(ctx, user.ID)
	if err != nil {
		logger.Error("failed to load user totalLinks", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load user stats")
	}

	totalClicks, err := s.statsRepo.GetUserTotalClicks(ctx, user.ID)
	if err != nil {
		logger.Error("failed to load user totalClicks", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load user stats")
	}

	now := time.Now()
	clicksToday, err := s.statsRepo.GetUserClicksByDate(ctx, user.ID, now)
	if err != nil {
		logger.Error("failed to load user clicksToday", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load user stats")
	}

	yesterdayClicks, err := s.statsRepo.GetUserClicksByDate(ctx, user.ID, now.AddDate(0, 0, -1))
	if err != nil {
		logger.Warn("failed to load yesterday clicks", "error", err, "userID", user.ID)
	}

	var growth float64
	if yesterdayClicks > 0 {
		growth = float64(clicksToday-yesterdayClicks) / float64(yesterdayClicks)
	}

	return &dto.UserOverviewStatsResponse{
		TotalLinks:  totalLinks,
		TotalClicks: totalClicks,
		ClicksToday: clicksToday,
		GrowthRate:  growth,
	}, nil
}

func (s *statsService) GetUserTrend(ctx context.Context, user *jwt.UserInfo, req *dto.UserTrendRequest) (*dto.UserTrendResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.UserTrendRequest{}
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "7d")
	if err != nil {
		return nil, err
	}

	var raw []*dto.UserTrendPoint
	switch resolved.Granularity {
	case "hour":
		raw, err = s.statsRepo.GetUserTrendByHour(ctx, user.ID, resolved.Start.Truncate(time.Hour), resolved.End.Truncate(time.Hour))
	default:
		startDate, endDate := normalizeDateRange(resolved.Start, resolved.End)
		raw, err = s.statsRepo.GetUserTrendByDay(ctx, user.ID, startDate, endDate)
	}
	if err != nil {
		logger.Error("failed to load user trend", "error", err, "userID", user.ID, "granularity", resolved.Granularity)
		return nil, bizErrors.New(response.InternalError, "failed to load user trend")
	}

	return &dto.UserTrendResponse{
		Trend: fillUserTrendData(resolved.Start, resolved.End, resolved.Granularity, raw),
	}, nil
}

func (s *statsService) GetGlobalStats(ctx context.Context) (*dto.GlobalStatsResponse, error) {
	var (
		eg          errgroup.Group
		globalStats dto.GlobalStatsResponse
	)

	eg.Go(func() (err error) {
		globalStats.TotalShortlinks, err = s.statsRepo.GetTotalShortlinksCount(ctx)
		return err
	})
	eg.Go(func() (err error) {
		globalStats.TotalClicks, err = s.statsRepo.GetTotalClicksSum(ctx)
		return err
	})
	eg.Go(func() (err error) {
		globalStats.ActiveUsers, err = s.statsRepo.GetActiveUsersCount(ctx, 30)
		return err
	})
	eg.Go(func() (err error) {
		globalStats.TopLinks, err = s.statsRepo.GetTopLinks(ctx, 5)
		return err
	})

	if err := eg.Wait(); err != nil {
		logger.Error("failed to load global stats", "error", err)
		return nil, bizErrors.New(response.InternalError, "failed to load global stats")
	}

	return &globalStats, nil
}

func (s *statsService) GetUserRegions(ctx context.Context, user *jwt.UserInfo, req *dto.GetProvincesStatsRequest) ([]*dto.RegionStatsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}

	rangeReq := dto.StatsRangeRequest{}
	if req != nil {
		rangeReq = req.StatsRangeRequest
	}
	startDate, endDate, err := s.resolveUserAggregateRange(rangeReq)
	if err != nil {
		return nil, err
	}

	results, err := s.statsRepo.GetUserRegions(ctx, user.ID, startDate, endDate)
	if err != nil {
		logger.Error("failed to load user regions", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load user regions")
	}
	return results, nil
}

func (s *statsService) GetUserCities(ctx context.Context, user *jwt.UserInfo, req *dto.GetCitiesStatsRequest) ([]*dto.RegionStatsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.GetCitiesStatsRequest{}
	}

	startDate, endDate, err := s.resolveUserAggregateRange(req.StatsRangeRequest)
	if err != nil {
		return nil, err
	}

	results, err := s.statsRepo.GetUserCities(ctx, user.ID, req.Province, startDate, endDate)
	if err != nil {
		logger.Error("failed to load user cities", "error", err, "userID", user.ID, "province", req.Province)
		return nil, bizErrors.New(response.InternalError, "failed to load user cities")
	}
	return results, nil
}

func (s *statsService) GetUserDevices(ctx context.Context, user *jwt.UserInfo, req *dto.GetDevicesStatsRequest) ([]*dto.DeviceStatsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.GetDevicesStatsRequest{}
	}

	startDate, endDate, err := s.resolveUserAggregateRange(req.StatsRangeRequest)
	if err != nil {
		return nil, err
	}

	results, err := s.statsRepo.GetUserDevices(ctx, user.ID, req.Dimension, startDate, endDate)
	if err != nil {
		logger.Error("failed to load user devices", "error", err, "userID", user.ID, "dimension", req.Dimension)
		return nil, bizErrors.New(response.InternalError, "failed to load user devices")
	}
	return results, nil
}

func (s *statsService) GetUserSources(ctx context.Context, user *jwt.UserInfo, req *dto.GetSourcesStatsRequest) ([]*dto.SourceStatsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}

	rangeReq := dto.StatsRangeRequest{}
	if req != nil {
		rangeReq = req.StatsRangeRequest
	}
	startTime, endTime, err := s.resolveUserLogRange(rangeReq)
	if err != nil {
		return nil, err
	}

	results, err := s.statsRepo.GetUserSources(ctx, user.ID, startTime, endTime)
	if err != nil {
		logger.Error("failed to load user sources", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load user sources")
	}
	return results, nil
}

func (s *statsService) GetUserTopLinks(ctx context.Context, user *jwt.UserInfo, req *dto.UserTopLinksRequest) ([]*dto.TopLinkInfo, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.UserTopLinksRequest{}
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "30d")
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	snapshots, err := s.getUserSnapshotsByRange(ctx, user.ID, resolved.Start, resolved.End, resolved.Granularity)
	if err != nil {
		logger.Error("failed to load user top links", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load user top links")
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].ClickCount == snapshots[j].ClickCount {
			return snapshots[i].ShortCode < snapshots[j].ShortCode
		}
		return snapshots[i].ClickCount > snapshots[j].ClickCount
	})

	if len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}

	results := make([]*dto.TopLinkInfo, 0, len(snapshots))
	for _, snapshot := range snapshots {
		results = append(results, &dto.TopLinkInfo{
			ShortCode:   snapshot.ShortCode,
			OriginalUrl: snapshot.OriginalUrl,
			ClickCount:  snapshot.ClickCount,
		})
	}
	return results, nil
}

func (s *statsService) GetUserDashboardActions(ctx context.Context, user *jwt.UserInfo, req *dto.DashboardActionsRequest) (*dto.DashboardActionsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.DashboardActionsRequest{}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "7d")
	if err != nil {
		return nil, err
	}

	currentSnapshots, err := s.getUserSnapshotsByRange(ctx, user.ID, resolved.Start, resolved.End, resolved.Granularity)
	if err != nil {
		logger.Error("failed to load dashboard action snapshots", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load dashboard actions")
	}

	window := resolved.End.Sub(resolved.Start)
	previousEnd := resolved.Start.Add(-time.Nanosecond)
	previousStart := previousEnd.Add(-window)
	previousSnapshots, err := s.getUserSnapshotsByRange(ctx, user.ID, previousStart, previousEnd, resolved.Granularity)
	if err != nil {
		logger.Error("failed to load previous dashboard action snapshots", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load dashboard actions")
	}

	previousClicks := make(map[string]int64, len(previousSnapshots))
	for _, snapshot := range previousSnapshots {
		previousClicks[snapshot.ShortCode] = snapshot.ClickCount
	}

	risingLinks := make([]*dto.DashboardRisingLink, 0, len(currentSnapshots))
	for _, snapshot := range currentSnapshots {
		if snapshot.ClickCount <= 0 {
			continue
		}
		prev := previousClicks[snapshot.ShortCode]
		delta := snapshot.ClickCount - prev
		growth := 0.0
		if prev > 0 {
			growth = float64(delta) / float64(prev)
		} else if snapshot.ClickCount > 0 {
			growth = 1
		}
		risingLinks = append(risingLinks, &dto.DashboardRisingLink{
			ShortCode:      snapshot.ShortCode,
			OriginalUrl:    snapshot.OriginalUrl,
			CurrentClicks:  snapshot.ClickCount,
			PreviousClicks: prev,
			DeltaClicks:    delta,
			GrowthRate:     growth,
		})
	}
	sort.Slice(risingLinks, func(i, j int) bool {
		if risingLinks[i].DeltaClicks == risingLinks[j].DeltaClicks {
			return risingLinks[i].GrowthRate > risingLinks[j].GrowthRate
		}
		return risingLinks[i].DeltaClicks > risingLinks[j].DeltaClicks
	})
	if len(risingLinks) > limit {
		risingLinks = risingLinks[:limit]
	}

	expiringSoon, err := s.statsRepo.GetUserExpiringSoonLinks(ctx, user.ID, 7, limit)
	if err != nil {
		logger.Error("failed to load expiring links", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load dashboard actions")
	}

	zeroClickLinks, err := s.statsRepo.GetUserZeroClickLinks(ctx, user.ID, limit)
	if err != nil {
		logger.Error("failed to load zero click links", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load dashboard actions")
	}

	return &dto.DashboardActionsResponse{
		ExpiringSoon:   expiringSoon,
		RisingLinks:    risingLinks,
		ZeroClickLinks: zeroClickLinks,
	}, nil
}

func (s *statsService) GetUserMap(ctx context.Context, user *jwt.UserInfo, req *dto.MapStatsRequest) (*dto.MapStatsResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.MapStatsRequest{}
	}

	scope := req.Scope
	if scope == "" {
		scope = "china"
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "30d")
	if err != nil {
		return nil, err
	}

	points, err := s.statsRepo.GetUserMap(ctx, user.ID, scope, resolved.Start, resolved.End, resolved.Granularity)
	if err != nil {
		logger.Error("failed to load user map stats", "error", err, "userID", user.ID, "scope", scope)
		return nil, bizErrors.New(response.InternalError, "failed to load map stats")
	}

	return &dto.MapStatsResponse{Scope: scope, Points: points}, nil
}

func (s *statsService) GetUserSourceTrend(ctx context.Context, user *jwt.UserInfo, req *dto.SourceTrendRequest) (*dto.SourceTrendResponse, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.SourceTrendRequest{}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "30d")
	if err != nil {
		return nil, err
	}

	topSources, err := s.statsRepo.GetUserSources(ctx, user.ID, resolved.Start, resolved.End)
	if err != nil {
		logger.Error("failed to load top sources for source trend", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load source trend")
	}
	if len(topSources) > limit {
		topSources = topSources[:limit]
	}

	sourceNames := make([]string, 0, len(topSources))
	for _, item := range topSources {
		sourceNames = append(sourceNames, item.Name)
	}

	series, err := s.statsRepo.GetUserSourceTrend(ctx, user.ID, resolved.Start, resolved.End, resolved.Granularity, sourceNames)
	if err != nil {
		logger.Error("failed to load source trend", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load source trend")
	}

	return &dto.SourceTrendResponse{
		Sources:     sourceNames,
		Granularity: resolved.Granularity,
		Series:      fillSourceTrendSeries(resolved.Start, resolved.End, resolved.Granularity, sourceNames, series),
	}, nil
}

func (s *statsService) GetUserTagPerformance(ctx context.Context, user *jwt.UserInfo, req *dto.TagPerformanceRequest) ([]*dto.TagPerformanceItem, error) {
	if user == nil {
		return nil, bizErrors.New(response.Unauthorized, "login required")
	}
	if req == nil {
		req = &dto.TagPerformanceRequest{}
	}

	startDate, endDate, err := s.resolveUserAggregateRange(req.StatsRangeRequest)
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}

	items, err := s.statsRepo.GetUserTagPerformance(ctx, user.ID, startDate, endDate, limit)
	if err != nil {
		logger.Error("failed to load tag performance", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.InternalError, "failed to load tag performance")
	}
	return items, nil
}

func (s *statsService) GetMap(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.MapStatsRequest) (*dto.MapStatsResponse, error) {
	_, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &dto.MapStatsRequest{}
	}

	scope := req.Scope
	if scope == "" {
		scope = "china"
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "30d")
	if err != nil {
		return nil, err
	}

	points, err := s.statsRepo.GetShortlinkMap(ctx, shortCode, scope, resolved.Start, resolved.End, resolved.Granularity)
	if err != nil {
		logger.Error("failed to load shortlink map stats", "error", err, "shortCode", shortCode, "scope", scope)
		return nil, bizErrors.New(response.InternalError, "failed to load map stats")
	}

	return &dto.MapStatsResponse{Scope: scope, Points: points}, nil
}

func (s *statsService) GetCompare(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetTrendRequest) (*dto.ShortlinkCompareResponse, error) {
	_, err := s.checkShortlinkOwnership(ctx, user, shortCode)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &dto.GetTrendRequest{}
	}

	resolved, err := resolveStatsRange(req.StatsRangeRequest, time.Now(), "7d")
	if err != nil {
		return nil, err
	}

	snapshots, err := s.getUserSnapshotsByRange(ctx, user.ID, resolved.Start, resolved.End, resolved.Granularity)
	if err != nil {
		logger.Error("failed to load compare snapshots", "error", err, "userID", user.ID, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "failed to load compare stats")
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].ClickCount == snapshots[j].ClickCount {
			return snapshots[i].ShortCode < snapshots[j].ShortCode
		}
		return snapshots[i].ClickCount > snapshots[j].ClickCount
	})

	var (
		totalClicks int64
		rangeClicks int64
		rankInRange int
	)
	for index, snapshot := range snapshots {
		totalClicks += snapshot.ClickCount
		if snapshot.ShortCode == shortCode {
			rangeClicks = snapshot.ClickCount
			rankInRange = index + 1
		}
	}
	if rankInRange == 0 {
		rankInRange = len(snapshots)
	}

	totalRankedLinks := len(snapshots)
	average := 0.0
	if totalRankedLinks > 0 {
		average = float64(totalClicks) / float64(totalRankedLinks)
	}

	share := 0.0
	if totalClicks > 0 {
		share = float64(rangeClicks) / float64(totalClicks)
	}

	versusAverage := 0.0
	if average > 0 {
		versusAverage = float64(rangeClicks) / average
	}

	return &dto.ShortlinkCompareResponse{
		RangeClicks:          rangeClicks,
		AccountShareRate:     share,
		RankInRange:          rankInRange,
		TotalRankedLinks:     totalRankedLinks,
		AverageClicksPerLink: average,
		VersusAverageRatio:   versusAverage,
	}, nil
}

func (s *statsService) resolveAggregateRange(sl *model.Shortlink, req dto.StatsRangeRequest) (time.Time, time.Time, error) {
	if !req.HasExplicitRange() {
		return dayStart(sl.CreatedAt), dayEnd(time.Now()), nil
	}

	resolved, err := resolveStatsRange(req, time.Now(), "30d")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	startDate, endDate := normalizeDateRange(resolved.Start, resolved.End)
	return startDate, endDate, nil
}

func (s *statsService) resolveLogRange(sl *model.Shortlink, req dto.StatsRangeRequest) (time.Time, time.Time, error) {
	if !req.HasExplicitRange() {
		return dayStart(sl.CreatedAt), time.Now(), nil
	}

	resolved, err := resolveStatsRange(req, time.Now(), "30d")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return resolved.Start, resolved.End, nil
}

func (s *statsService) resolveUserAggregateRange(req dto.StatsRangeRequest) (time.Time, time.Time, error) {
	resolved, err := resolveStatsRange(req, time.Now(), "30d")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	startDate, endDate := normalizeDateRange(resolved.Start, resolved.End)
	return startDate, endDate, nil
}

func (s *statsService) resolveUserLogRange(req dto.StatsRangeRequest) (time.Time, time.Time, error) {
	resolved, err := resolveStatsRange(req, time.Now(), "30d")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return resolved.Start, resolved.End, nil
}

func fillUserTrendData(start, end time.Time, granularity string, raw []*dto.UserTrendPoint) []*dto.UserTrendPoint {
	data := make(map[string]int64)
	for _, point := range raw {
		data[point.Time] += point.Count
	}

	step, format, normalizedStart, normalizedEnd := trendSeriesBounds(start, end, granularity)

	result := make([]*dto.UserTrendPoint, 0)
	for ts := normalizedStart; !ts.After(normalizedEnd); ts = ts.Add(step) {
		key := ts.Format(format)
		result = append(result, &dto.UserTrendPoint{
			Time:  key,
			Count: data[key],
		})
	}
	return result
}

func fillShortlinkTrendData(start, end time.Time, granularity string, raw []*dto.TrendStatsResponse) []*dto.TrendStatsResponse {
	data := make(map[string]int64)
	for _, point := range raw {
		data[point.Date] += point.Clicks
	}

	step, format, normalizedStart, normalizedEnd := trendSeriesBounds(start, end, granularity)

	result := make([]*dto.TrendStatsResponse, 0)
	for ts := normalizedStart; !ts.After(normalizedEnd); ts = ts.Add(step) {
		key := ts.Format(format)
		result = append(result, &dto.TrendStatsResponse{
			Date:   key,
			Clicks: data[key],
		})
	}
	return result
}

func trendSeriesBounds(start, end time.Time, granularity string) (time.Duration, string, time.Time, time.Time) {
	if granularity == "hour" {
		return time.Hour, "2006-01-02 15:00:00", start.Truncate(time.Hour), end.Truncate(time.Hour)
	}
	return 24 * time.Hour, "2006-01-02", dayStart(start), dayStart(end)
}

func (s *statsService) getUserSnapshotsByRange(ctx context.Context, userID uint, start, end time.Time, granularity string) ([]*dto.LinkClicksSnapshot, error) {
	if granularity == "hour" {
		return s.statsRepo.GetUserLinkSnapshotsByHour(ctx, userID, start, end)
	}
	startDate, endDate := normalizeDateRange(start, end)
	return s.statsRepo.GetUserLinkSnapshotsByDay(ctx, userID, startDate, endDate)
}

func fillSourceTrendSeries(start, end time.Time, granularity string, sourceOrder []string, raw []*dto.SourceTrendSeries) []*dto.SourceTrendSeries {
	step, format, normalizedStart, normalizedEnd := trendSeriesBounds(start, end, granularity)
	rawMap := make(map[string]map[string]int64, len(raw))
	for _, series := range raw {
		points := make(map[string]int64, len(series.Points))
		for _, point := range series.Points {
			points[point.Time] += point.Count
		}
		rawMap[series.Source] = points
	}

	result := make([]*dto.SourceTrendSeries, 0, len(sourceOrder))
	for _, source := range sourceOrder {
		seriesData := rawMap[source]
		points := make([]*dto.SourceTrendPoint, 0)
		for ts := normalizedStart; !ts.After(normalizedEnd); ts = ts.Add(step) {
			key := ts.Format(format)
			points = append(points, &dto.SourceTrendPoint{
				Time:  key,
				Count: seriesData[key],
			})
		}
		result = append(result, &dto.SourceTrendSeries{
			Source: source,
			Points: points,
		})
	}
	return result
}
