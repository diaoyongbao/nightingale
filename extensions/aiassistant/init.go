//go:build ai_assistant
// +build ai_assistant

package aiassistant

import (
	"net/http"

	"github.com/ccfos/nightingale/v6/extensions/registry"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/logger"
)

type AIAssistantExtension struct {
	ctx *ctx.Context
}

func (a *AIAssistantExtension) Name() string {
	return "ai-assistant"
}

func (a *AIAssistantExtension) Init(ctx *ctx.Context) error {
	a.ctx = ctx
	logger.Infof("AI Assistant extension initialized")
	return nil
}

func (a *AIAssistantExtension) RegisterRoutes(router *gin.Engine) error {
	aiGroup := router.Group("/api/n9e/ai-assistant")
	{
		aiGroup.GET("/status", a.handleStatus)
		aiGroup.POST("/chat", a.handleChat)
		aiGroup.GET("/conversations", a.handleGetConversations)
	}
	logger.Infof("AI Assistant routes registered")
	return nil
}

// 路由处理函数
func (a *AIAssistantExtension) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"name":    "ai-assistant",
			"status":  "running",
			"version": "1.0.0",
		},
		"error": "",
	})
}

func (a *AIAssistantExtension) handleChat(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"message": "AI Assistant chat endpoint - 功能开发中",
			"status":  "placeholder",
		},
		"error": "",
	})
}

func (a *AIAssistantExtension) handleGetConversations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data":  []interface{}{},
		"error": "",
	})
}

func init() {
	registry.Register(&AIAssistantExtension{})
}