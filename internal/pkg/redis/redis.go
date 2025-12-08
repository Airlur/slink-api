package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"short-link/internal/pkg/logger"
)

var (
	Client *redis.Client
)

// Set 设置键值对
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// 如果value是字符串，直接设置
	if str, ok := value.(string); ok {
		return Client.Set(ctx, key, str, expiration).Err()
	}

	// 其他类型转换为JSON
	bytes, err := json.Marshal(value)
	if err != nil {
		logger.Error("Redis marshal failed", "error", err, "key", key)
		return err
	}

	err = Client.Set(ctx, key, bytes, expiration).Err()
	if err != nil {
		logger.Error("Redis set failed", "error", err, "key", key)
		return err
	}

	return nil
}

// Get 获取值
func Get(ctx context.Context, key string) (string, error) {
	result, err := Client.Get(ctx, key).Result()
	if err != nil {
		// 当错误是 Nil 时，将 err 返回给调用者
		if err == redis.Nil { 
			return "", err 
		}
		logger.Error("Redis get failed", "error", err, "key", key)
		return "", err
	}
	return result, nil
}

// GetObj 获取对象
func GetObj(ctx context.Context, key string, obj interface{}) error {
	result, err := Client.Get(ctx, key).Result()
	if err != nil {
		// 当错误是 Nil 时，将 err 返回给调用者
		if err == redis.Nil {
			return err
		}
		logger.Error("Redis get failed", "error", err, "key", key)
		return err
	}

	err = json.Unmarshal([]byte(result), obj)
	if err != nil {
		logger.Error("Redis unmarshal failed", "error", err, "key", key)
		return err
	}

	return nil
}

// Del 删除键
func Del(ctx context.Context, key string) error {
	return Client.Del(ctx, key).Err()
}

// Exists 检查键是否存在
func Exists(ctx context.Context, key string) bool {
	result, err := Client.Exists(ctx, key).Result()
	if err != nil {
		logger.Error("Redis exists check failed", "error", err, "key", key)
		return false
	}
	return result > 0
}

// SetNX 如果键不存在则设置
func SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	// 如果value是字符串，直接设置
	if str, ok := value.(string); ok {
		return Client.SetNX(ctx, key, str, expiration).Result()
	}

	// 其他类型转换为JSON
	bytes, err := json.Marshal(value)
	if err != nil {
		logger.Error("Redis marshal failed", "error", err, "key", key)
		return false, err
	}

	return Client.SetNX(ctx, key, bytes, expiration).Result()
}

// IncrWithExpiration 对指定key执行自增操作，并为key设置过期时间（仅当key无过期时间时）
// 实现逻辑：
// 1. 使用Redis Pipeline批量执行INCR和EXPIRENX命令，减少网络往返
// 2. INCR：将key的值自增1，若key不存在则初始化为1
// 3. EXPIRENX：仅当key当前无过期时间时，设置过期时间为expiration（避免刷新已有过期时间）
// 适用场景：
// - 需按时间窗口（如每日、每小时）计数的限流场景（如短链接创建次数、验证码发送次数）
// - 需保证计数周期严格固定（如每日0点重置）的业务逻辑
// 返回值：自增后的计数；若操作失败，返回0和错误信息
func IncrWithExpiration(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	pipe := Client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, expiration) // 确保key无过期时间时设置
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("Redis Incr with expiration failed", "error", err, "key", key)
		return 0, err
	}
	return incrCmd.Result()
}

// GetAndDel 使用Lua脚本实现原子性的 "获取并删除" 操作
func GetAndDel(ctx context.Context, key string) (string, error) {
	// Lua脚本：
	// KEYS[1] - 要操作的键
	// 1. 获取键的值
	// 2. 如果值存在，则删除该键
	// 3. 返回获取到的值
	luaScript := `
		local val = redis.call('GET', KEYS[1])
		if val then
			redis.call('DEL', KEYS[1])
		end
		return val
	`
	// 执行脚本
	result, err := Client.Eval(ctx, luaScript, []string{key}).Result()
	if err != nil {
		// 如果key不存在，Eval会返回 redis.Nil，我们将其原样返回
		if err == redis.Nil {
			return "", err
		}
		logger.Error("执行GetAndDel Lua脚本失败", "error", err, "key", key)
		return "", err
	}

	// 转换结果
	if resultStr, ok := result.(string); ok {
		return resultStr, nil
	}

	return "", nil
}

// 【新增】HIncrBy 原子性地为一个哈希中的字段增加指定值
func HIncrBy(ctx context.Context, key, field string, value int64) error {
	return Client.HIncrBy(ctx, key, field, value).Err()
}

// 【新增】LPush 将一个或多个值插入到列表头部
func LPush(ctx context.Context, key string, values ...interface{}) error {
	return Client.LPush(ctx, key, values...).Err()
}

// 【新增，为下一步准备】 LTrim 对一个列表进行修剪，让其只包含指定范围的元素
func LTrim(ctx context.Context, key string, start, stop int64) error {
	return Client.LTrim(ctx, key, start, stop).Err()
}

// 【新增，为下一步准备】 LRange 返回列表中指定范围的元素
func LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return Client.LRange(ctx, key, start, stop).Result()
}

// 【新增，为下一步准备】 Rename 原子性地重命名key
func Rename(ctx context.Context, oldKey, newKey string) error {
	return Client.Rename(ctx, oldKey, newKey).Err()
}

// 【新增，为下一步准备】 HGetAll 获取哈希中所有字段和值
func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return Client.HGetAll(ctx, key).Result()
}

// Close 关闭Redis连接
func Close() error {
	return Client.Close()
}