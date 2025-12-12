package eventbus

import (
	"short-link/internal/pkg/logger"
	"time"
)

// AccessLogEvent 定义了访问日志事件的数据结构
type AccessLogEvent struct {
	ShortCode string    // 访问的短码
	IP        string    // 访问者IP
	UserAgent string    // 访问者User-Agent
	Referer   string    // 来源地址
	UserID    uint      // 访问者用户ID（如果已登录，否则为0）
	Timestamp time.Time // 访问时间
}

// accessLogChannel 是一个带缓冲的channel，用于异步传递访问日志事件
var accessLogChannel chan AccessLogEvent

// InitEventBus 初始化事件总线
func InitEventBus() {
	// 创建一个足够大的缓冲channel，例如1024，以应对突发流量
	// 如果channel满了，新的日志事件将被丢弃，以保证主流程（短链接跳转）的性能
	// accessLogChannel = make(chan AccessLogEvent, 1024)
	accessLogChannel = make(chan AccessLogEvent, 2000)
}

// PublishAccessLog 发布一个访问日志事件（非阻塞）
func PublishAccessLog(event AccessLogEvent) {
	// 使用 select 和 default 来实现非阻塞发送
	// 如果channel已满，default分支会立即执行，防止阻塞API响应
	select {
	case accessLogChannel <- event:
		// 成功发送事件到channel
	default:
		// channel已满，丢弃事件并记录警告日志
		logger.Warn("访问日志事件channel已满，事件被丢弃", "shortCode", event.ShortCode)
	}
}

// SubscribeAccessLog 订阅访问日志事件
// 返回一个只读的channel，供后台Worker消费
func SubscribeAccessLog() <-chan AccessLogEvent {
	return accessLogChannel
}
