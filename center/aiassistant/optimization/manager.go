// Package optimization AI 助手优化管理器
// n9e-2kai: AI 助手模块 - 优化组件管理器
package optimization

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant/cache"
	"github.com/ccfos/nightingale/v6/center/aiassistant/cost"
	"github.com/ccfos/nightingale/v6/center/aiassistant/executor"
	"github.com/ccfos/nightingale/v6/center/aiassistant/observability"
	"github.com/ccfos/nightingale/v6/center/aiassistant/ratelimit"
	"github.com/ccfos/nightingale/v6/center/aiassistant/retry"
	"github.com/ccfos/nightingale/v6/center/aiassistant/router"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/redis/go-redis/v9"
)

// Manager 优化管理器
type Manager struct {
	RateLimiter *ratelimit.RateLimiter
	ToolCache   *cache.ToolCache
	ModelRouter *router.ModelRouter
	Executor    *executor.ConcurrentExecutor
	Retry       *retry.RetryHandler
	CostTracker *cost.CostTracker

	appCtx *ctx.Context
	mu     sync.RWMutex
}

// NewManager 创建优化管理器
func NewManager(appCtx *ctx.Context, redisClient redis.Cmdable) (*Manager, error) {
	mgr := &Manager{
		appCtx: appCtx,
	}

	var err error

	// 初始化速率限制器
	mgr.RateLimiter, err = ratelimit.NewRateLimiter(appCtx, redisClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}

	// 初始化工具缓存
	mgr.ToolCache, err = cache.NewToolCache(appCtx, redisClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool cache: %w", err)
	}

	// 初始化模型路由器
	mgr.ModelRouter, err = router.NewModelRouter(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model router: %w", err)
	}

	// 初始化并发执行器
	mgr.Executor, err = executor.NewConcurrentExecutor(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create concurrent executor: %w", err)
	}

	// 初始化重试处理器
	mgr.Retry, err = retry.NewRetryHandler(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry handler: %w", err)
	}

	// 初始化成本追踪器
	mgr.CostTracker, err = cost.NewCostTracker(appCtx, redisClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost tracker: %w", err)
	}

	return mgr, nil
}


// CheckRateLimit 检查速率限制
func (m *Manager) CheckRateLimit(ctx context.Context, userID string, busiGroupID int64) (*ratelimit.RateLimitResult, error) {
	if m.RateLimiter == nil {
		return &ratelimit.RateLimitResult{Allowed: true}, nil
	}

	result, err := m.RateLimiter.Allow(ctx, userID, busiGroupID)
	if err != nil {
		return nil, err
	}

	// 记录指标
	if !result.Allowed {
		observability.RecordRateLimitHit(userID, busiGroupID)
	}

	return result, nil
}

// GetCachedResult 获取缓存结果
func (m *Manager) GetCachedResult(ctx context.Context, toolName string, args map[string]interface{}) (*cache.CachedResult, bool) {
	if m.ToolCache == nil {
		return nil, false
	}

	result, err := m.ToolCache.Get(ctx, toolName, args)
	if err != nil {
		return nil, false
	}

	hit := result != nil
	observability.RecordCacheHit(toolName, hit)

	return result, hit
}

// SetCachedResult 设置缓存结果
func (m *Manager) SetCachedResult(ctx context.Context, toolName string, args map[string]interface{}, result interface{}) error {
	if m.ToolCache == nil {
		return nil
	}

	return m.ToolCache.Set(ctx, toolName, args, result)
}

// GetModel 获取任务类型对应的模型
func (m *Manager) GetModel(taskType router.TaskType) string {
	if m.ModelRouter == nil {
		return ""
	}
	return m.ModelRouter.GetModel(taskType)
}

// GetModelConfig 获取任务类型的完整模型配置
func (m *Manager) GetModelConfig(taskType router.TaskType) *ModelConfig {
	if m.ModelRouter == nil {
		return nil
	}

	config := m.ModelRouter.GetModelConfig(taskType)
	if config == nil {
		return nil
	}

	return &ModelConfig{
		Model:       config.Model,
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
	}
}

// ModelConfig 模型配置
type ModelConfig struct {
	Model       string
	MaxTokens   int
	Temperature float64
}

// ExecuteWithRetry 带重试执行
func (m *Manager) ExecuteWithRetry(ctx context.Context, fn func() (interface{}, error)) *retry.RetryResult {
	if m.Retry == nil {
		result, err := fn()
		return &retry.RetryResult{
			Result:   result,
			Error:    err,
			Attempts: 1,
		}
	}

	return m.Retry.Execute(ctx, fn)
}

// ExecuteToolsConcurrently 并发执行工具
func (m *Manager) ExecuteToolsConcurrently(ctx context.Context, toolCalls []executor.ToolCall, executorFn executor.ToolExecutor) []executor.ToolCallResult {
	if m.Executor == nil {
		// 顺序执行
		results := make([]executor.ToolCallResult, len(toolCalls))
		for i, tc := range toolCalls {
			start := time.Now()
			result, err := executorFn(ctx, tc)
			results[i] = executor.ToolCallResult{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Result:     result,
				Error:      err,
				Duration:   time.Since(start).Milliseconds(),
				Success:    err == nil,
			}
		}
		return results
	}

	return m.Executor.ExecuteAll(ctx, toolCalls, executorFn)
}

// RecordUsage 记录使用量
func (m *Manager) RecordUsage(ctx context.Context, userID, sessionID, model string, promptTokens, completionTokens int) error {
	if m.CostTracker == nil {
		return nil
	}

	return m.CostTracker.RecordUsage(ctx, &cost.UsageRecord{
		UserID:           userID,
		SessionID:        sessionID,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	})
}

// Reload 重新加载所有配置
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if m.RateLimiter != nil {
		if err := m.RateLimiter.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("rate limiter reload: %w", err))
		}
	}

	if m.ToolCache != nil {
		if err := m.ToolCache.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("tool cache reload: %w", err))
		}
	}

	if m.ModelRouter != nil {
		if err := m.ModelRouter.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("model router reload: %w", err))
		}
	}

	if m.Executor != nil {
		if err := m.Executor.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("executor reload: %w", err))
		}
	}

	if m.Retry != nil {
		if err := m.Retry.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("retry handler reload: %w", err))
		}
	}

	if m.CostTracker != nil {
		if err := m.CostTracker.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("cost tracker reload: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("reload errors: %v", errs)
	}

	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) *Stats {
	stats := &Stats{}

	if m.RateLimiter != nil {
		stats.RateLimitConfig = m.RateLimiter.GetConfig()
	}

	if m.ToolCache != nil {
		stats.CacheConfig = m.ToolCache.GetConfig()
	}

	if m.ModelRouter != nil {
		stats.ModelRouterConfig = m.ModelRouter.GetConfig()
	}

	if m.Executor != nil {
		stats.ConcurrentConfig = m.Executor.GetConfig()
	}

	if m.Retry != nil {
		stats.RetryConfig = m.Retry.GetConfig()
	}

	if m.CostTracker != nil {
		stats.CostConfig = m.CostTracker.GetConfig()
		// 获取今日成本
		date := time.Now().Format("2006-01-02")
		if dailyCost, err := m.CostTracker.GetDailyCost(ctx, date); err == nil {
			stats.TodayCost = dailyCost
		}
	}

	return stats
}

// Stats 统计信息
type Stats struct {
	RateLimitConfig   interface{} `json:"rate_limit_config"`
	CacheConfig       interface{} `json:"cache_config"`
	ModelRouterConfig interface{} `json:"model_router_config"`
	ConcurrentConfig  interface{} `json:"concurrent_config"`
	RetryConfig       interface{} `json:"retry_config"`
	CostConfig        interface{} `json:"cost_config"`
	TodayCost         interface{} `json:"today_cost"`
}
