package service

import (
	"context"
	"fmt"
	"regexp"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/logger"
	"time"

	"gorm.io/gorm"
)

type MaintenanceService interface {
	CleanupOldLogs(ctx context.Context) error
}

type maintenanceService struct {
	db *gorm.DB
}

func NewMaintenanceService(db *gorm.DB) MaintenanceService {
	return &maintenanceService{db: db}
}

// CleanupOldLogs 实现了清理过期日志分表的功能
func (s *maintenanceService) CleanupOldLogs(ctx context.Context) error {
	logger.Info("开始执行 [清理过期日志] 定时任务")

	// 1. 计算保留期限的阈值
	// 例如，如果保留90天，今天是10月19日，那么90天前是7月21日，所有7月之前的表（如 access_logs_202506）都应被删除
	retentionDays := config.GlobalConfig.Lifecycle.LogRetentionDays
	thresholdDate := time.Now().AddDate(0, 0, -retentionDays)

	// 2. 查询数据库中所有 access_logs_YYYYMM 格式的表
	var tableNames []string
	// 使用Raw SQL查询information_schema，这是最直接的方式
	err := s.db.WithContext(ctx).
		Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE 'access_logs_%'").
		Scan(&tableNames).Error
	if err != nil {
		logger.Error("查询日志分表列表失败", "error", err)
		return err
	}

	// 3. 遍历所有找到的表，判断是否过期
	logTableRegex := regexp.MustCompile(`^access_logs_(\d{6})$`)
	var tablesToDrop []string
	for _, tableName := range tableNames {
		matches := logTableRegex.FindStringSubmatch(tableName)
		if len(matches) < 2 {
			continue // 表名格式不匹配，跳过
		}
		
		// 从表名 "access_logs_202507" 中解析出日期 "202507"
		tableMonthStr := matches[1]
		tableTime, err := time.Parse("200601", tableMonthStr)
		if err != nil {
			logger.Warn("解析日志分表月份失败", "tableName", tableName, "error", err)
			continue
		}
		
		// 如果这张表的月份，是在我们的保留阈值日期之前，则标记为待删除
		if tableTime.Before(thresholdDate) {
			tablesToDrop = append(tablesToDrop, tableName)
		}
	}

	// 4. 执行删除
	if len(tablesToDrop) > 0 {
		logger.Info("发现过期的日志分表，准备删除", "tables", tablesToDrop)
		for _, tableName := range tablesToDrop {
			dropSQL := fmt.Sprintf("DROP TABLE `%s`;", tableName)
			if err := s.db.WithContext(ctx).Exec(dropSQL).Error; err != nil {
				// 记录错误，但继续尝试删除其他表
				logger.Error("删除过期日志分表失败", "error", err, "tableName", tableName)
			} else {
				logger.Info("成功删除过期日志分表", "tableName", tableName)
			}
		}
	} else {
		logger.Info("没有发现需要清理的过期日志分表")
	}

	logger.Info("[清理过期日志] 定时任务执行完毕")
	return nil
}