package main

import (
	"short-link/internal/bootstrap"
	"short-link/internal/pkg/logger"
)

func main() {
	// 创建并初始化 server
	server := bootstrap.New()
	if err := server.Initialize(); err != nil {
		// 如果初始化失败，此时日志系统可能已经启动
		// 使用我们自己的 logger 记录致命错误，以保持日志格式统一
		logger.Fatal("Failed to initialize server: %v", err)
	}

	// 延迟同步日志，确保所有缓冲的日志都被写入
	defer logger.Log.Sync()

	// 运行 server
	if err := server.Run(); err != nil {
		logger.Fatal("Failed to run server: %v", err)
	}
}
