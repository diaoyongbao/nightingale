// Package ratelimit AI 助手速率限制
// n9e-2kai: AI 助手模块 - 速率限制器
package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	redis       redis.Cmdable
	localLimits map[string]*rate.Limiter
	config      *models.RateLimitConfig
	mu          sync.RWMutex
	appCtx      *ctx.Context
}

// RateLimitResult 速率限制结果
type RateLimitResult struct {
	Allowed    bool          `json:"allowed"`
	RetryAfter time.Duration `json:"retry_after"`
	Remaining  int           `json:"remaining"`
	Limit      int           `json:"limit"`
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(appCtx *ctx.Context, redisClient redis.Cmdable) (*RateLimiter, error) {
	config, err := models.GetRateLimitConfig(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to load rate limit config: %w", err)
	}

	return &RateLimiter{
		redis:       redisClient,
		localLimits: make(map[string]*rate.Limiter),
		config:      config,
		appCtx:      appCtx,
	}, nil
}


// Allow 检查是否允许请求
func (r *RateLimiter) Allow(ctx context.Context, userID string, busiGroupID int64) (*RateLimitResult, error) {
	limit := r.getLimit(userID, busiGroupID)
	key := r.buildKey(userID, busiGroupID)

	// 尝试使用 Redis
	if r.redis != nil {
		return r.redisAllow(ctx, key, limit)
	}

	// 降级到本地限流
	return r.localAllow(userID, limit)
}

// redisAllow 使用 Redis 实现分布式限流
func (r *RateLimiter) redisAllow(ctx context.Context, key string, limit int) (*RateLimitResult, error) {
	// 使用 Redis INCR + EXPIRE 实现滑动窗口
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		// Redis 不可用时降级到本地限流
		return r.localAllow(key, limit)
	}

	// 首次请求设置过期时间
	if count == 1 {
		r.redis.Expire(ctx, key, time.Minute)
	}

	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	if int(count) > limit {
		ttl, _ := r.redis.TTL(ctx, key).Result()
		if ttl < 0 {
			ttl = time.Minute
		}
		return &RateLimitResult{
			Allowed:    false,
			RetryAfter: ttl,
			Remaining:  0,
			Limit:      limit,
		}, nil
	}

	return &RateLimitResult{
		Allowed:   true,
		Remaining: remaining,
		Limit:     limit,
	}, nil
}

// localAllow 本地限流（降级方案）
func (r *RateLimiter) localAllow(key string, limit int) (*RateLimitResult, error) {
	r.mu.Lock()
	limiter, exists := r.localLimits[key]
	if !exists {
		// 创建新的本地限流器
		// rate.Limit 是每秒允许的请求数，这里转换为每分钟
		limiter = rate.NewLimiter(rate.Limit(float64(limit)/60.0), r.config.BurstSize)
		r.localLimits[key] = limiter
	}
	r.mu.Unlock()

	if limiter.Allow() {
		return &RateLimitResult{
			Allowed:   true,
			Remaining: int(limiter.Tokens()),
			Limit:     limit,
		}, nil
	}

	// 计算重试时间
	reservation := limiter.Reserve()
	delay := reservation.Delay()
	reservation.Cancel()

	return &RateLimitResult{
		Allowed:    false,
		RetryAfter: delay,
		Remaining:  0,
		Limit:      limit,
	}, nil
}

// buildKey 构建 Redis 键
func (r *RateLimiter) buildKey(userID string, busiGroupID int64) string {
	// 按分钟窗口
	minute := time.Now().Unix() / 60
	return fmt.Sprintf("ai:ratelimit:%s:%d:%d", userID, busiGroupID, minute)
}

// getLimit 获取用户的速率限制
func (r *RateLimiter) getLimit(userID string, busiGroupID int64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 优先检查用户级别限制
	if limit, ok := r.config.UserLimits[userID]; ok {
		return limit
	}

	// 其次检查业务组级别限制
	if limit, ok := r.config.BusiGroupLimits[busiGroupID]; ok {
		return limit
	}

	// 使用默认限制
	return r.config.DefaultRPM
}

// Reload 重新加载配置
func (r *RateLimiter) Reload() error {
	config, err := models.GetRateLimitConfig(r.appCtx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config

	// 清空本地限流器缓存，让它们使用新配置重建
	r.localLimits = make(map[string]*rate.Limiter)

	return nil
}

// GetConfig 获取当前配置
func (r *RateLimiter) GetConfig() *models.RateLimitConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// CleanupLocalLimits 清理过期的本地限流器（可定期调用）
func (r *RateLimiter) CleanupLocalLimits() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 简单策略：如果本地限流器数量超过 1000，清空重建
	if len(r.localLimits) > 1000 {
		r.localLimits = make(map[string]*rate.Limiter)
	}
}
