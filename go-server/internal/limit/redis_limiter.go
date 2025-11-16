// go-server/internal/limit/redis_limiter.go
package limit

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter 抽象：根据 fingerprint 判断是否允许访问
type RateLimiter interface {
	Allow(ctx context.Context, fingerprint string) (bool, error)
}

// RedisRateLimiter: 每个 fingerprint 在「当前时间到下一个周四凌晨」这段窗口里有 N 次配额
type RedisRateLimiter struct {
	Client    *redis.Client
	Limit     int
	KeyPrefix string
}

func NewRedisRateLimiter(client *redis.Client, limit int, prefix string) *RedisRateLimiter {
	return &RedisRateLimiter{
		Client:    client,
		Limit:     limit,
		KeyPrefix: prefix,
	}
}

// nextThursdayMidnight 计算下一个周四 00:00
func nextThursdayMidnight() time.Time {
	now := time.Now()
	weekday := now.Weekday() // Sunday=0 ... Thursday=4
	daysUntilThursday := (4 - int(weekday) + 7) % 7
	if daysUntilThursday == 0 {
		daysUntilThursday = 7
	}
	thursday := now.AddDate(0, 0, daysUntilThursday)
	return time.Date(thursday.Year(), thursday.Month(), thursday.Day(), 0, 0, 0, 0, thursday.Location())
}

func (r *RedisRateLimiter) Allow(ctx context.Context, fingerprint string) (bool, error) {
	key := r.KeyPrefix + fingerprint

	// 不存在则初始化
	exists, err := r.Client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if exists == 0 {
		expireAt := nextThursdayMidnight()
		duration := time.Until(expireAt)
		if err := r.Client.Set(ctx, key, r.Limit, duration).Err(); err != nil {
			return false, err
		}
		log.Printf("新设备指纹 %s 已设置访问限制为 %d 次，截止到 %s", fingerprint, r.Limit, expireAt.Format(time.RFC3339))
	}

	// 查看剩余配额
	val, err := r.Client.Get(ctx, key).Int()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if val <= 0 {
		return false, nil
	}

	// pipeline: DECR + 保持 TTL
	pipe := r.Client.TxPipeline()
	pipe.Decr(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	ttl := ttlCmd.Val()
	if ttl > 0 {
		r.Client.Expire(ctx, key, ttl)
	}
	return true, nil
}
