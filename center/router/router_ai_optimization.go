// Package router AI 助手优化配置路由
// n9e-2kai: AI 助手模块 - 优化配置管理路由
package router

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant/optimization"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// 优化管理器（延迟初始化）
var aiOptimizationManager *optimization.Manager

// configAIOptimizationRoutes 配置 AI 优化路由
func (rt *Router) configAIOptimizationRoutes(aiAssistant *gin.RouterGroup) {
	opt := aiAssistant.Group("/optimization")
	{
		// 配置管理
		opt.GET("/config", rt.auth(), rt.admin(), rt.aiOptConfigList)
		opt.GET("/config/:type", rt.auth(), rt.admin(), rt.aiOptConfigGet)
		opt.PUT("/config/:type", rt.auth(), rt.admin(), rt.aiOptConfigUpdate)
		opt.POST("/reload", rt.auth(), rt.admin(), rt.aiOptConfigReload)

		// 统计信息
		opt.GET("/stats", rt.auth(), rt.admin(), rt.aiOptStats)
	}

	// 成本统计
	cost := aiAssistant.Group("/cost")
	{
		cost.GET("/daily", rt.auth(), rt.admin(), rt.aiCostDaily)
		cost.GET("/daily/:date", rt.auth(), rt.admin(), rt.aiCostDailyByDate)
		cost.GET("/user/:user_id", rt.auth(), rt.admin(), rt.aiCostByUser)
	}
}


// initAIOptimization 初始化 AI 优化管理器
func (rt *Router) initAIOptimization() error {
	if aiOptimizationManager != nil {
		return nil
	}

	var err error
	aiOptimizationManager, err = optimization.NewManager(rt.Ctx, rt.Redis)
	if err != nil {
		return fmt.Errorf("failed to create optimization manager: %w", err)
	}

	return nil
}

// ===== 配置管理 =====

func (rt *Router) aiOptConfigList(c *gin.Context) {
	configs, err := models.AIOptimizationConfigGets(rt.Ctx, "enabled = 1")
	ginx.NewRender(c).Data(configs, err)
}

func (rt *Router) aiOptConfigGet(c *gin.Context) {
	configType := c.Param("type")

	configs, err := models.AIOptimizationConfigGetByType(rt.Ctx, configType)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(configs, nil)
}

func (rt *Router) aiOptConfigUpdate(c *gin.Context) {
	configType := c.Param("type")

	var req struct {
		ConfigKey   string `json:"config_key"`
		ConfigValue string `json:"config_value"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	if req.ConfigKey == "" {
		req.ConfigKey = models.OptConfigKeyDefault
	}

	// 验证 JSON 格式
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue), &jsonTest); err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid JSON format: %v", err))
		return
	}

	// 验证配置类型
	validTypes := map[string]bool{
		models.OptConfigTypeRateLimit:   true,
		models.OptConfigTypeCache:       true,
		models.OptConfigTypeModelRouter: true,
		models.OptConfigTypeRetry:       true,
		models.OptConfigTypeCost:        true,
		models.OptConfigTypeConcurrent:  true,
	}
	if !validTypes[configType] {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid config type: %s", configType))
		return
	}

	// 更新或创建配置
	err := models.AIOptimizationConfigUpsert(
		rt.Ctx,
		configType,
		req.ConfigKey,
		req.ConfigValue,
		req.Description,
		c.GetString("username"),
	)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":   "config updated successfully",
		"timestamp": time.Now().Unix(),
	}, nil)
}

func (rt *Router) aiOptConfigReload(c *gin.Context) {
	if err := rt.initAIOptimization(); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	if aiOptimizationManager == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("optimization manager not initialized"))
		return
	}

	if err := aiOptimizationManager.Reload(); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":   "optimization config reloaded successfully",
		"timestamp": time.Now().Unix(),
	}, nil)
}

// ===== 统计信息 =====

func (rt *Router) aiOptStats(c *gin.Context) {
	if err := rt.initAIOptimization(); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	if aiOptimizationManager == nil {
		ginx.NewRender(c).Data(gin.H{
			"error": "optimization manager not initialized",
		}, nil)
		return
	}

	stats := aiOptimizationManager.GetStats(c.Request.Context())
	ginx.NewRender(c).Data(stats, nil)
}

// ===== 成本统计 =====

func (rt *Router) aiCostDaily(c *gin.Context) {
	if err := rt.initAIOptimization(); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	if aiOptimizationManager == nil || aiOptimizationManager.CostTracker == nil {
		ginx.NewRender(c).Data(gin.H{
			"date":        time.Now().Format("2006-01-02"),
			"total":       0,
			"by_model":    map[string]float64{},
			"total_calls": 0,
		}, nil)
		return
	}

	date := time.Now().Format("2006-01-02")
	dailyCost, err := aiOptimizationManager.CostTracker.GetDailyCost(c.Request.Context(), date)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(dailyCost, nil)
}

func (rt *Router) aiCostDailyByDate(c *gin.Context) {
	date := c.Param("date")

	if err := rt.initAIOptimization(); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	if aiOptimizationManager == nil || aiOptimizationManager.CostTracker == nil {
		ginx.NewRender(c).Data(gin.H{
			"date":        date,
			"total":       0,
			"by_model":    map[string]float64{},
			"total_calls": 0,
		}, nil)
		return
	}

	dailyCost, err := aiOptimizationManager.CostTracker.GetDailyCost(c.Request.Context(), date)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(dailyCost, nil)
}

func (rt *Router) aiCostByUser(c *gin.Context) {
	userID := c.Param("user_id")
	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))

	if err := rt.initAIOptimization(); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	if aiOptimizationManager == nil || aiOptimizationManager.CostTracker == nil {
		ginx.NewRender(c).Data(gin.H{
			"user_id": userID,
			"date":    date,
			"total":   0,
			"calls":   0,
		}, nil)
		return
	}

	total, calls, err := aiOptimizationManager.CostTracker.GetUserCost(c.Request.Context(), userID, date)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"user_id": userID,
		"date":    date,
		"total":   total,
		"calls":   calls,
	}, nil)
}

// GetOptimizationManager 获取优化管理器（供其他模块使用）
func (rt *Router) GetOptimizationManager() *optimization.Manager {
	rt.initAIOptimization()
	return aiOptimizationManager
}
