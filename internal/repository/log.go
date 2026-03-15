package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"slink-api/internal/model"
	"slink-api/internal/pkg/logger"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type LogRepository interface {
	Create(ctx context.Context, log *model.AccessLog) error
	CreateInBatches(ctx context.Context, logs []*model.AccessLog) error
	GetRecentLogs(ctx context.Context, shortCode string, limit int) ([]*model.AccessLog, error)
	ListLogs(ctx context.Context, shortCode string, startTime, endTime time.Time, page, limit int) ([]*model.AccessLog, int64, error)
}

type logRepository struct {
	db            *gorm.DB
	createdTables sync.Map
}

func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

func (r *logRepository) getTableName(t time.Time) string {
	return fmt.Sprintf("access_logs_%s", t.Format("200601"))
}

func (r *logRepository) ensureTableExists(ctx context.Context, tableName string, tableTime time.Time) error {
	if _, ok := r.createdTables.Load(tableName); ok {
		return nil
	}

	if !r.db.WithContext(ctx).Migrator().HasTable(tableName) {
		createSQL := fmt.Sprintf("CREATE TABLE `%s` LIKE `access_logs_template`;", tableName)
		if err := r.db.WithContext(ctx).Exec(createSQL).Error; err != nil {
			return err
		}

		comment := fmt.Sprintf("access log partition (%s)", tableTime.Format("2006-01"))
		alterSQL := fmt.Sprintf("ALTER TABLE `%s` COMMENT='%s';", tableName, comment)
		if err := r.db.WithContext(ctx).Exec(alterSQL).Error; err != nil {
			logger.Warn("failed to update log table comment", "error", err, "tableName", tableName)
		}
	}

	r.createdTables.Store(tableName, true)
	return nil
}

func isTableNotExistError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146
}

func (r *logRepository) Create(ctx context.Context, log *model.AccessLog) error {
	tableName := r.getTableName(log.AccessedAt)
	if err := r.ensureTableExists(ctx, tableName, log.AccessedAt); err != nil {
		return err
	}

	err := r.db.WithContext(ctx).Table(tableName).Create(log).Error
	if err != nil && isTableNotExistError(err) {
		r.createdTables.Delete(tableName)
		logger.Warn("log table cache became stale, retrying create", "tableName", tableName)
		return r.Create(ctx, log)
	}
	return err
}

func (r *logRepository) CreateInBatches(ctx context.Context, logs []*model.AccessLog) error {
	if len(logs) == 0 {
		return nil
	}

	logsByTable := make(map[string][]*model.AccessLog)
	for _, log := range logs {
		tableName := r.getTableName(log.AccessedAt)
		logsByTable[tableName] = append(logsByTable[tableName], log)
	}

	const batchSize = 500

	for tableName, batchLogs := range logsByTable {
		if err := r.ensureTableExists(ctx, tableName, batchLogs[0].AccessedAt); err != nil {
			return err
		}

		for i := 0; i < len(batchLogs); i += batchSize {
			end := i + batchSize
			if end > len(batchLogs) {
				end = len(batchLogs)
			}
			currentBatch := batchLogs[i:end]

			err := r.db.WithContext(ctx).Table(tableName).Create(currentBatch).Error
			if err != nil && isTableNotExistError(err) {
				r.createdTables.Delete(tableName)
				if ensureErr := r.ensureTableExists(ctx, tableName, currentBatch[0].AccessedAt); ensureErr != nil {
					return ensureErr
				}
				err = r.db.WithContext(ctx).Table(tableName).Create(currentBatch).Error
			}
			if err != nil {
				return err
			}
		}

		logger.Info("batched log insert completed", "tableName", tableName, "rows", len(batchLogs))
	}

	return nil
}

func (r *logRepository) GetRecentLogs(ctx context.Context, shortCode string, limit int) ([]*model.AccessLog, error) {
	if limit <= 0 {
		limit = 10
	}
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -89)
	logs, _, err := r.listLogsAcrossTables(ctx, shortCode, startTime, endTime, 1, limit)
	return logs, err
}

func (r *logRepository) ListLogs(ctx context.Context, shortCode string, startTime, endTime time.Time, page, limit int) ([]*model.AccessLog, int64, error) {
	if startTime.IsZero() {
		startTime = endTime.AddDate(0, 0, -89)
	}
	if endTime.IsZero() {
		endTime = time.Now()
	}
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}
	return r.listLogsAcrossTables(ctx, shortCode, startTime, endTime, page, limit)
}

func (r *logRepository) listLogsAcrossTables(ctx context.Context, shortCode string, startTime, endTime time.Time, page, limit int) ([]*model.AccessLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	need := page * limit
	offset := (page - 1) * limit
	var (
		total  int64
		merged []*model.AccessLog
	)

	startMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	endMonth := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, endTime.Location())

	for current := startMonth; !current.After(endMonth); current = current.AddDate(0, 1, 0) {
		tableName := r.getTableName(current)
		base := r.db.WithContext(ctx).Table(tableName).
			Where("short_code = ? AND accessed_at BETWEEN ? AND ?", shortCode, startTime, endTime)

		var tableTotal int64
		if err := base.Count(&tableTotal).Error; err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, 0, err
		}
		total += tableTotal
		if tableTotal == 0 {
			continue
		}

		var rows []*model.AccessLog
		if err := base.Order("accessed_at DESC, id DESC").Limit(need).Find(&rows).Error; err != nil {
			if isTableNotExistError(err) {
				continue
			}
			return nil, 0, err
		}
		merged = append(merged, rows...)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].AccessedAt.Equal(merged[j].AccessedAt) {
			return merged[i].ID > merged[j].ID
		}
		return merged[i].AccessedAt.After(merged[j].AccessedAt)
	})

	if offset >= len(merged) {
		return []*model.AccessLog{}, total, nil
	}
	endIndex := offset + limit
	if endIndex > len(merged) {
		endIndex = len(merged)
	}

	return merged[offset:endIndex], total, nil
}
