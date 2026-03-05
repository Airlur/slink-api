package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"slink-api/internal/dto"
	"slink-api/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StatsRepository interface {
	// IncrementClicks 原子性地增加多个维度的点击计数
	IncrementClicks(ctx context.Context, log *model.AccessLog) error
	// GetTotalClicks 获取一个短链接的总点击量
	GetTotalClicks(ctx context.Context, shortCode string) (int64, error)
	// GetClicksByDate 获取一个短链接在指定日期的点击量
	GetClicksByDate(ctx context.Context, shortCode string, date string) (int64, error)
	// GetTopRegion 获取一个短链接点击量最高的地域
	GetTopRegion(ctx context.Context, shortCode string) (string, error)
	// GetTrend 获取指定时间范围内的点击趋势
	GetTrend(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.TrendStatsResponse, error)
	GetProvinces(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error)
	GetCities(ctx context.Context, shortCode string, province string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error)
	GetDevices(ctx context.Context, shortCode string, dimension string, startDate, endDate time.Time) ([]*dto.DeviceStatsResponse, error)
	GetSources(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.SourceStatsResponse, error)
	GetUserTotalLinks(ctx context.Context, userID uint) (int64, error)
	GetUserTotalClicks(ctx context.Context, userID uint) (int64, error)
	GetUserClicksByDate(ctx context.Context, userID uint, date time.Time) (int64, error)
	GetUserTrendByDay(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.UserTrendPoint, error)
	GetUserTrendByHour(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.UserTrendPoint, error)
	// 管理员调用接口
	GetTotalShortlinksCount(ctx context.Context) (int64, error)
	GetTotalClicksSum(ctx context.Context) (int64, error)
	GetActiveUsersCount(ctx context.Context, days int) (int64, error)
	GetTopLinks(ctx context.Context, limit int) ([]*dto.TopLinkInfo, error)
}

type statsRepository struct {
	db *gorm.DB
}

func NewStatsRepository(db *gorm.DB) StatsRepository {
	return &statsRepository{db: db}
}

// IncrementClicks 使用事务来保证所有统计表更新的原子性
func (r *statsRepository) IncrementClicks(ctx context.Context, log *model.AccessLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 更新 shortlinks 表中的总点击数
		if err := tx.Model(&model.Shortlink{}).Where("short_code = ?", log.ShortCode).
			UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error; err != nil {
			return err
		}

		// 2. 更新 stats_daily 表
		dailyStat := model.StatsDaily{
			ShortCode: log.ShortCode,
			Date:      log.AccessedAt,
			Clicks:    1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + 1")}),
		}).Create(&dailyStat).Error; err != nil {
			return err
		}

		// 3. 更新 stats_region_daily 表
		regionStat := model.StatsRegionDaily{
			ShortCode: log.ShortCode,
			Date:      log.AccessedAt,
			Province:  log.Province,
			City:      log.City,
			Clicks:    1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "province"}, {Name: "city"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + 1")}),
		}).Create(&regionStat).Error; err != nil {
			return err
		}

		// 4. 更新 stats_device_daily 表
		deviceStat := model.StatsDeviceDaily{
			ShortCode:  log.ShortCode,
			Date:       log.AccessedAt,
			DeviceType: log.DeviceType,
			OsVersion:  log.OsVersion,
			Browser:    log.Browser,
			Clicks:     1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "device_type"}, {Name: "os_version"}, {Name: "browser"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + 1")}),
		}).Create(&deviceStat).Error; err != nil {
			return err
		}
		
		return nil
	})
}

// GetTotalClicks 从 stats_daily 表中汇总总点击量
func (r *statsRepository) GetTotalClicks(ctx context.Context, shortCode string) (int64, error) {
	var totalClicks int64
	err := r.db.WithContext(ctx).Model(&model.StatsDaily{}).
		Where("short_code = ?", shortCode).
		Select("SUM(clicks)").
		Row().
		Scan(&totalClicks)
	return totalClicks, err
}

// GetClicksByDate 从 stats_daily 表中获取指定日期的点击量
func (r *statsRepository) GetClicksByDate(ctx context.Context, shortCode string, date string) (int64, error) {
	var clicks int64
	// GORM 在处理 time.Time 时会自动格式化为 YYYY-MM-DD
	err := r.db.WithContext(ctx).Model(&model.StatsDaily{}).
		Where("short_code = ? AND date = ?", shortCode, date).
		Select("clicks").
		Row().
		Scan(&clicks)
	return clicks, err
}
 

// GetTopRegion 从 stats_region_daily 表中获取点击量最高的省份
func (r *statsRepository) GetTopRegion(ctx context.Context, shortCode string) (string, error) {
	var topRegion struct {
		Province string
	}
	err := r.db.WithContext(ctx).Model(&model.StatsRegionDaily{}).
		Where("short_code = ?", shortCode).
		Select("province").
		Group("province").
		Order("SUM(clicks) DESC").
		Limit(1).
		Scan(&topRegion).Error
		
	if err != nil {
		return "未知", err
	}
	if topRegion.Province == "" {
		return "暂无数据", nil
	}
	return topRegion.Province, nil
}


// GetTrend 从 stats_daily 表中获取点击趋势
func (r *statsRepository) GetTrend(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.TrendStatsResponse, error) {
	var trendData []*dto.TrendStatsResponse
	err := r.db.WithContext(ctx).Model(&model.StatsDaily{}).
		Select("DATE_FORMAT(date, '%Y-%m-%d') as date, clicks").
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate).
		Order("date ASC").
		Scan(&trendData).Error
	return trendData, err
}

// GetProvinces 从 stats_region_daily 表中按省份分组统计点击量
func (r *statsRepository) GetProvinces(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error) {
	var results []*dto.RegionStatsResponse
	err := r.db.WithContext(ctx).Model(&model.StatsRegionDaily{}).
		Select("province as name, SUM(clicks) as value").
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate).
		Group("province").
		Order("value DESC").
		Scan(&results).Error
	return results, err
}

// GetCities 从 stats_region_daily 表中按城市分组统计点击量
func (r *statsRepository) GetCities(ctx context.Context, shortCode string, province string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error) {
	var results []*dto.RegionStatsResponse
	
	db := r.db.WithContext(ctx).Model(&model.StatsRegionDaily{}).
		Select("city as name, SUM(clicks) as value").
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate)

	// 如果提供了省份参数，则增加筛选条件
	if province != "" {
		db = db.Where("province = ?", province)
	}

	err := db.Group("city").
		Order("value DESC").
		Limit(20). // 限制最多返回Top 20的城市
		Scan(&results).Error
	return results, err
}

// GetDevices 从 stats_device_daily 表中按指定维度分组统计
func (r *statsRepository) GetDevices(ctx context.Context, shortCode string, dimension string, startDate, endDate time.Time) ([]*dto.DeviceStatsResponse, error) {
	var results []*dto.DeviceStatsResponse
	
	// 根据维度选择要查询和分组的列
	var selectColumn string
	switch dimension {
	case "device_type":
		selectColumn = "device_type"
	case "os_version":
		selectColumn = "os_version"
	case "browser":
		selectColumn = "browser"
	default:
		return nil, errors.New("invalid dimension for device stats")
	}

	err := r.db.WithContext(ctx).Model(&model.StatsDeviceDaily{}).
		Select(fmt.Sprintf("%s as name, SUM(clicks) as value", selectColumn)).
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate).
		Group(selectColumn).
		Order("value DESC").
		Limit(10). // 限制最多返回Top 10
		Scan(&results).Error
	return results, err
}

// GetSources 从 access_logs 分表中按来源渠道分组统计
// 注意：这是一个示例，它直接查询原始日志，性能较低。
// 生产环境中，来源渠道也应该有自己的预聚合汇总表。
func (r *statsRepository) GetSources(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.SourceStatsResponse, error) {
	var results []*dto.SourceStatsResponse
	
	// 来源分析暂时直接查当月的日志表作为演示
	tableName := fmt.Sprintf("access_logs_%s", time.Now().Format("200601"))

	err := r.db.WithContext(ctx).Table(tableName).
		Select("channel as name, COUNT(*) as value").
		Where("short_code = ? AND accessed_at BETWEEN ? AND ?", shortCode, startDate, endDate).
		Group("channel").
		Order("value DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

// GetUserTotalLinks 统计用户短链接数量
func (r *statsRepository) GetUserTotalLinks(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// GetUserTotalClicks 汇总用户短链接点击量
func (r *statsRepository) GetUserTotalClicks(ctx context.Context, userID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).
		Select("COALESCE(SUM(click_count), 0)").
		Where("user_id = ?", userID).
		Row().
		Scan(&total)
	return total, err
}

// GetUserClicksByDate 获取用户短链在指定日期的点击数
func (r *statsRepository) GetUserClicksByDate(ctx context.Context, userID uint, date time.Time) (int64, error) {
	var clicks int64
	err := r.db.WithContext(ctx).Table("stats_daily sd").
		Select("COALESCE(SUM(sd.clicks), 0)").
		Joins("JOIN shortlinks s ON sd.short_code = s.short_code").
		Where("s.user_id = ?", userID).
		Where("sd.date = ?", date.Format("2006-01-02")).
		Row().
		Scan(&clicks)
	return clicks, err
}

// GetUserTrendByDay 汇总用户日级趋势
func (r *statsRepository) GetUserTrendByDay(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.UserTrendPoint, error) {
	var rows []struct {
		Time  string
		Count int64
	}
	err := r.db.WithContext(ctx).Table("stats_daily sd").
		Select("DATE_FORMAT(sd.date, '%Y-%m-%d') AS time, SUM(sd.clicks) AS count").
		Joins("JOIN shortlinks s ON sd.short_code = s.short_code").
		Where("s.user_id = ?", userID).
		Where("sd.date BETWEEN ? AND ?", startDate, endDate).
		Group("time").
		Order("time ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	points := make([]*dto.UserTrendPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, &dto.UserTrendPoint{
			Time:  row.Time,
			Count: row.Count,
		})
	}
	return points, nil
}

// GetUserTrendByHour 汇总用户小时级趋势
func (r *statsRepository) GetUserTrendByHour(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.UserTrendPoint, error) {
	buckets := make(map[string]int64)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []struct {
			Time  string
			Count int64
		}
		err := r.db.WithContext(ctx).Table(tableName).
			Select("DATE_FORMAT(accessed_at, '%Y-%m-%d %H:00:00') AS time, COUNT(*) AS count").
			Where("user_id = ?", userID).
			Where("accessed_at BETWEEN ? AND ?", startTime, endTime).
			Group("time").
			Order("time ASC").
			Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			buckets[row.Time] += row.Count
		}
	}

	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	points := make([]*dto.UserTrendPoint, 0, len(keys))
	for _, key := range keys {
		points = append(points, &dto.UserTrendPoint{
			Time:  key,
			Count: buckets[key],
		})
	}
	return points, nil
}

// ======================= 管理员调用 =======================

// GetTotalShortlinksCount 获取平台短链接总数
func (r *statsRepository) GetTotalShortlinksCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).Count(&count).Error
	return count, err
}

// GetTotalClicksSum 获取平台总点击量
func (r *statsRepository) GetTotalClicksSum(ctx context.Context) (int64, error) {
	var totalClicks int64
	// 直接从 shortlinks 主表汇总，性能最高
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).Select("SUM(click_count)").Row().Scan(&totalClicks)
	return totalClicks, err
}

// GetActiveUsersCount 获取近期活跃用户数
func (r *statsRepository) GetActiveUsersCount(ctx context.Context, days int) (int64, error) {
	var count int64
	// 定义：近期有过登录的用户
	threshold := time.Now().AddDate(0, 0, -days)
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("last_login_at > ?", threshold).Count(&count).Error
	return count, err
}

// GetTopLinks 获取热门短链接
func (r *statsRepository) GetTopLinks(ctx context.Context, limit int) ([]*dto.TopLinkInfo, error) {
	var topLinks []*dto.TopLinkInfo
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).
		Select("short_code, original_url, click_count").
		Order("click_count DESC").
		Limit(limit).
		Scan(&topLinks).Error
	return topLinks, err
}
