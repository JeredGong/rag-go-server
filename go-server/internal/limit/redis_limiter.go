// go-server/internal/limit/redis_limiter.go
//
// 限流器模块
//
// 本模块基于 Redis 实现分布式限流功能，防止单个设备过度调用 API。
// 采用「周期窗口 + 计数器」的策略，为每个设备指纹分配固定周期内的访问配额。
//
// 限流策略特点：
// 1. 以设备指纹作为唯一标识，支持跨多个服务实例的统一限流
// 2. 使用「到下一个周四凌晨」作为窗口边界，配额自动重置
// 3. 计数器递减，剩余配额为 0 时拒绝访问
// 4. 支持灵活配置每个指纹的访问上限
//
// 适用场景：
// - 免费用户访问配额控制
// - 防止单设备恶意刷接口
// - 公平分配有限的 API 调用额度
package limit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter 定义限流器的通用接口
//
// 通过接口抽象，可以方便地切换不同的限流实现（如内存限流、数据库限流等），
// 也便于在单元测试中使用 mock 对象。
type RateLimiter interface {
	// Allow 判断指定设备指纹是否有剩余配额
	//
	// 参数：
	//   - ctx: 上下文对象，用于控制超时和取消
	//   - fingerprint: 设备指纹字符串，作为限流的唯一标识
	//
	// 返回值：
	//   - bool: true 表示允许访问，false 表示配额已用尽
	//   - error: 如果 Redis 操作失败，返回错误信息
	Allow(ctx context.Context, fingerprint string) (bool, error)
}

// RedisRateLimiter 基于 Redis 的限流器实现
//
// 工作原理：
// 1. 首次访问时，在 Redis 中创建一个计数器，初始值为配额上限
// 2. 设置过期时间为「下一个周四凌晨」，到期自动删除，实现配额重置
// 3. 每次访问时递减计数器，计数器为 0 时拒绝访问
// 4. 使用 Redis 的原子操作（DECR）保证并发安全
type RedisRateLimiter struct {
	// Client Redis 客户端，用于执行读写操作
	Client *redis.Client

	// Limit 每个设备指纹在一个周期内的访问配额上限
	Limit int

	// KeyPrefix Redis 键的前缀，用于区分不同业务的限流数据
	// 最终存储的键格式为：prefix + fingerprint
	KeyPrefix string
}

var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

local current = redis.call("GET", key)
if not current then
	redis.call("SET", key, limit - 1, "PX", ttl)
	return limit - 1
end

current = tonumber(current)
if current <= 0 then
	return -1
end

return redis.call("DECR", key)
`)

// NewRedisRateLimiter 创建一个新的 Redis 限流器实例
//
// 参数：
//   - client: Redis 客户端实例
//   - limit: 每个设备的访问配额上限
//   - prefix: Redis 键的前缀字符串
//
// 返回值：
//   - *RedisRateLimiter: 限流器实例
func NewRedisRateLimiter(client *redis.Client, limit int, prefix string) *RedisRateLimiter {
	return &RedisRateLimiter{
		Client:    client,
		Limit:     limit,
		KeyPrefix: prefix,
	}
}

// nextThursdayMidnight 计算下一个周四 00:00 的时间点
//
// 这个函数用于确定配额的过期时间。选择周四作为重置点可以：
// 1. 为用户提供稳定的每周访问周期
// 2. 避免在周末或周一（访问高峰期）重置配额
//
// 计算逻辑：
// - 如果今天是周四之前，返回本周四 00:00
// - 如果今天是周四或之后，返回下周四 00:00
//
// 返回值：
//   - time.Time: 下一个周四午夜的时间对象
func nextThursdayMidnight() time.Time {
	now := time.Now()
	weekday := now.Weekday() // Sunday=0, Monday=1, ..., Thursday=4, ..., Saturday=6
	
	// 计算距离下一个周四还有多少天
	// 使用模运算确保结果在 0-6 范围内
	daysUntilThursday := (4 - int(weekday) + 7) % 7
	
	// 如果今天就是周四，则跳到下周四
	if daysUntilThursday == 0 {
		daysUntilThursday = 7
	}
	
	// 在当前日期基础上加上天数，得到下一个周四
	thursday := now.AddDate(0, 0, daysUntilThursday)
	
	// 将时间归零到午夜 00:00:00
	return time.Date(thursday.Year(), thursday.Month(), thursday.Day(), 0, 0, 0, 0, thursday.Location())
}

// Allow 判断指定设备是否允许访问
//
// 实现流程：
// 1. 检查 Redis 中是否存在该设备的记录
// 2. 如果不存在，初始化配额并设置过期时间
// 3. 获取当前剩余配额
// 4. 如果配额充足，递减配额并允许访问
// 5. 如果配额不足，拒绝访问
//
// 参数：
//   - ctx: 上下文对象
//   - fingerprint: 设备指纹
//
// 返回值：
//   - bool: 是否允许访问
//   - error: 操作错误
func (r *RedisRateLimiter) Allow(ctx context.Context, fingerprint string) (bool, error) {
	// 构造 Redis 键：prefix + 设备指纹
	key := r.KeyPrefix + fingerprint

	expireAt := nextThursdayMidnight()
	duration := time.Until(expireAt)
	if duration <= 0 {
		duration = 7 * 24 * time.Hour
	}

	res, err := rateLimitScript.Run(ctx, r.Client, []string{key}, r.Limit, duration.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("执行限流脚本失败: %w", err)
	}

	if res < 0 {
		return false, nil
	}

	if res == int64(r.Limit-1) {
		log.Printf("新设备指纹 %s 已设置访问限制为 %d 次，截止到 %s",
			fingerprint, r.Limit, expireAt.Format(time.RFC3339))
	} else if res < 3 {
		log.Printf("[限流提醒] 设备 %s 剩余配额 %d 次", fingerprint, res)
	}

	return true, nil
}
