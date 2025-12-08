package constant

const (
	// 短链接相关
	ShortCodeLength = 6 // 短码长度

	// 缓存相关
	NullCacheValue       = "NULL_CACHE" // 缓存不存在的KEY，防止缓存穿透
	CacheShortLinkPrefix = "short:"     // 短链接缓存前缀
	CacheTTL             = 86400        // 缓存过期时间（秒）

	// 日志相关
	LogBufferKey = "logs:buffer:raw" // 原始日志缓冲区
	LogBatchSize = 100               // 批量处理大小

	// 统计相关
	StatsTotalPrefix  = "stats:total:"  // 总点击数前缀
	StatsDailyPrefix  = "stats:daily:"  // 每日统计前缀
	StatsRegionPrefix = "stats:region:" // 地域统计前缀
	StatsDevicePrefix = "stats:device:" // 设备统计前缀

	// [重构新增] 统计Key追踪相关
	StatsActiveKeysSet = "stats:active_keys" // 活跃统计Key集合，用于替代KEYS扫描
	StatsRetryQueue    = "stats:retry_queue" // 统计同步失败重试队列
	StatsRetryMaxCount = 3                   // 最大重试次数

	// [重构新增] 批量处理相关
	StatsSyncBatchSize = 500 // 每次同步处理的Key数量上限
)
