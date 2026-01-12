// Package cost AI 助手成本追踪
// n9e-2kai: AI 助手模块 - 成本追踪器
package cost

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant/observability"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/redis/go-redis/v9"
)

// UsageRecord 使用记录
type UsageRecord struct {
	Timestamp        int64   `json:"timestamp"`
	UserID           string  `json:"user_id"`
	SessionID        string  `json:"session_id"`
	Model            string  `json:"model"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

// DailyCost 每日成本
type DailyCost struct {
	Date       string             `json:"date"`
	Total      float64            `json:"total"`
	ByModel    map[string]float64 `json:"by_model"`
	ByUser     map[string]float64 `json:"by_user"`
	TotalCalls int64              `json:"total_calls"`
}

// CostTracker 成本追踪器
type CostTracker struct {
	redis  redis.Cmdable
	config *models.CostConfig
	mu     sync.RWMutex
	appCtx *ctx.Context
}

// NewCostTracker 创建成本追踪器
func NewCostTracker(appCtx *ctx.Context, redisClient redis.Cmdable) (*CostTracker, error) {
	config, err := models.GetCostConfig(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to load cost config: %w", err)
	}

	return &CostTracker{
		redis:  redisClient,
		config: config,
		appCtx: appCtx,
	}, nil
}


// RecordUsage 记录使用量
func (t *CostTracker) RecordUsage(ctx context.Context, record *UsageRecord) error {
	// 计算成本
	record.TotalTokens = record.PromptTokens + record.CompletionTokens
	record.Cost = t.CalculateCost(record.Model, record.PromptTokens, record.CompletionTokens)
	record.Timestamp = time.Now().Unix()

	// 更新 Prometheus 指标
	observability.RecordTokens(record.Model, record.PromptTokens, record.CompletionTokens)
	observability.RecordCost(record.Model, record.Cost)

	if t.redis == nil {
		return nil
	}

	// 存储到 Redis（按天聚合）
	date := time.Now().Format("2006-01-02")
	dayKey := fmt.Sprintf("ai:cost:%s", date)
	userKey := fmt.Sprintf("ai:cost:user:%s:%s", record.UserID, date)
	modelKey := fmt.Sprintf("ai:cost:model:%s:%s", record.Model, date)

	pipe := t.redis.Pipeline()

	// 总成本
	pipe.HIncrByFloat(ctx, dayKey, "total", record.Cost)
	pipe.HIncrBy(ctx, dayKey, "calls", 1)
	pipe.HIncrBy(ctx, dayKey, "prompt_tokens", int64(record.PromptTokens))
	pipe.HIncrBy(ctx, dayKey, "completion_tokens", int64(record.CompletionTokens))

	// 按模型统计
	pipe.HIncrByFloat(ctx, dayKey, fmt.Sprintf("model:%s", record.Model), record.Cost)

	// 用户成本
	pipe.HIncrByFloat(ctx, userKey, "total", record.Cost)
	pipe.HIncrBy(ctx, userKey, "calls", 1)

	// 模型成本
	pipe.HIncrByFloat(ctx, modelKey, "total", record.Cost)
	pipe.HIncrBy(ctx, modelKey, "calls", 1)

	// 设置过期时间（30 天）
	expiration := 30 * 24 * time.Hour
	pipe.Expire(ctx, dayKey, expiration)
	pipe.Expire(ctx, userKey, expiration)
	pipe.Expire(ctx, modelKey, expiration)

	_, err := pipe.Exec(ctx)
	return err
}

// CalculateCost 计算成本
func (t *CostTracker) CalculateCost(model string, promptTokens, completionTokens int) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	price, ok := t.config.ModelPrices[model]
	if !ok {
		return 0
	}

	promptCost := float64(promptTokens) / 1000.0 * price.PromptPricePer1K
	completionCost := float64(completionTokens) / 1000.0 * price.CompletionPricePer1K

	return promptCost + completionCost
}

// GetDailyCost 获取每日成本
func (t *CostTracker) GetDailyCost(ctx context.Context, date string) (*DailyCost, error) {
	if t.redis == nil {
		return &DailyCost{Date: date}, nil
	}

	key := fmt.Sprintf("ai:cost:%s", date)
	data, err := t.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	result := &DailyCost{
		Date:    date,
		ByModel: make(map[string]float64),
		ByUser:  make(map[string]float64),
	}

	for k, v := range data {
		switch k {
		case "total":
			fmt.Sscanf(v, "%f", &result.Total)
		case "calls":
			fmt.Sscanf(v, "%d", &result.TotalCalls)
		default:
			if len(k) > 6 && k[:6] == "model:" {
				var cost float64
				fmt.Sscanf(v, "%f", &cost)
				result.ByModel[k[6:]] = cost
			}
		}
	}

	return result, nil
}

// GetUserCost 获取用户成本
func (t *CostTracker) GetUserCost(ctx context.Context, userID, date string) (float64, int64, error) {
	if t.redis == nil {
		return 0, 0, nil
	}

	key := fmt.Sprintf("ai:cost:user:%s:%s", userID, date)
	data, err := t.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, 0, err
	}

	var total float64
	var calls int64

	if v, ok := data["total"]; ok {
		fmt.Sscanf(v, "%f", &total)
	}
	if v, ok := data["calls"]; ok {
		fmt.Sscanf(v, "%d", &calls)
	}

	return total, calls, nil
}

// GetUserCostRange 获取用户一段时间的成本
func (t *CostTracker) GetUserCostRange(ctx context.Context, userID string, startDate, endDate time.Time) (float64, int64, error) {
	var totalCost float64
	var totalCalls int64

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		cost, calls, err := t.GetUserCost(ctx, userID, date)
		if err != nil {
			continue
		}
		totalCost += cost
		totalCalls += calls
	}

	return totalCost, totalCalls, nil
}

// Reload 重新加载配置
func (t *CostTracker) Reload() error {
	config, err := models.GetCostConfig(t.appCtx)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.config = config

	return nil
}

// GetConfig 获取当前配置
func (t *CostTracker) GetConfig() *models.CostConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.config
}

// GetAlertThreshold 获取告警阈值
func (t *CostTracker) GetAlertThreshold() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.config.AlertThreshold
}

// CheckThreshold 检查是否超过阈值
func (t *CostTracker) CheckThreshold(ctx context.Context) (bool, float64, error) {
	date := time.Now().Format("2006-01-02")
	dailyCost, err := t.GetDailyCost(ctx, date)
	if err != nil {
		return false, 0, err
	}

	threshold := t.GetAlertThreshold()
	return dailyCost.Total > threshold, dailyCost.Total, nil
}
