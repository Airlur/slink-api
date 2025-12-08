package service

import (
	"context"
	"errors"
	"time"

	"short-link/internal/dto"
	"short-link/internal/dto/common"
	"short-link/internal/model"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/response"
	"short-link/internal/repository"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// StatsService 定义了统计服务接口
type StatsService interface {
	GetOverview(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.OverviewStatsResponse, error)
	GetTrend(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetTrendRequest) ([]*dto.TrendStatsResponse, error)
	GetProvinces(ctx context.Context, user *jwt.UserInfo, shortCode string) ([]*dto.RegionStatsResponse, error)
	GetCities(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetCitiesStatsRequest) ([]*dto.RegionStatsResponse, error)
	GetDevices(ctx context.Context, user *jwt.UserInfo, shortCode, dimension string) ([]*dto.DeviceStatsResponse, error)
	GetSources(ctx context.Context, user *jwt.UserInfo, shortCode string) ([]*dto.SourceStatsResponse, error)
	GetLogs(ctx context.Context, user *jwt.UserInfo, shortCode string, req *common.PaginationRequest) (*common.PaginatedData[*model.AccessLog], error)
	GetGlobalStats(ctx context.Context) (*dto.GlobalStatsResponse, error)
}

type statsService struct {
	shortlinkRepo repository.ShortlinkRepository
	statsRepo     repository.StatsRepository
	logRepo       repository.LogRepository
}

// NewStatsService 创建一个新的统计服务实例
func NewStatsService(slRepo repository.ShortlinkRepository, stRepo repository.StatsRepository, logRepo repository.LogRepository) StatsService {
	return &statsService{
		shortlinkRepo: slRepo,
		statsRepo:     stRepo,
		logRepo:       logRepo,
	}
}

// checkShortlinkOwnership 是一个内部辅助函数，用于校验短链接所有权
func (s *statsService) checkShortlinkOwnership(ctx context.Context, user *jwt.UserInfo, shortCode string) (*model.Shortlink, error) {
	// 注意：对于统计查询，我们总是需要关联数据，所以preload固定为true
	sl, err := s.shortlinkRepo.GetUniqueShortCode(ctx, shortCode, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		logger.Error("获取短链接信息失败 (in statsService)", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "获取统计信息失败")
	}
	if sl.UserId != int64(user.ID) {
		logger.Info("鉴权失败", "短链接创建用户ID", sl.UserId, "当前登录用户ID", user.ID)
		return nil, bizErrors.New(response.Forbidden, "无权查看此短链接的统计信息")
	}
	// 成功时，返回查询到的 shortlink 对象
	return sl, nil
}

// GetOverview 获取概览统计数据
func (s *statsService) GetOverview(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.OverviewStatsResponse, error) {
	// 1. 校验短链接所有权
	sl, err := s.checkShortlinkOwnership(ctx, user, shortCode); 
	if err != nil {
		return nil, err
	}

	// 2. 使用 errgroup 并发执行多个统计查询，提升性能
	var eg errgroup.Group
	var overview dto.OverviewStatsResponse

	// a. 查询总点击量
	eg.Go(func() error {
		// 直接从 shortlinks 表主记录获取，这是最快的
		overview.TotalClicks = sl.ClickCount
		return nil
	})

	// b. 查询今日点击量
	eg.Go(func() error {
		today := time.Now().Format("2006-01-02")
		clicks, err := s.statsRepo.GetClicksByDate(ctx, shortCode, today)
		if err != nil {
			// 在并发查询中，我们只记录日志，最终由 eg.Wait() 统一返回一个通用错误
			logger.Error("获取今日点击量失败", "error", err, "shortCode", shortCode)
			return err
		}
		overview.ClicksToday = clicks
		return nil
	})
	
	// c. 查询Top地域
	eg.Go(func() error {
		region, err := s.statsRepo.GetTopRegion(ctx, shortCode)
		if err != nil {
			logger.Error("获取Top地域失败", "error", err, "shortCode", shortCode)
			return err
		}
		overview.TopRegion = region
		return nil
	})

	// 3. 等待所有并发查询完成
	if err := eg.Wait(); err != nil {
		// 如果任何一个goroutine返回了错误，eg.Wait()就会返回第一个遇到的错误
		return nil, bizErrors.New(response.InternalError, "获取统计概览数据失败")
	}

	overview.TopSource = "暂未实现" // 填充占位符
	return &overview, nil
}

// GetTrend 获取点击趋势
func (s *statsService) GetTrend(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetTrendRequest) ([]*dto.TrendStatsResponse, error) {
	// 1. 校验短链接所有权
	if _, err := s.checkShortlinkOwnership(ctx, user, shortCode); err != nil {
		return nil, err
	}

	// 2. 解析时间范围
	endDate := time.Now()
	var startDate time.Time
	var err error

	switch req.Period {
	case "7d":
		startDate = endDate.AddDate(0, 0, -6)
	case "30d":
		startDate = endDate.AddDate(0, 0, -29)
	default:
		if req.Start != "" && req.End != "" {
			startDate, err = time.Parse("2006-01-02", req.Start)
			if err != nil { return nil, bizErrors.New(response.InvalidParam, "开始日期格式错误") }
			endDate, err = time.Parse("2006-01-02", req.End)
			if err != nil { return nil, bizErrors.New(response.InvalidParam, "结束日期格式错误") }
		} else {
			startDate = endDate.AddDate(0, 0, -6)
		}
	}

	// 3. 调用Repository查询
	trend, err := s.statsRepo.GetTrend(ctx, shortCode, startDate, endDate)
	if err != nil {
		logger.Error("获取点击趋势失败", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "获取统计数据失败")
	}
	return trend, nil
}

// GetProvinces 获取省级统计
func (s *statsService) GetProvinces(ctx context.Context, user *jwt.UserInfo, shortCode string) ([]*dto.RegionStatsResponse, error) {
	if _, err := s.checkShortlinkOwnership(ctx, user, shortCode); err != nil {
		return nil, err
	}

	endDate := time.Now()
	startDate := time.Time{} // 零值时间表示不限制开始时间

	provinces, err := s.statsRepo.GetProvinces(ctx, shortCode, startDate, endDate)
	if err != nil {
		logger.Error("获取省级统计失败", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "获取统计数据失败")
	}
	return provinces, nil
}

// GetCities 获取市级统计
func (s *statsService) GetCities(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.GetCitiesStatsRequest) ([]*dto.RegionStatsResponse, error) {
	if _, err := s.checkShortlinkOwnership(ctx, user, shortCode); err != nil {
		return nil, err
	}

	endDate := time.Now()
	startDate := time.Time{}

	cities, err := s.statsRepo.GetCities(ctx, shortCode, req.Province, startDate, endDate)
	if err != nil {
		logger.Error("获取市级统计失败", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "获取统计数据失败")
	}
	return cities, nil
}

// 【新增】GetDevices 获取设备统计
func (s *statsService) GetDevices(ctx context.Context, user *jwt.UserInfo, shortCode, dimension string) ([]*dto.DeviceStatsResponse, error) {
	if _, err := s.checkShortlinkOwnership(ctx, user, shortCode); err != nil {
		return nil, err
	}
	// 默认查询历史所有数据
	endDate := time.Now()
	startDate := time.Time{}
	return s.statsRepo.GetDevices(ctx, shortCode, dimension, startDate, endDate)
}

// 【新增】GetSources 获取来源统计
func (s *statsService) GetSources(ctx context.Context, user *jwt.UserInfo, shortCode string) ([]*dto.SourceStatsResponse, error) {
	if _, err := s.checkShortlinkOwnership(ctx, user, shortCode); err != nil {
		return nil, err
	}
	endDate := time.Now()
	startDate := time.Time{}
	return s.statsRepo.GetSources(ctx, shortCode, startDate, endDate)
}

// 【新增】GetLogs 获取原始日志
func (s *statsService) GetLogs(ctx context.Context, user *jwt.UserInfo, shortCode string, req *common.PaginationRequest) (*common.PaginatedData[*model.AccessLog], error) {
	if _, err := s.checkShortlinkOwnership(ctx, user, shortCode); err != nil {
		return nil, err
	}
	if req.Page <= 0 { req.Page = 1 }
	if req.Limit <= 0 { req.Limit = 10 }
	
	logs, total, err := s.logRepo.ListLogs(ctx, shortCode, req.Page, req.Limit)
	if err != nil {
		logger.Error("获取原始日志失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "获取日志失败")
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


// GetGlobalStats 获取平台全局统计
func (s *statsService) GetGlobalStats(ctx context.Context) (*dto.GlobalStatsResponse, error) {
	// 使用 errgroup 并发执行所有全局统计查询
	var eg errgroup.Group
	var globalStats dto.GlobalStatsResponse

	eg.Go(func() (err error) {
		globalStats.TotalShortlinks, err = s.statsRepo.GetTotalShortlinksCount(ctx)
		return err
	})
	eg.Go(func() (err error) {
		globalStats.TotalClicks, err = s.statsRepo.GetTotalClicksSum(ctx)
		return err
	})
	eg.Go(func() (err error) {
		globalStats.ActiveUsers, err = s.statsRepo.GetActiveUsersCount(ctx, 30) // 统计近30天
		return err
	})
	eg.Go(func() (err error) {
		globalStats.TopLinks, err = s.statsRepo.GetTopLinks(ctx, 5) // 获取Top 5
		return err
	})

	if err := eg.Wait(); err != nil {
		logger.Error("获取全局统计数据失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "获取全局统计失败")
	}

	return &globalStats, nil
}