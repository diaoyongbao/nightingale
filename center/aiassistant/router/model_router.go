// Package router AI 助手模型路由
// n9e-2kai: AI 助手模块 - 模型分层路由器
package router

import (
	"fmt"
	"sync"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeRouting   TaskType = "routing"   // 路由决策（意图识别）
	TaskTypeExecution TaskType = "execution" // 工具执行后处理
	TaskTypeSummary   TaskType = "summary"   // 结果汇总
	TaskTypeGeneral   TaskType = "general"   // 通用对话
	TaskTypeKnowledge TaskType = "knowledge" // 知识库查询
)

// ModelRouter 模型路由器
type ModelRouter struct {
	config *models.ModelRouterConfig
	mu     sync.RWMutex
	appCtx *ctx.Context
}

// NewModelRouter 创建模型路由器
func NewModelRouter(appCtx *ctx.Context) (*ModelRouter, error) {
	config, err := models.GetModelRouterConfig(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to load model router config: %w", err)
	}

	return &ModelRouter{
		config: config,
		appCtx: appCtx,
	}, nil
}


// GetModelConfig 根据任务类型获取完整模型配置
func (r *ModelRouter) GetModelConfig(taskType TaskType) *models.TaskModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if config, ok := r.config.TaskModels[string(taskType)]; ok {
		return &config
	}

	// 如果没有配置该任务类型，返回 nil
	return nil
}

// GetModel 根据任务类型获取模型名称
func (r *ModelRouter) GetModel(taskType TaskType) string {
	config := r.GetModelConfig(taskType)
	if config != nil && config.Model != "" {
		return config.Model
	}
	return r.GetFallbackModel()
}

// GetFallbackModel 获取全局降级模型
func (r *ModelRouter) GetFallbackModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.FallbackModel
}

// GetFallbackChain 获取降级模型链
func (r *ModelRouter) GetFallbackChain(taskType TaskType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var chain []string

	// 1. 任务特定的降级列表
	if config, ok := r.config.TaskModels[string(taskType)]; ok {
		chain = append(chain, config.Fallbacks...)
	}

	// 2. 全局模型优先级列表
	chain = append(chain, r.config.ModelPriority...)

	// 3. 最终降级模型
	if r.config.FallbackModel != "" {
		chain = append(chain, r.config.FallbackModel)
	}

	return chain
}

// GetMaxTokens 获取任务类型的最大 token 数
func (r *ModelRouter) GetMaxTokens(taskType TaskType) int {
	config := r.GetModelConfig(taskType)
	if config != nil && config.MaxTokens > 0 {
		return config.MaxTokens
	}
	// 默认值
	return 2000
}

// GetTemperature 获取任务类型的温度参数
func (r *ModelRouter) GetTemperature(taskType TaskType) float64 {
	config := r.GetModelConfig(taskType)
	if config != nil {
		return config.Temperature
	}
	// 默认值
	return 0.7
}

// IsConfigured 检查任务类型是否已配置模型
func (r *ModelRouter) IsConfigured(taskType TaskType) bool {
	config := r.GetModelConfig(taskType)
	return config != nil && config.Model != ""
}

// Reload 重新加载配置
func (r *ModelRouter) Reload() error {
	config, err := models.GetModelRouterConfig(r.appCtx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config

	return nil
}

// GetConfig 获取当前配置
func (r *ModelRouter) GetConfig() *models.ModelRouterConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// ValidateConfig 验证配置
func (r *ModelRouter) ValidateConfig(config *models.ModelRouterConfig) error {
	if len(config.TaskModels) == 0 && config.FallbackModel == "" {
		return fmt.Errorf("at least one task model or fallback model must be configured")
	}
	return nil
}

// GetAllTaskTypes 获取所有支持的任务类型
func GetAllTaskTypes() []TaskType {
	return []TaskType{
		TaskTypeRouting,
		TaskTypeExecution,
		TaskTypeSummary,
		TaskTypeGeneral,
		TaskTypeKnowledge,
	}
}
