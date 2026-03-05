package repository

import (
	"context"
	"errors"
	"fmt"
	"time"
	"sync"

	"slink-api/internal/model"
	"slink-api/internal/pkg/logger"
	
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type LogRepository interface {
	Create(ctx context.Context, log *model.AccessLog) error
	CreateInBatches(ctx context.Context, logs []*model.AccessLog) error
	GetRecentLogs(ctx context.Context, shortCode string, limit int) ([]*model.AccessLog, error)
	// 分页获取原始日志
	ListLogs(ctx context.Context, shortCode string, page, limit int) ([]*model.AccessLog, int64, error)
}

type logRepository struct {
	db *gorm.DB
	// 使用 sync.Map 来线程安全地缓存已创建的表名
	createdTables sync.Map
}

func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

// getTableName 根据时间动态生成表名，例如 "access_logs_202510"
func (r *logRepository) getTableName(t time.Time) string {
	return fmt.Sprintf("access_logs_%s", t.Format("200601"))
}

// ensureTableExists 确保目标表存在，如果不存在则自动创建，并设置动态注释
func (r *logRepository) ensureTableExists(ctx context.Context, tableName string, tableTime time.Time) error {
	// 1先从内存缓存中检查，如果已确认过存在，则直接返回，性能最高
	if _, ok := r.createdTables.Load(tableName); ok {
		return nil
	}

	// 如果内存中没有，则查询数据库
	// 注意：HasTable 的性能不如直接执行 CREATE IF NOT EXISTS，但更符合GORM的用法
	// 在极高并发下，这里可能需要加分布式锁，但对于每月一次的创建操作，目前已足够
	if !r.db.WithContext(ctx).Migrator().HasTable(tableName) {
		// 如果表不存在，则使用模板表来创建新表
		// 【前提】需要在数据库中手动创建一个名为 `access_logs_template` 的表
		createSQL := fmt.Sprintf("CREATE TABLE `%s` LIKE `access_logs_template`;", tableName)
		if err := r.db.WithContext(ctx).Exec(createSQL).Error; err != nil {
			return err
		}
 
		//【核心新增点】修改新表的注释
		comment := fmt.Sprintf("访问日志表 (%s)", tableTime.Format("2006年01月"))
		alterSQL := fmt.Sprintf("ALTER TABLE `%s` COMMENT='%s';", tableName, comment)
		if err := r.db.WithContext(ctx).Exec(alterSQL).Error; err != nil {
			// 即使修改注释失败，也不应阻塞主流程，但需要记录日志
			logger.Warn("修改日志分表注释失败", "error", err, "tableName", tableName)
		}
	}
	
	// 将已确认存在的表名存入内存缓存
	r.createdTables.Store(tableName, true)
	return nil
}

// isTableNotExistError 是一个辅助函数，用于检查GORM错误是否为“表不存在”
func isTableNotExistError(err error) bool {
	var mysqlErr *mysql.MySQLError
	// errors.As 可以深入检查被包装的错误
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
		return true
	}
	return false
}

// ====================== DB 操作方法 ======================
// Create 实现了向动态分表中插入日志的逻辑
func (r *logRepository) Create(ctx context.Context, log *model.AccessLog) error {
	tableName := r.getTableName(log.AccessedAt)

	// 在插入前，确保当月的表已经存在，并传入时间用于生成注释
	if err := r.ensureTableExists(ctx, tableName, log.AccessedAt); err != nil {
		return err
	}

	err := r.db.WithContext(ctx).Table(tableName).Create(log).Error
	
	// 【核心修正点】如果插入失败，并且错误是“表不存在”
	if err != nil && isTableNotExistError(err) {
		// 这说明内存缓存是脏的，与数据库状态不一致
		// 1. 从内存缓存中删除脏数据
		r.createdTables.Delete(tableName)
		logger.Warn("检测到日志表状态不一致，已清理内存缓存", "tableName", tableName)

		// 2. 再次调用 Create，这次 ensureTableExists 会重新建表
		// 为了防止无限递归，这里不再检查错误，如果再次失败则直接向上抛出
		return r.Create(ctx, log)
	}
	
	return err
}

// CreateInBatches 实现了日志的批量插入
func (r *logRepository) CreateInBatches0(ctx context.Context, logs []*model.AccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	
	// 按月份对日志进行分组
	logsByTable := make(map[string][]*model.AccessLog)
	for _, log := range logs {
		tableName := r.getTableName(log.AccessedAt)
		logsByTable[tableName] = append(logsByTable[tableName], log)
	}

	// 对每个月份的表执行一次批量插入
	for tableName, batch := range logsByTable {
		if err := r.ensureTableExists(ctx, tableName, batch[0].AccessedAt); err != nil {
			return err
		}
		// GORM 的 Create 方法本身就支持传入切片进行批量插入
		if err := r.db.WithContext(ctx).Table(tableName).Create(batch).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *logRepository) CreateInBatchesOld(ctx context.Context, logs []*model.AccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	
	// 按月份对日志进行分组
	logsByTable := make(map[string][]*model.AccessLog)
	for _, log := range logs {
		tableName := r.getTableName(log.AccessedAt)
		logsByTable[tableName] = append(logsByTable[tableName], log)
	}

	// 对每个月的表执行一次批量插入
	for tableName, batch := range logsByTable {
		err := r.db.WithContext(ctx).Table(tableName).Create(batch).Error
		
		// 【核心修正点】与Create方法一样的缓存失效和重试逻辑
		if err != nil && isTableNotExistError(err) {
			r.createdTables.Delete(tableName)
			logger.Warn("检测到日志表状态不一致（批量），已清理内存缓存", "tableName", tableName)

			// 只需要重试建表，然后再次插入即可
			if ensureErr := r.ensureTableExists(ctx, tableName, batch[0].AccessedAt); ensureErr != nil {
				return ensureErr
			}
			return r.db.WithContext(ctx).Table(tableName).Create(batch).Error
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *logRepository) CreateInBatches(ctx context.Context, logs []*model.AccessLog) error {
	if len(logs) == 0 {
		return nil
	}

	// 按月份分组（原有逻辑保留）
	logsByTable := make(map[string][]*model.AccessLog)
	for _, log := range logs {
		tableName := r.getTableName(log.AccessedAt)
		logsByTable[tableName] = append(logsByTable[tableName], log)
	}

	// 【核心修改1】设置单批最大条数（500条/批，可根据字段数调整）
	batchSize := 500

	// 对每个月的表分批插入
	for tableName, batchLogs := range logsByTable {
		total := len(batchLogs)
		// 循环拆分批次
		for i := 0; i < total; i += batchSize {
			// 计算当前批次的结束索引
			end := i + batchSize
			if end > total {
				end = total
			}
			// 截取当前批次的日志
			currentBatch := batchLogs[i:end]

			// 执行单批插入
			err := r.db.WithContext(ctx).Table(tableName).Create(currentBatch).Error
			if err != nil {
				// 原有表不存在的重试逻辑（保留）
				if isTableNotExistError(err) {
					r.createdTables.Delete(tableName)
					logger.Warn("检测到日志表状态不一致（批量），已清理内存缓存", "tableName", tableName)
					// 重试建表
					if ensureErr := r.ensureTableExists(ctx, tableName, currentBatch[0].AccessedAt); ensureErr != nil {
						return ensureErr
					}
					// 重新插入当前批次
					if err = r.db.WithContext(ctx).Table(tableName).Create(currentBatch).Error; err != nil {
						return err
					}
					continue
				}
				// 其他错误直接返回
				return err
			}
		}
		logger.Info("批量写入日志成功", "tableName", tableName, "total条数", total, "批次数量", (total+batchSize-1)/batchSize)
	}
	return nil
}

// GetRecentLogs 查询最近日志
func (r *logRepository) GetRecentLogs(ctx context.Context, shortCode string, limit int) ([]*model.AccessLog, error) {
	var logs []*model.AccessLog
	// 我们只查询当月的表，这对于“最近访问”是合理的简化
	tableName := r.getTableName(time.Now())

	err := r.db.WithContext(ctx).Table(tableName).
		Where("short_code = ?", shortCode).
		Order("accessed_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// ListLogs 分页获取原始访问日志
func (r *logRepository) ListLogs(ctx context.Context, shortCode string, page, limit int) ([]*model.AccessLog, int64, error) {
	var logs []*model.AccessLog
	var total int64
	
	// 同样，我们只查询当月的表作为简化实现
	tableName := r.getTableName(time.Now())

	db := r.db.WithContext(ctx).Table(tableName).Where("short_code = ?", shortCode)

	// 先执行Count
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// 再执行分页查询
	offset := (page - 1) * limit
	err := db.Order("accessed_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&logs).Error
		
	return logs, total, err
}