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
			name := strings.TrimSpace(row.Name)
			if name == "" {
				name = "Direct"
			}
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
			name := strings.TrimSpace(row.Name)
			if name == "" {
				name = "Direct"
			}
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
