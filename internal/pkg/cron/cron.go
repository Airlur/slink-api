package cron

import (
	"context"
	"time"

	"slink-api/internal/pkg/logger"
	"slink-api/internal/service"

	"github.com/robfig/cron/v3"
)

var c *cron.Cron

// InitCron 初始化并启动定时任务调度器
func InitCronOld(writerSvc service.BatchWriterService) {
	// 创建一个支持秒级任务的调度器
	c = cron.New(cron.WithSeconds())

	// 添加任务：每分钟执行一次数据同步
	_, err := c.AddFunc("@every 1m", func() {
		// 使用带超时的context执行任务
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := writerSvc.SyncRedisToDB(ctx); err != nil {
			logger.Error("执行[Redis -> DB]同步任务失败", "error", err)
		}
	})

	if err != nil {
		logger.Fatal("添加数据同步定时任务失败", "error", err)
	}

	// 启动调度器
	c.Start()
	logger.Info("定时任务调度器已启动")

	// 可以在这里注册一个应用关闭时的钩子，来优雅地停止调度器
	// e.g., <-appCloseSignal; c.Stop()
}

// InitCron 初始化并启动定时任务调度器 (已更新)
func InitCron(writerSvc service.BatchWriterService, maintenanceSvc service.MaintenanceService) *cron.Cron {
	// 创建一个支持秒级任务的调度器
	c := cron.New(cron.WithSeconds())

	// 任务1：每10s执行一次统计日志数据同步
	_, err := c.AddFunc("@every 10s", func() {
		// 使用带超时的context执行任务
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := writerSvc.SyncRedisToDB(ctx); err != nil {
			logger.Error("执行[Redis -> DB]同步任务失败", "error", err)
		}
	})

	if err != nil {
		logger.Fatal("添加数据同步定时任务失败", "error", err)
	}

	// 【新增】任务2：每天凌晨2点执行一次日志清理
	_, err = c.AddFunc("0 35 20 * * *", func() { // Cron表达式: "秒 分 时 日 月 周"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // 清理任务可能耗时较长
		defer cancel()
		if err := maintenanceSvc.CleanupOldLogs(ctx); err != nil {
			logger.Error("执行[清理过期日志]任务失败", "error", err)
		}
	})
	if err != nil {
		logger.Fatal("添加日志清理定时任务失败", "error", err)
	}

	// 启动调度器
	c.Start()
	logger.Info("定时任务调度器已启动")

	// 可以在这里注册一个应用关闭时的钩子，来优雅地停止调度器
	// e.g., <-appCloseSignal; c.Stop()

	return c
}