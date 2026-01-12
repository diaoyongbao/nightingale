// Package models AI 优化配置表模型
// n9e-2kai: AI 助手模块 - 优化配置模型（速率限制、缓存、模型路由、重试、成本）
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// AIOptimizationConfig AI 优化配置表
type AIOptimizationConfig struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ConfigType  string `json:"config_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_type_key"`
	ConfigKey   string `json:"config_key" gorm:"type:varchar(128);not null;uniqueIndex:idx_type_key"`
	ConfigValue string `json:"config_value" gorm:"type:text;not null"`
	Description string `json:"description" gorm:"type:varchar(500)"`
	Enabled     int    `json:"enabled" gorm:"default:1;index:idx_enabled"`
	CreateAt    int64  `json:"create_at" gorm:"not null"`
	CreateBy    string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt    int64  `json:"update_at" gorm:"not null"`
	UpdateBy    string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (AIOptimizationConfig) TableName() string {
	return "ai_optimization_config"
}

// 配置类型常量
const (
	OptConfigTypeRateLimit   = "rate_limit"
	OptConfigTypeCache       = "cache"
	OptConfigTypeModelRouter = "model_router"
	OptConfigTypeRetry       = "retry"
	OptConfigTypeCost        = "cost"
	OptConfigTypeConcurrent  = "concurrent"
)

// 配置 Key 常量
const (
	OptConfigKeyDefault = "default"
)


// ============ 配置结构体定义 ============

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	DefaultRPM      int            `json:"default_rpm"`       // 默认每分钟请求数
	BurstSize       int            `json:"burst_size"`        // 突发容量
	UserLimits      map[string]int `json:"user_limits"`       // 用户级别限制
	BusiGroupLimits map[int64]int  `json:"busi_group_limits"` // 业务组限制
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled         bool           `json:"enabled"`          // 是否启用缓存
	DefaultTTL      int            `json:"default_ttl"`      // 默认 TTL（秒）
	ToolTTLs        map[string]int `json:"tool_ttls"`        // 按工具设置 TTL
	IdempotentTools []string       `json:"idempotent_tools"` // 幂等工具列表
}

// ModelRouterConfig 模型路由配置
type ModelRouterConfig struct {
	TaskModels    map[string]TaskModelConfig `json:"task_models"`    // 任务类型 -> 模型配置
	FallbackModel string                     `json:"fallback_model"` // 全局降级模型
	ModelPriority []string                   `json:"model_priority"` // 模型优先级列表
}

// TaskModelConfig 任务模型配置
type TaskModelConfig struct {
	Model       string   `json:"model"`       // 主模型
	Fallbacks   []string `json:"fallbacks"`   // 降级模型列表
	MaxTokens   int      `json:"max_tokens"`  // 最大 token 数
	Temperature float64  `json:"temperature"` // 温度参数
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries     int     `json:"max_retries"`     // 最大重试次数
	InitialBackoff int     `json:"initial_backoff"` // 初始退避时间（毫秒）
	MaxBackoff     int     `json:"max_backoff"`     // 最大退避时间（毫秒）
	Multiplier     float64 `json:"multiplier"`      // 退避倍数
}

// CostConfig 成本配置
type CostConfig struct {
	ModelPrices    map[string]ModelPrice `json:"model_prices"`    // 模型价格
	AlertThreshold float64               `json:"alert_threshold"` // 告警阈值（美元/天）
}

// ModelPrice 模型价格
type ModelPrice struct {
	PromptPricePer1K     float64 `json:"prompt_price_per_1k"`
	CompletionPricePer1K float64 `json:"completion_price_per_1k"`
}

// ConcurrentConfig 并发配置
type ConcurrentConfig struct {
	MaxConcurrency int `json:"max_concurrency"` // 最大并发数
}


// ============ CRUD 方法 ============

// AIOptimizationConfigGets 查询配置列表
func AIOptimizationConfigGets(c *ctx.Context, where string, args ...interface{}) ([]AIOptimizationConfig, error) {
	var configs []AIOptimizationConfig
	session := DB(c).Order("config_type asc, config_key asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&configs).Error
	return configs, err
}

// AIOptimizationConfigGetByType 根据类型查询所有配置
func AIOptimizationConfigGetByType(c *ctx.Context, configType string) ([]AIOptimizationConfig, error) {
	var configs []AIOptimizationConfig
	err := DB(c).Where("config_type = ? AND enabled = 1", configType).Find(&configs).Error
	return configs, err
}

// AIOptimizationConfigGetByTypeAndKey 根据类型和 key 查询
func AIOptimizationConfigGetByTypeAndKey(c *ctx.Context, configType, configKey string) (*AIOptimizationConfig, error) {
	var config AIOptimizationConfig
	err := DB(c).Where("config_type = ? AND config_key = ?", configType, configKey).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

// AIOptimizationConfigGetEnabled 获取启用的配置
func AIOptimizationConfigGetEnabled(c *ctx.Context, configType, configKey string) (*AIOptimizationConfig, error) {
	var config AIOptimizationConfig
	err := DB(c).Where("config_type = ? AND config_key = ? AND enabled = 1", configType, configKey).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

// Create 创建配置
func (a *AIOptimizationConfig) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	a.CreateAt = now
	a.UpdateAt = now
	return DB(c).Create(a).Error
}

// Update 更新配置
func (a *AIOptimizationConfig) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(a).Updates(updates).Error
}

// Delete 删除配置
func (a *AIOptimizationConfig) Delete(c *ctx.Context) error {
	return DB(c).Delete(a).Error
}

// Upsert 创建或更新配置
func AIOptimizationConfigUpsert(c *ctx.Context, configType, configKey, configValue, description, updateBy string) error {
	existing, err := AIOptimizationConfigGetByTypeAndKey(c, configType, configKey)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	if existing == nil {
		// 创建
		config := &AIOptimizationConfig{
			ConfigType:  configType,
			ConfigKey:   configKey,
			ConfigValue: configValue,
			Description: description,
			Enabled:     1,
			CreateAt:    now,
			CreateBy:    updateBy,
			UpdateAt:    now,
			UpdateBy:    updateBy,
		}
		return DB(c).Create(config).Error
	}

	// 更新
	return DB(c).Model(existing).Updates(map[string]interface{}{
		"config_value": configValue,
		"description":  description,
		"update_at":    now,
		"update_by":    updateBy,
	}).Error
}


// ============ 配置解析方法 ============

// GetRateLimitConfig 获取速率限制配置
func GetRateLimitConfig(c *ctx.Context) (*RateLimitConfig, error) {
	config, err := AIOptimizationConfigGetEnabled(c, OptConfigTypeRateLimit, OptConfigKeyDefault)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// 返回默认配置
		return &RateLimitConfig{
			DefaultRPM:      10,
			BurstSize:       5,
			UserLimits:      make(map[string]int),
			BusiGroupLimits: make(map[int64]int),
		}, nil
	}

	var result RateLimitConfig
	if err := json.Unmarshal([]byte(config.ConfigValue), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCacheConfig 获取缓存配置
func GetCacheConfig(c *ctx.Context) (*CacheConfig, error) {
	config, err := AIOptimizationConfigGetEnabled(c, OptConfigTypeCache, OptConfigKeyDefault)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// 返回默认配置
		return &CacheConfig{
			Enabled:         true,
			DefaultTTL:      120,
			ToolTTLs:        make(map[string]int),
			IdempotentTools: []string{},
		}, nil
	}

	var result CacheConfig
	if err := json.Unmarshal([]byte(config.ConfigValue), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetModelRouterConfig 获取模型路由配置
func GetModelRouterConfig(c *ctx.Context) (*ModelRouterConfig, error) {
	config, err := AIOptimizationConfigGetEnabled(c, OptConfigTypeModelRouter, OptConfigKeyDefault)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// 返回默认配置（空模型，需要管理员配置）
		return &ModelRouterConfig{
			TaskModels:    make(map[string]TaskModelConfig),
			FallbackModel: "",
			ModelPriority: []string{},
		}, nil
	}

	var result ModelRouterConfig
	if err := json.Unmarshal([]byte(config.ConfigValue), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRetryConfig 获取重试配置
func GetRetryConfig(c *ctx.Context) (*RetryConfig, error) {
	config, err := AIOptimizationConfigGetEnabled(c, OptConfigTypeRetry, OptConfigKeyDefault)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// 返回默认配置
		return &RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1000,
			MaxBackoff:     30000,
			Multiplier:     2.0,
		}, nil
	}

	var result RetryConfig
	if err := json.Unmarshal([]byte(config.ConfigValue), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCostConfig 获取成本配置
func GetCostConfig(c *ctx.Context) (*CostConfig, error) {
	config, err := AIOptimizationConfigGetEnabled(c, OptConfigTypeCost, OptConfigKeyDefault)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// 返回默认配置
		return &CostConfig{
			ModelPrices:    make(map[string]ModelPrice),
			AlertThreshold: 100.0,
		}, nil
	}

	var result CostConfig
	if err := json.Unmarshal([]byte(config.ConfigValue), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConcurrentConfig 获取并发配置
func GetConcurrentConfig(c *ctx.Context) (*ConcurrentConfig, error) {
	config, err := AIOptimizationConfigGetEnabled(c, OptConfigTypeConcurrent, OptConfigKeyDefault)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// 返回默认配置
		return &ConcurrentConfig{
			MaxConcurrency: 5,
		}, nil
	}

	var result ConcurrentConfig
	if err := json.Unmarshal([]byte(config.ConfigValue), &result); err != nil {
		return nil, err
	}
	return &result, nil
}


// ============ 初始化默认配置 ============

// InitDefaultOptimizationConfigs 初始化默认优化配置
func InitDefaultOptimizationConfigs(c *ctx.Context) error {
	configs := []AIOptimizationConfig{
		{
			ConfigType:  OptConfigTypeRateLimit,
			ConfigKey:   OptConfigKeyDefault,
			ConfigValue: `{"default_rpm":10,"burst_size":5,"user_limits":{},"busi_group_limits":{}}`,
			Description: "默认速率限制配置",
			Enabled:     1,
		},
		{
			ConfigType:  OptConfigTypeCache,
			ConfigKey:   OptConfigKeyDefault,
			ConfigValue: `{"enabled":true,"default_ttl":120,"tool_ttls":{"k8s_list_pods":30,"k8s_get_nodes":60,"dbm_sql_query":300},"idempotent_tools":["k8s_list_pods","k8s_get_nodes","k8s_describe","search_ops_kb"]}`,
			Description: "工具缓存配置",
			Enabled:     1,
		},
		{
			ConfigType:  OptConfigTypeModelRouter,
			ConfigKey:   OptConfigKeyDefault,
			ConfigValue: `{"task_models":{"routing":{"model":"","fallbacks":[],"max_tokens":500,"temperature":0.1},"execution":{"model":"","fallbacks":[],"max_tokens":2000,"temperature":0.3},"summary":{"model":"","fallbacks":[],"max_tokens":4000,"temperature":0.5},"general":{"model":"","fallbacks":[],"max_tokens":2000,"temperature":0.7},"knowledge":{"model":"","fallbacks":[],"max_tokens":2000,"temperature":0.3}},"fallback_model":"","model_priority":[]}`,
			Description: "模型分层配置（管理员需配置具体模型名称）",
			Enabled:     1,
		},
		{
			ConfigType:  OptConfigTypeRetry,
			ConfigKey:   OptConfigKeyDefault,
			ConfigValue: `{"max_retries":3,"initial_backoff":1000,"max_backoff":30000,"multiplier":2.0}`,
			Description: "重试策略配置",
			Enabled:     1,
		},
		{
			ConfigType:  OptConfigTypeCost,
			ConfigKey:   OptConfigKeyDefault,
			ConfigValue: `{"model_prices":{},"alert_threshold":100.0}`,
			Description: "成本监控配置",
			Enabled:     1,
		},
		{
			ConfigType:  OptConfigTypeConcurrent,
			ConfigKey:   OptConfigKeyDefault,
			ConfigValue: `{"max_concurrency":5}`,
			Description: "并发执行配置",
			Enabled:     1,
		},
	}

	for _, cfg := range configs {
		existing, err := AIOptimizationConfigGetByTypeAndKey(c, cfg.ConfigType, cfg.ConfigKey)
		if err != nil {
			return err
		}
		if existing == nil {
			cfg.CreateBy = "system"
			cfg.UpdateBy = "system"
			if err := cfg.Create(c); err != nil {
				return err
			}
		}
	}

	return nil
}
