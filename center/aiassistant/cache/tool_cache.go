// Package cache AI 助手工具缓存
// n9e-2kai: AI 助手模块 - 工具结果缓存
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/redis/go-redis/v9"
)

// ToolCache 工具缓存
type ToolCache struct {
	redis  redis.Cmdable
	config *models.CacheConfig
	mu     sync.RWMutex
	appCtx *ctx.Context
}

// CachedResult 缓存结果
type CachedResult struct {
	Result    interface{} `json:"result"`
	CachedAt  int64       `json:"cached_at"`
	ExpiresAt int64       `json:"expires_at"`
	ToolName  string      `json:"tool_name"`
}

// NewToolCache 创建工具缓存
func NewToolCache(appCtx *ctx.Context, redisClient redis.Cmdable) (*ToolCache, error) {
	config, err := models.GetCacheConfig(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to load cache config: %w", err)
	}

	return &ToolCache{
		redis:  redisClient,
		config: config,
		appCtx: appCtx,
	}, nil
}


// Get 获取缓存结果
func (c *ToolCache) Get(ctx context.Context, toolName string, args map[string]interface{}) (*CachedResult, error) {
	if !c.IsEnabled() {
		return nil, nil
	}

	if !c.IsIdempotent(toolName) {
		return nil, nil // 非幂等工具不缓存
	}

	if c.redis == nil {
		return nil, nil
	}

	key := c.BuildKey(toolName, args)
	data, err := c.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result CachedResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// 检查是否过期
	if result.ExpiresAt < time.Now().Unix() {
		return nil, nil
	}

	return &result, nil
}

// Set 设置缓存
func (c *ToolCache) Set(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
	if !c.IsEnabled() {
		return nil
	}

	if !c.IsIdempotent(toolName) {
		return nil
	}

	if c.redis == nil {
		return nil
	}

	key := c.BuildKey(toolName, args)
	ttl := c.GetTTL(toolName)

	cached := CachedResult{
		Result:    result,
		CachedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(ttl).Unix(),
		ToolName:  toolName,
	}

	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}

	return c.redis.Set(ctx, key, data, ttl).Err()
}

// Delete 删除缓存
func (c *ToolCache) Delete(ctx context.Context, toolName string, args map[string]interface{}) error {
	if c.redis == nil {
		return nil
	}

	key := c.BuildKey(toolName, args)
	return c.redis.Del(ctx, key).Err()
}

// BuildKey 构建缓存键
func (c *ToolCache) BuildKey(toolName string, args map[string]interface{}) string {
	argsJSON, _ := json.Marshal(args)
	hash := sha256.Sum256(argsJSON)
	return fmt.Sprintf("ai:cache:%s:%x", toolName, hash[:8])
}

// IsIdempotent 检查工具是否幂等
func (c *ToolCache) IsIdempotent(toolName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, t := range c.config.IdempotentTools {
		if t == toolName {
			return true
		}
	}
	return false
}

// GetTTL 获取工具的 TTL
func (c *ToolCache) GetTTL(toolName string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 优先使用工具特定的 TTL
	if ttl, ok := c.config.ToolTTLs[toolName]; ok {
		return time.Duration(ttl) * time.Second
	}

	// 使用默认 TTL
	return time.Duration(c.config.DefaultTTL) * time.Second
}

// IsEnabled 检查缓存是否启用
func (c *ToolCache) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.Enabled
}

// Reload 重新加载配置
func (c *ToolCache) Reload() error {
	config, err := models.GetCacheConfig(c.appCtx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config

	return nil
}

// GetConfig 获取当前配置
func (c *ToolCache) GetConfig() *models.CacheConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Clear 清空所有缓存（谨慎使用）
func (c *ToolCache) Clear(ctx context.Context) error {
	if c.redis == nil {
		return nil
	}

	// 使用 SCAN 删除所有 ai:cache:* 键
	var cursor uint64
	for {
		keys, nextCursor, err := c.redis.Scan(ctx, cursor, "ai:cache:*", 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := c.redis.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}
