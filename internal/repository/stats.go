package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"slink-api/internal/dto"
	"slink-api/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StatsRepository interface {
	IncrementClicks(ctx context.Context, log *model.AccessLog) error
	GetTotalClicks(ctx context.Context, shortCode string) (int64, error)
	GetClicksByDate(ctx context.Context, shortCode string, date string) (int64, error)
	GetTopRegion(ctx context.Context, shortCode string) (string, error)
	GetTopSource(ctx context.Context, shortCode string, startTime, endTime time.Time) (string, error)
	GetTrend(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.TrendStatsResponse, error)
	GetTrendByHour(ctx context.Context, shortCode string, startTime, endTime time.Time) ([]*dto.TrendStatsResponse, error)
	GetProvinces(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error)
	GetCities(ctx context.Context, shortCode string, province string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error)
	GetDevices(ctx context.Context, shortCode string, dimension string, startDate, endDate time.Time) ([]*dto.DeviceStatsResponse, error)
	GetSources(ctx context.Context, shortCode string, startTime, endTime time.Time) ([]*dto.SourceStatsResponse, error)
	GetUserTotalLinks(ctx context.Context, userID uint) (int64, error)
	GetUserTotalClicks(ctx context.Context, userID uint) (int64, error)
	GetUserClicksByDate(ctx context.Context, userID uint, date time.Time) (int64, error)
	GetUserTrendByDay(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.UserTrendPoint, error)
	GetUserTrendByHour(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.UserTrendPoint, error)
	GetUserRegions(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error)
	GetUserCities(ctx context.Context, userID uint, province string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error)
	GetUserDevices(ctx context.Context, userID uint, dimension string, startDate, endDate time.Time) ([]*dto.DeviceStatsResponse, error)
	GetUserSources(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.SourceStatsResponse, error)
	GetUserTopLinksByRange(ctx context.Context, userID uint, startDate, endDate time.Time, limit int) ([]*dto.TopLinkInfo, error)
	GetUserExpiringSoonLinks(ctx context.Context, userID uint, days, limit int) ([]*dto.DashboardExpiringLink, error)
	GetUserZeroClickLinks(ctx context.Context, userID uint, limit int) ([]*dto.DashboardZeroClickLink, error)
	GetUserLinkSnapshotsByDay(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.LinkClicksSnapshot, error)
	GetUserLinkSnapshotsByHour(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.LinkClicksSnapshot, error)
	GetUserMap(ctx context.Context, userID uint, scope string, startTime, endTime time.Time, granularity string) ([]*dto.MapStatsPoint, error)
	GetShortlinkMap(ctx context.Context, shortCode string, scope string, startTime, endTime time.Time, granularity string) ([]*dto.MapStatsPoint, error)
	GetUserSourceTrend(ctx context.Context, userID uint, startTime, endTime time.Time, granularity string, sources []string) ([]*dto.SourceTrendSeries, error)
	GetUserTagPerformance(ctx context.Context, userID uint, startDate, endDate time.Time, limit int) ([]*dto.TagPerformanceItem, error)
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

func (r *statsRepository) IncrementClicks(ctx context.Context, log *model.AccessLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Shortlink{}).
			Where("short_code = ?", log.ShortCode).
			UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error; err != nil {
			return err
		}

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

		regionStat := model.StatsRegionDaily{
			ShortCode: log.ShortCode,
			Date:      log.AccessedAt,
			Country:   normalizeMapName(log.Country),
			Province:  log.Province,
			City:      log.City,
			Clicks:    1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "country"}, {Name: "province"}, {Name: "city"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + 1")}),
		}).Create(&regionStat).Error; err != nil {
			return err
		}

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

func (r *statsRepository) GetTotalClicks(ctx context.Context, shortCode string) (int64, error) {
	var totalClicks int64
	err := r.db.WithContext(ctx).Model(&model.StatsDaily{}).
		Where("short_code = ?", shortCode).
		Select("COALESCE(SUM(clicks), 0)").
		Row().
		Scan(&totalClicks)
	return totalClicks, err
}

func (r *statsRepository) GetClicksByDate(ctx context.Context, shortCode string, date string) (int64, error) {
	var clicks int64
	err := r.db.WithContext(ctx).Model(&model.StatsDaily{}).
		Where("short_code = ? AND date = ?", shortCode, date).
		Select("COALESCE(SUM(clicks), 0)").
		Row().
		Scan(&clicks)
	return clicks, err
}

func (r *statsRepository) GetTopRegion(ctx context.Context, shortCode string) (string, error) {
	var row struct {
		Province string
	}
	err := r.db.WithContext(ctx).Model(&model.StatsRegionDaily{}).
		Select("province").
		Where("short_code = ?", shortCode).
		Group("province").
		Order("SUM(clicks) DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "Unknown", err
	}
	if strings.TrimSpace(row.Province) == "" {
		return "No data", nil
	}
	return row.Province, nil
}

func (r *statsRepository) GetTopSource(ctx context.Context, shortCode string, startTime, endTime time.Time) (string, error) {
	sources, err := r.GetSources(ctx, shortCode, startTime, endTime)
	if err != nil {
		return "Unknown", err
	}
	if len(sources) == 0 {
		return "No data", nil
	}
	return sources[0].Name, nil
}

func (r *statsRepository) GetTrend(ctx context.Context, shortCode string, startDate, endDate time.Time) ([]*dto.TrendStatsResponse, error) {
	var trendData []*dto.TrendStatsResponse
	err := r.db.WithContext(ctx).Model(&model.StatsDaily{}).
		Select("DATE_FORMAT(date, '%Y-%m-%d') as date, clicks").
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate).
		Order("date ASC").
		Scan(&trendData).Error
	return trendData, err
}

func (r *statsRepository) GetTrendByHour(ctx context.Context, shortCode string, startTime, endTime time.Time) ([]*dto.TrendStatsResponse, error) {
	buckets := make(map[string]int64)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []struct {
			Date   string
			Clicks int64
		}
		err := r.db.WithContext(ctx).Table(tableName).
			Select("DATE_FORMAT(accessed_at, '%Y-%m-%d %H:00:00') AS date, COUNT(*) AS clicks").
			Where("short_code = ?", shortCode).
			Where("accessed_at BETWEEN ? AND ?", startTime, endTime).
			Group("date").
			Order("date ASC").
			Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			buckets[row.Date] += row.Clicks
		}
	}

	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]*dto.TrendStatsResponse, 0, len(keys))
	for _, key := range keys {
		result = append(result, &dto.TrendStatsResponse{
			Date:   key,
			Clicks: buckets[key],
		})
	}
	return result, nil
}

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

func (r *statsRepository) GetCities(ctx context.Context, shortCode string, province string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error) {
	var results []*dto.RegionStatsResponse

	query := r.db.WithContext(ctx).Model(&model.StatsRegionDaily{}).
		Select("city as name, SUM(clicks) as value").
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate)

	if province != "" {
		query = query.Where("province = ?", province)
	}

	err := query.Group("city").
		Order("value DESC").
		Limit(20).
		Scan(&results).Error
	return results, err
}

func (r *statsRepository) GetDevices(ctx context.Context, shortCode string, dimension string, startDate, endDate time.Time) ([]*dto.DeviceStatsResponse, error) {
	var results []*dto.DeviceStatsResponse

	selectColumn, err := normalizeDeviceDimension(dimension)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Model(&model.StatsDeviceDaily{}).
		Select(fmt.Sprintf("%s as name, SUM(clicks) as value", selectColumn)).
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startDate, endDate).
		Group(selectColumn).
		Order("value DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

func (r *statsRepository) GetSources(ctx context.Context, shortCode string, startTime, endTime time.Time) ([]*dto.SourceStatsResponse, error) {
	if startTime.IsZero() {
		return nil, errors.New("startTime is required for source stats")
	}

	buckets := make(map[string]int64)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []*dto.SourceStatsResponse
		err := r.db.WithContext(ctx).Table(tableName).
			Select("channel as name, COUNT(*) as value").
			Where("short_code = ? AND accessed_at BETWEEN ? AND ?", shortCode, startTime, endTime).
			Group("channel").
			Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			name := normalizeSourceName(row.Name)
			buckets[name] += row.Value
		}
	}

	results := make([]*dto.SourceStatsResponse, 0, len(buckets))
	for name, value := range buckets {
		results = append(results, &dto.SourceStatsResponse{
			Name:  name,
			Value: value,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Value == results[j].Value {
			return results[i].Name < results[j].Name
		}
		return results[i].Value > results[j].Value
	})
	if len(results) > 10 {
		results = results[:10]
	}
	return results, nil
}

func (r *statsRepository) GetUserTotalLinks(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

func (r *statsRepository) GetUserTotalClicks(ctx context.Context, userID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).
		Select("COALESCE(SUM(click_count), 0)").
		Where("user_id = ?", userID).
		Row().
		Scan(&total)
	return total, err
}

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
		err := r.db.WithContext(ctx).Table(tableName+" al").
			Select("DATE_FORMAT(al.accessed_at, '%Y-%m-%d %H:00:00') AS time, COUNT(*) AS count").
			Joins("JOIN shortlinks s ON al.short_code = s.short_code").
			Where("s.user_id = ?", userID).
			Where("al.accessed_at BETWEEN ? AND ?", startTime, endTime).
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
	for key := range buckets {
		keys = append(keys, key)
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

func (r *statsRepository) GetUserRegions(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error) {
	var results []*dto.RegionStatsResponse
	err := r.db.WithContext(ctx).Table("stats_region_daily srd").
		Select("srd.province as name, SUM(srd.clicks) as value").
		Joins("JOIN shortlinks s ON srd.short_code = s.short_code").
		Where("s.user_id = ?", userID).
		Where("srd.date BETWEEN ? AND ?", startDate, endDate).
		Group("srd.province").
		Order("value DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

func (r *statsRepository) GetUserCities(ctx context.Context, userID uint, province string, startDate, endDate time.Time) ([]*dto.RegionStatsResponse, error) {
	var results []*dto.RegionStatsResponse
	query := r.db.WithContext(ctx).Table("stats_region_daily srd").
		Select("srd.city as name, SUM(srd.clicks) as value").
		Joins("JOIN shortlinks s ON srd.short_code = s.short_code").
		Where("s.user_id = ?", userID).
		Where("srd.date BETWEEN ? AND ?", startDate, endDate)

	if province != "" {
		query = query.Where("srd.province = ?", province)
	}

	err := query.Group("srd.city").
		Order("value DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

func (r *statsRepository) GetUserDevices(ctx context.Context, userID uint, dimension string, startDate, endDate time.Time) ([]*dto.DeviceStatsResponse, error) {
	var results []*dto.DeviceStatsResponse

	selectColumn, err := normalizeDeviceDimension(dimension)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Table("stats_device_daily sdd").
		Select(fmt.Sprintf("sdd.%s as name, SUM(sdd.clicks) as value", selectColumn)).
		Joins("JOIN shortlinks s ON sdd.short_code = s.short_code").
		Where("s.user_id = ?", userID).
		Where("sdd.date BETWEEN ? AND ?", startDate, endDate).
		Group(fmt.Sprintf("sdd.%s", selectColumn)).
		Order("value DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

func (r *statsRepository) GetUserSources(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.SourceStatsResponse, error) {
	if startTime.IsZero() {
		return nil, errors.New("startTime is required for user source stats")
	}

	buckets := make(map[string]int64)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []*dto.SourceStatsResponse
		err := r.db.WithContext(ctx).Table(tableName+" al").
			Select("al.channel as name, COUNT(*) as value").
			Joins("JOIN shortlinks s ON al.short_code = s.short_code").
			Where("s.user_id = ? AND al.accessed_at BETWEEN ? AND ?", userID, startTime, endTime).
			Group("al.channel").
			Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			name := normalizeSourceName(row.Name)
			buckets[name] += row.Value
		}
	}

	results := make([]*dto.SourceStatsResponse, 0, len(buckets))
	for name, value := range buckets {
		results = append(results, &dto.SourceStatsResponse{Name: name, Value: value})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Value == results[j].Value {
			return results[i].Name < results[j].Name
		}
		return results[i].Value > results[j].Value
	})
	if len(results) > 10 {
		results = results[:10]
	}
	return results, nil
}

func (r *statsRepository) GetUserTopLinksByRange(ctx context.Context, userID uint, startDate, endDate time.Time, limit int) ([]*dto.TopLinkInfo, error) {
	if limit <= 0 {
		limit = 5
	}

	var topLinks []*dto.TopLinkInfo
	err := r.db.WithContext(ctx).Table("stats_daily sd").
		Select("sd.short_code, s.original_url, SUM(sd.clicks) as click_count").
		Joins("JOIN shortlinks s ON sd.short_code = s.short_code").
		Where("s.user_id = ?", userID).
		Where("sd.date BETWEEN ? AND ?", startDate, endDate).
		Group("sd.short_code, s.original_url").
		Order("click_count DESC").
		Limit(limit).
		Scan(&topLinks).Error
	return topLinks, err
}

func (r *statsRepository) GetTotalShortlinksCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).Count(&count).Error
	return count, err
}

func (r *statsRepository) GetTotalClicksSum(ctx context.Context) (int64, error) {
	var totalClicks int64
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).
		Select("COALESCE(SUM(click_count), 0)").
		Row().
		Scan(&totalClicks)
	return totalClicks, err
}

func (r *statsRepository) GetActiveUsersCount(ctx context.Context, days int) (int64, error) {
	var count int64
	threshold := time.Now().AddDate(0, 0, -days)
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("last_login_at > ?", threshold).
		Count(&count).Error
	return count, err
}

func (r *statsRepository) GetTopLinks(ctx context.Context, limit int) ([]*dto.TopLinkInfo, error) {
	var topLinks []*dto.TopLinkInfo
	err := r.db.WithContext(ctx).Model(&model.Shortlink{}).
		Select("short_code, original_url, click_count").
		Order("click_count DESC").
		Limit(limit).
		Scan(&topLinks).Error
	return topLinks, err
}

func normalizeDeviceDimension(dimension string) (string, error) {
	switch dimension {
	case "", "device_type":
		return "device_type", nil
	case "os", "os_version":
		return "os_version", nil
	case "browser":
		return "browser", nil
	default:
		return "", errors.New("invalid dimension for device stats")
	}
}

func (r *statsRepository) GetUserExpiringSoonLinks(ctx context.Context, userID uint, days, limit int) ([]*dto.DashboardExpiringLink, error) {
	if days <= 0 {
		days = 7
	}
	if limit <= 0 {
		limit = 5
	}

	now := time.Now()
	deadline := now.AddDate(0, 0, days)

	var rows []struct {
		ShortCode   string
		OriginalUrl string
		ExpireAt    *time.Time
		ClickCount  int64
	}
	err := r.db.WithContext(ctx).Table("shortlinks s").
		Select("s.short_code, s.original_url, s.expire_at, s.click_count").
		Where("s.user_id = ?", userID).
		Where("s.deleted_at IS NULL").
		Where("s.status = 1").
		Where("s.expire_at IS NOT NULL AND s.expire_at BETWEEN ? AND ?", now, deadline).
		Order("s.expire_at ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]*dto.DashboardExpiringLink, 0, len(rows))
	for _, row := range rows {
		remainingDays := 0
		if row.ExpireAt != nil {
			diff := row.ExpireAt.Sub(now)
			if diff > 0 {
				remainingDays = int((diff + 24*time.Hour - time.Nanosecond) / (24 * time.Hour))
			}
		}
		result = append(result, &dto.DashboardExpiringLink{
			ShortCode:     row.ShortCode,
			OriginalUrl:   row.OriginalUrl,
			ExpireAt:      row.ExpireAt,
			RemainingDays: remainingDays,
			ClickCount:    row.ClickCount,
		})
	}
	return result, nil
}

func (r *statsRepository) GetUserZeroClickLinks(ctx context.Context, userID uint, limit int) ([]*dto.DashboardZeroClickLink, error) {
	if limit <= 0 {
		limit = 5
	}

	var rows []struct {
		ShortCode   string
		OriginalUrl string
		CreatedAt   time.Time
	}
	err := r.db.WithContext(ctx).Table("shortlinks s").
		Select("s.short_code, s.original_url, s.created_at").
		Where("s.user_id = ?", userID).
		Where("s.deleted_at IS NULL").
		Where("s.status = 1 AND s.click_count = 0").
		Order("s.created_at ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	now := time.Now()
	result := make([]*dto.DashboardZeroClickLink, 0, len(rows))
	for _, row := range rows {
		ageDays := int(now.Sub(row.CreatedAt) / (24 * time.Hour))
		if ageDays < 0 {
			ageDays = 0
		}
		result = append(result, &dto.DashboardZeroClickLink{
			ShortCode:   row.ShortCode,
			OriginalUrl: row.OriginalUrl,
			CreatedAt:   row.CreatedAt,
			AgeDays:     ageDays,
		})
	}
	return result, nil
}

func (r *statsRepository) GetUserLinkSnapshotsByDay(ctx context.Context, userID uint, startDate, endDate time.Time) ([]*dto.LinkClicksSnapshot, error) {
	var snapshots []*dto.LinkClicksSnapshot
	err := r.db.WithContext(ctx).Table("shortlinks s").
		Select(`
			s.short_code,
			s.original_url,
			s.expire_at,
			s.created_at,
			COALESCE(agg.click_count, 0) AS click_count
		`).
		Joins(`
			LEFT JOIN (
				SELECT short_code, SUM(clicks) AS click_count
				FROM stats_daily
				WHERE date BETWEEN ? AND ?
				GROUP BY short_code
			) agg ON agg.short_code = s.short_code
		`, startDate, endDate).
		Where("s.user_id = ?", userID).
		Where("s.deleted_at IS NULL").
		Scan(&snapshots).Error
	return snapshots, err
}

func (r *statsRepository) GetUserLinkSnapshotsByHour(ctx context.Context, userID uint, startTime, endTime time.Time) ([]*dto.LinkClicksSnapshot, error) {
	snapshots, err := r.listUserLinkBase(ctx, userID)
	if err != nil {
		return nil, err
	}

	clicksByCode, err := r.collectUserHourlyLinkClicks(ctx, userID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	for _, snapshot := range snapshots {
		snapshot.ClickCount = clicksByCode[snapshot.ShortCode]
	}
	return snapshots, nil
}

func (r *statsRepository) GetUserMap(ctx context.Context, userID uint, scope string, startTime, endTime time.Time, granularity string) ([]*dto.MapStatsPoint, error) {
	if granularity == "hour" {
		return r.collectUserMapFromLogs(ctx, userID, scope, startTime, endTime)
	}
	return r.collectUserMapFromDaily(ctx, userID, scope, startTime, endTime)
}

func (r *statsRepository) GetShortlinkMap(ctx context.Context, shortCode string, scope string, startTime, endTime time.Time, granularity string) ([]*dto.MapStatsPoint, error) {
	if granularity == "hour" {
		return r.collectShortlinkMapFromLogs(ctx, shortCode, scope, startTime, endTime)
	}
	return r.collectShortlinkMapFromDaily(ctx, shortCode, scope, startTime, endTime)
}

func (r *statsRepository) GetUserSourceTrend(ctx context.Context, userID uint, startTime, endTime time.Time, granularity string, sources []string) ([]*dto.SourceTrendSeries, error) {
	if len(sources) == 0 {
		return []*dto.SourceTrendSeries{}, nil
	}

	normalizedSources := make([]string, 0, len(sources))
	sourceSet := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		normalized := normalizeSourceName(source)
		if normalized == "" {
			normalized = "direct"
		}
		if _, exists := sourceSet[normalized]; exists {
			continue
		}
		sourceSet[normalized] = struct{}{}
		normalizedSources = append(normalizedSources, normalized)
	}

	data := make(map[string]map[string]int64, len(normalizedSources))
	for _, source := range normalizedSources {
		data[source] = make(map[string]int64)
	}

	bucketExpr, bucketFormat := bucketExprForGranularity(granularity)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []struct {
			Source string
			Bucket string
			Count  int64
		}
		err := r.db.WithContext(ctx).Table(tableName+" al").
			Select(fmt.Sprintf("al.channel AS source, %s AS bucket, COUNT(*) AS count", bucketExpr)).
			Joins("JOIN shortlinks s ON al.short_code = s.short_code").
			Where("s.user_id = ? AND s.deleted_at IS NULL", userID).
			Where("al.accessed_at BETWEEN ? AND ?", startTime, endTime).
			Where("al.channel IN ?", normalizedSources).
			Group("source, bucket").
			Order("bucket ASC").
			Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			source := normalizeSourceName(row.Source)
			data[source][row.Bucket] += row.Count
		}
	}

	series := make([]*dto.SourceTrendSeries, 0, len(normalizedSources))
	for _, source := range normalizedSources {
		buckets := data[source]
		keys := make([]string, 0, len(buckets))
		for key := range buckets {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		points := make([]*dto.SourceTrendPoint, 0, len(keys))
		for _, key := range keys {
			points = append(points, &dto.SourceTrendPoint{
				Time:  formatBucketTime(key, bucketFormat),
				Count: buckets[key],
			})
		}

		series = append(series, &dto.SourceTrendSeries{
			Source: source,
			Points: points,
		})
	}

	return series, nil
}

func (r *statsRepository) GetUserTagPerformance(ctx context.Context, userID uint, startDate, endDate time.Time, limit int) ([]*dto.TagPerformanceItem, error) {
	if limit <= 0 {
		limit = 8
	}

	var items []*dto.TagPerformanceItem
	err := r.db.WithContext(ctx).Table("stats_daily sd").
		Select(`
			CASE
				WHEN t.tag_name IS NULL OR t.tag_name = '' THEN '未分类'
				ELSE t.tag_name
			END AS tag,
			SUM(sd.clicks) AS clicks,
			COUNT(DISTINCT s.short_code) AS link_count,
			ROUND(SUM(sd.clicks) / NULLIF(COUNT(DISTINCT s.short_code), 0), 2) AS avg_clicks_per_link
		`).
		Joins("JOIN shortlinks s ON sd.short_code = s.short_code").
		Joins("LEFT JOIN tags t ON t.short_code = s.short_code AND t.deleted_at IS NULL").
		Where("s.user_id = ? AND s.deleted_at IS NULL", userID).
		Where("sd.date BETWEEN ? AND ?", startDate, endDate).
		Group("tag").
		Order("clicks DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *statsRepository) listUserLinkBase(ctx context.Context, userID uint) ([]*dto.LinkClicksSnapshot, error) {
	var snapshots []*dto.LinkClicksSnapshot
	err := r.db.WithContext(ctx).Table("shortlinks s").
		Select("s.short_code, s.original_url, s.expire_at, s.created_at, 0 AS click_count").
		Where("s.user_id = ?", userID).
		Where("s.deleted_at IS NULL").
		Scan(&snapshots).Error
	return snapshots, err
}

func (r *statsRepository) collectUserHourlyLinkClicks(ctx context.Context, userID uint, startTime, endTime time.Time) (map[string]int64, error) {
	clicksByCode := make(map[string]int64)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []struct {
			ShortCode string
			Clicks    int64
		}
		err := r.db.WithContext(ctx).Table(tableName+" al").
			Select("al.short_code, COUNT(*) AS clicks").
			Joins("JOIN shortlinks s ON al.short_code = s.short_code").
			Where("s.user_id = ? AND s.deleted_at IS NULL", userID).
			Where("al.accessed_at BETWEEN ? AND ?", startTime, endTime).
			Group("al.short_code").
			Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			clicksByCode[row.ShortCode] += row.Clicks
		}
	}

	return clicksByCode, nil
}

func (r *statsRepository) collectUserMapFromDaily(ctx context.Context, userID uint, scope string, startTime, endTime time.Time) ([]*dto.MapStatsPoint, error) {
	column := "srd.province"
	query := r.db.WithContext(ctx).Table("stats_region_daily srd").
		Joins("JOIN shortlinks s ON srd.short_code = s.short_code").
		Where("s.user_id = ? AND s.deleted_at IS NULL", userID).
		Where("srd.date BETWEEN ? AND ?", startTime, endTime)

	if scope == "world" {
		column = "srd.country"
	} else {
		query = query.Where("srd.country = ?", "China")
	}

	var points []*dto.MapStatsPoint
	err := query.
		Select(fmt.Sprintf("%s AS name, SUM(srd.clicks) AS value", column)).
		Group(column).
		Order("value DESC").
		Scan(&points).Error
	if err != nil {
		return nil, err
	}
	return filterMapPoints(points), nil
}

func (r *statsRepository) collectShortlinkMapFromDaily(ctx context.Context, shortCode string, scope string, startTime, endTime time.Time) ([]*dto.MapStatsPoint, error) {
	column := "country"
	if scope != "world" {
		column = "province"
	}

	query := r.db.WithContext(ctx).Table("stats_region_daily").
		Where("short_code = ? AND date BETWEEN ? AND ?", shortCode, startTime, endTime)
	if scope != "world" {
		query = query.Where("country = ?", "China")
	}

	var points []*dto.MapStatsPoint
	err := query.Select(fmt.Sprintf("%s AS name, SUM(clicks) AS value", column)).
		Group(column).
		Order("value DESC").
		Scan(&points).Error
	if err != nil {
		return nil, err
	}
	return filterMapPoints(points), nil
}

func (r *statsRepository) collectUserMapFromLogs(ctx context.Context, userID uint, scope string, startTime, endTime time.Time) ([]*dto.MapStatsPoint, error) {
	return r.collectMapFromLogs(ctx, scope, startTime, endTime, func(db *gorm.DB) *gorm.DB {
		return db.Joins("JOIN shortlinks s ON al.short_code = s.short_code").
			Where("s.user_id = ? AND s.deleted_at IS NULL", userID)
	})
}

func (r *statsRepository) collectShortlinkMapFromLogs(ctx context.Context, shortCode string, scope string, startTime, endTime time.Time) ([]*dto.MapStatsPoint, error) {
	return r.collectMapFromLogs(ctx, scope, startTime, endTime, func(db *gorm.DB) *gorm.DB {
		return db.Where("al.short_code = ?", shortCode)
	})
}

func (r *statsRepository) collectMapFromLogs(ctx context.Context, scope string, startTime, endTime time.Time, decorate func(*gorm.DB) *gorm.DB) ([]*dto.MapStatsPoint, error) {
	column := "al.country"
	if scope != "world" {
		column = "al.province"
	}

	buckets := make(map[string]int64)
	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := fmt.Sprintf("access_logs_%s", current.Format("200601"))
		var rows []*dto.MapStatsPoint
		query := r.db.WithContext(ctx).Table(tableName + " al").
			Select(fmt.Sprintf("%s AS name, COUNT(*) AS value", column)).
			Where("al.accessed_at BETWEEN ? AND ?", startTime, endTime)
		if scope != "world" {
			query = query.Where("al.country = ?", "China")
		}
		if decorate != nil {
			query = decorate(query)
		}
		err := query.Group("name").Scan(&rows).Error
		if err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, err
		}
		for _, row := range rows {
			name := normalizeMapName(row.Name)
			buckets[name] += row.Value
		}
	}

	return mapBucketsToPoints(buckets), nil
}

func mapBucketsToPoints(buckets map[string]int64) []*dto.MapStatsPoint {
	points := make([]*dto.MapStatsPoint, 0, len(buckets))
	for name, value := range buckets {
		if value <= 0 {
			continue
		}
		name = normalizeMapName(name)
		if name == "Unknown" {
			continue
		}
		points = append(points, &dto.MapStatsPoint{Name: name, Value: value})
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].Value == points[j].Value {
			return points[i].Name < points[j].Name
		}
		return points[i].Value > points[j].Value
	})
	return points
}

func filterMapPoints(points []*dto.MapStatsPoint) []*dto.MapStatsPoint {
	buckets := make(map[string]int64, len(points))
	for _, point := range points {
		name := normalizeMapName(point.Name)
		if name == "Unknown" {
			continue
		}
		buckets[name] += point.Value
	}
	return mapBucketsToPoints(buckets)
}

func bucketExprForGranularity(granularity string) (string, string) {
	if granularity == "hour" {
		return "DATE_FORMAT(al.accessed_at, '%Y-%m-%d %H:00:00')", "2006-01-02 15:00:00"
	}
	return "DATE_FORMAT(al.accessed_at, '%Y-%m-%d')", "2006-01-02"
}

func formatBucketTime(value, format string) string {
	ts, err := time.ParseInLocation(format, value, time.Local)
	if err != nil {
		return value
	}
	return ts.Format(format)
}

func normalizeMapName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Unknown"
	}
	return value
}

func normalizeSourceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "direct"
	}
	if strings.EqualFold(value, "direct") {
		return "direct"
	}
	return value
}
