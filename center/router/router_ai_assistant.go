// Package router AI 助手路由
// n9e-2kai: AI 助手模块 - 路由注册
package router

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant/chat"
	"github.com/ccfos/nightingale/v6/center/aiassistant/config"
	"github.com/ccfos/nightingale/v6/center/aiassistant/confirmation"
	"github.com/ccfos/nightingale/v6/center/aiassistant/knowledge"
	"github.com/ccfos/nightingale/v6/center/aiassistant/optimization"
	"github.com/ccfos/nightingale/v6/center/aiassistant/risk"
	"github.com/ccfos/nightingale/v6/center/aiassistant/session"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

// AI 助手相关组件（延迟初始化）
var (
	aiChatHandler       *chat.Handler
	aiConfigLoader      *config.Loader
	aiOptManager        *optimization.Manager
	aiKnowledgeRegistry *knowledge.KnowledgeToolRegistry
	aiInitialized       bool
)

// configAIAssistantRoutes 配置 AI 助手路由
func (rt *Router) configAIAssistantRoutes(pages *gin.RouterGroup) {
	aiAssistant := pages.Group("/ai-assistant")
	{
		// 健康检查
		aiAssistant.GET("/health", rt.aiAssistantHealth)

		// 对话 API
		aiAssistant.POST("/chat", rt.auth(), rt.aiAssistantChat)
		aiAssistant.POST("/chat/upload", rt.auth(), rt.aiAssistantUpload)

		// 知识库 API（旧版兼容）
		aiAssistant.POST("/knowledge/query", rt.auth(), rt.aiAssistantKnowledgeQuery)

		// 知识库 Provider 管理（仅 Admin）
		aiAssistant.GET("/knowledge-providers", rt.auth(), rt.admin(), rt.knowledgeProviderList)
		aiAssistant.POST("/knowledge-providers", rt.auth(), rt.admin(), rt.knowledgeProviderCreate)
		aiAssistant.PUT("/knowledge-providers/:id", rt.auth(), rt.admin(), rt.knowledgeProviderUpdate)
		aiAssistant.DELETE("/knowledge-providers/:id", rt.auth(), rt.admin(), rt.knowledgeProviderDelete)
		aiAssistant.POST("/knowledge-providers/:id/test", rt.auth(), rt.admin(), rt.knowledgeProviderTest)

		// 知识库工具管理（仅 Admin）
		aiAssistant.GET("/knowledge-tools", rt.auth(), rt.admin(), rt.knowledgeToolList)
		aiAssistant.POST("/knowledge-tools", rt.auth(), rt.admin(), rt.knowledgeToolCreate)
		aiAssistant.PUT("/knowledge-tools/:id", rt.auth(), rt.admin(), rt.knowledgeToolUpdate)
		aiAssistant.DELETE("/knowledge-tools/:id", rt.auth(), rt.admin(), rt.knowledgeToolDelete)

		// 知识库配置重载
		aiAssistant.POST("/knowledge-reload", rt.auth(), rt.admin(), rt.knowledgeReload)

		// 文件下载
		aiAssistant.GET("/files/download/:path", rt.aiAssistantFileDownload)

		// 会话管理
		aiAssistant.GET("/sessions/stats", rt.auth(), rt.aiAssistantSessionStats)
		aiAssistant.GET("/sessions/:session_id", rt.auth(), rt.aiAssistantSessionGet)
		aiAssistant.DELETE("/sessions/:session_id", rt.auth(), rt.aiAssistantSessionDelete)
		aiAssistant.POST("/sessions/archive", rt.auth(), rt.admin(), rt.aiAssistantSessionArchive)

		// MCP Server 管理（仅 Admin）
		aiAssistant.GET("/mcp/servers", rt.auth(), rt.admin(), rt.mcpServerList)
		aiAssistant.POST("/mcp/servers", rt.auth(), rt.admin(), rt.mcpServerCreate)
		aiAssistant.PUT("/mcp/servers/:id", rt.auth(), rt.admin(), rt.mcpServerUpdate)
		aiAssistant.DELETE("/mcp/servers/:id", rt.auth(), rt.admin(), rt.mcpServerDelete)

		// MCP 模板管理
		aiAssistant.GET("/mcp/templates", rt.auth(), rt.mcpTemplateList)
		aiAssistant.POST("/mcp/templates", rt.auth(), rt.admin(), rt.mcpTemplateCreate)
		aiAssistant.PUT("/mcp/templates/:id", rt.auth(), rt.admin(), rt.mcpTemplateUpdate)
		aiAssistant.DELETE("/mcp/templates/:id", rt.auth(), rt.admin(), rt.mcpTemplateDelete)

		// AI 配置管理（仅 Admin）
		aiAssistant.GET("/config", rt.auth(), rt.admin(), rt.aiConfigList)
		aiAssistant.GET("/config/:key", rt.auth(), rt.admin(), rt.aiConfigGet)
		aiAssistant.PUT("/config/:key", rt.auth(), rt.admin(), rt.aiConfigUpdate)
		aiAssistant.POST("/config/reload", rt.auth(), rt.admin(), rt.aiConfigReload)
		aiAssistant.POST("/config/test", rt.auth(), rt.admin(), rt.aiConfigTest)

		// n9e-2kai: 优化配置管理（仅 Admin）
		aiAssistant.GET("/optimization/config", rt.auth(), rt.admin(), rt.optimizationConfigList)
		aiAssistant.PUT("/optimization/config/:type", rt.auth(), rt.admin(), rt.optimizationConfigUpdate)
		aiAssistant.POST("/optimization/reload", rt.auth(), rt.admin(), rt.optimizationReload)
		aiAssistant.GET("/optimization/stats", rt.auth(), rt.admin(), rt.optimizationStats)

		// n9e-2kai: 成本统计查询（仅 Admin）
		aiAssistant.GET("/cost/daily", rt.auth(), rt.admin(), rt.costDailyStats)
		aiAssistant.GET("/cost/user/:user_id", rt.auth(), rt.admin(), rt.costUserStats)

		// n9e-2kai: 动态 Agent 管理
		aiAssistant.GET("/agents", rt.auth(), rt.aiAgentList)                            // 获取 Agent 列表（用于 @Mention）
		aiAssistant.GET("/agents/:id", rt.auth(), rt.admin(), rt.aiAgentGet)             // 获取单个 Agent
		aiAssistant.POST("/agents", rt.auth(), rt.admin(), rt.aiAgentCreate)             // 创建 Agent
		aiAssistant.PUT("/agents/:id", rt.auth(), rt.admin(), rt.aiAgentUpdate)          // 更新 Agent
		aiAssistant.DELETE("/agents/:id", rt.auth(), rt.admin(), rt.aiAgentDelete)       // 删除 Agent
		aiAssistant.PUT("/agents/:id/tools", rt.auth(), rt.admin(), rt.aiAgentBindTools) // 绑定工具

		// n9e-2kai: 动态 Tool 管理
		aiAssistant.GET("/tools", rt.auth(), rt.admin(), rt.aiToolList)          // 获取 Tool 列表
		aiAssistant.GET("/tools/:id", rt.auth(), rt.admin(), rt.aiToolGet)       // 获取单个 Tool
		aiAssistant.POST("/tools", rt.auth(), rt.admin(), rt.aiToolCreate)       // 创建 Tool
		aiAssistant.PUT("/tools/:id", rt.auth(), rt.admin(), rt.aiToolUpdate)    // 更新 Tool
		aiAssistant.DELETE("/tools/:id", rt.auth(), rt.admin(), rt.aiToolDelete) // 删除 Tool
	}
}

// ===== 健康检查 =====

func (rt *Router) aiAssistantHealth(c *gin.Context) {
	health := gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"components": gin.H{
			"database": "ok",
			"redis":    "unknown",
			"ai":       "unknown",
		},
	}

	// 检查数据库连接
	if rt.Ctx.DB == nil {
		health["components"].(gin.H)["database"] = "error"
		health["status"] = "degraded"
	}

	// 检查 Redis 连接
	if rt.Redis != nil {
		if err := rt.Redis.Ping(c.Request.Context()).Err(); err == nil {
			health["components"].(gin.H)["redis"] = "ok"
		} else {
			health["components"].(gin.H)["redis"] = "error"
			health["status"] = "degraded"
		}
	} else {
		health["components"].(gin.H)["redis"] = "not_configured"
	}

	// 检查 AI 服务配置
	// n9e-2kai: 直接从数据库读取配置，不依赖延迟初始化的 aiConfigLoader
	aiConfig, err := models.AIConfigGetByKey(rt.Ctx, models.AIConfigKeyDefaultModel)
	if err == nil && aiConfig != nil && aiConfig.ConfigValue != "" {
		// 解析配置检查 API Key 是否存在
		var modelConfig config.AIModelConfig
		if json.Unmarshal([]byte(aiConfig.ConfigValue), &modelConfig) == nil && modelConfig.APIKey != "" {
			health["components"].(gin.H)["ai"] = "configured"
		} else {
			health["components"].(gin.H)["ai"] = "not_configured"
		}
	} else {
		health["components"].(gin.H)["ai"] = "not_configured"
	}

	ginx.NewRender(c).Data(health, nil)
}

// ===== 对话 API =====

// initAIAssistant 初始化 AI 助手组件（延迟初始化）
func (rt *Router) initAIAssistant() {
	if aiInitialized {
		return
	}

	// 初始化配置加载器
	aiConfigLoader = config.NewLoader(rt.Ctx)

	// 尝试初始化 AI 客户端
	var aiClient ai.AIClient
	modelConfig, err := aiConfigLoader.GetAIModelConfig(models.AIConfigKeyDefaultModel)
	if err == nil && modelConfig != nil && modelConfig.APIKey != "" {
		client, err := ai.NewOpenAIClient(ai.OpenAIClientConfig{
			Model:         modelConfig.Model,
			APIKey:        modelConfig.APIKey,
			BaseURL:       modelConfig.BaseURL,
			Timeout:       60 * time.Second,
			SkipSSLVerify: false,
		})
		if err == nil {
			aiClient = client
			logger.Infof("AI client initialized with model: %s", modelConfig.Model)
		} else {
			logger.Warningf("Failed to create AI client: %v", err)
		}
	} else {
		logger.Infof("AI model not configured, chat will return placeholder response")
	}

	// 初始化会话管理器（如果 Redis 可用）
	var sessionMgr *session.Manager
	if rt.Redis != nil {
		sessionConfig, _ := aiConfigLoader.GetSessionConfig()
		sessionMgr = session.NewManager(rt.Redis, &session.Config{
			TTL:                   time.Duration(sessionConfig.TTL) * time.Second,
			MaxMessagesPerSession: sessionConfig.MaxMessagesPerSession,
			MaxSessionsPerUser:    sessionConfig.MaxSessionsPerUser,
			RedisPrefix:           "ai_assistant:",
		})
		logger.Infof("Session manager initialized")
	}

	// 初始化确认管理器
	var confirmMgr *confirmation.Manager
	if rt.Redis != nil {
		confirmMgr = confirmation.NewManager(rt.Redis, confirmation.DefaultConfig())
	}

	// 初始化风险检查器
	riskChecker := risk.NewChecker(risk.DefaultConfig())

	// 初始化知识库工具注册表（Function Calling 架构）
	aiKnowledgeRegistry = knowledge.NewKnowledgeToolRegistry(rt.Ctx)
	if err := aiKnowledgeRegistry.LoadFromConfig(); err != nil {
		logger.Warningf("Failed to load knowledge tools: %v", err)
	} else {
		logger.Infof("Knowledge tool registry initialized with %d tools", aiKnowledgeRegistry.GetToolCount())
	}

	// n9e-2kai: 初始化优化管理器
	if rt.Redis != nil {
		optMgr, err := optimization.NewManager(rt.Ctx, rt.Redis)
		if err != nil {
			logger.Warningf("Failed to create optimization manager: %v", err)
		} else {
			aiOptManager = optMgr
			logger.Infof("Optimization manager initialized")
		}
	}

	// 创建 Chat Handler
	aiChatHandler = chat.NewHandler(rt.Ctx, aiClient, sessionMgr, confirmMgr, riskChecker, aiKnowledgeRegistry, aiOptManager)

	aiInitialized = true
	logger.Infof("AI Assistant initialized successfully")
}

func (rt *Router) aiAssistantChat(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	var req chat.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 获取用户 ID
	userID := c.GetInt64("userid")

	// 调用 Chat Handler
	resp, err := aiChatHandler.HandleChat(c.Request.Context(), &req, userID)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(resp, nil)
}

func (rt *Router) aiAssistantUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	defer file.Close()

	// 延迟初始化 AI 助手组件
	rt.initAIAssistant()

	// 获取文件配置
	if aiConfigLoader == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("AI assistant not initialized"))
		return
	}

	fileConfig, err := aiConfigLoader.GetFileConfig()
	if err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("failed to get file config: %v", err))
		return
	}

	// 验证文件大小
	if header.Size > fileConfig.MaxSize {
		ginx.NewRender(c).Data(nil, fmt.Errorf("file size %d exceeds limit %d", header.Size, fileConfig.MaxSize))
		return
	}

	// 验证文件类型（如果配置了允许的类型）
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		// 尝试从文件扩展名推断
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		case ".gif":
			mimeType = "image/gif"
		case ".txt":
			mimeType = "text/plain"
		case ".json":
			mimeType = "application/json"
		case ".pdf":
			mimeType = "application/pdf"
		default:
			mimeType = "application/octet-stream"
		}
	}

	// 生成文件 ID 和存储路径
	fileID := "file_" + uuid.New().String()
	fileName := fmt.Sprintf("%s_%s", fileID, header.Filename)

	// 确保存储目录存在
	if err := os.MkdirAll(fileConfig.StoragePath, 0755); err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("failed to create storage directory: %v", err))
		return
	}

	// 保存文件
	filePath := filepath.Join(fileConfig.StoragePath, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("failed to create file: %v", err))
		return
	}
	defer dst.Close()

	// 复制文件内容并计算 SHA256
	hasher := sha256.New()
	writer := io.MultiWriter(dst, hasher)

	if _, err := io.Copy(writer, file); err != nil {
		os.Remove(filePath) // 清理失败的文件
		ginx.NewRender(c).Data(nil, fmt.Errorf("failed to save file: %v", err))
		return
	}

	// 计算文件哈希
	sha256Hash := fmt.Sprintf("%x", hasher.Sum(nil))

	// 生成下载 token 和 URL
	downloadToken := rt.generateDownloadToken(fileName)
	downloadURL := fmt.Sprintf("/api/n9e/ai-assistant/files/download/%s?token=%s", fileName, downloadToken)

	// 返回文件信息
	ginx.NewRender(c).Data(gin.H{
		"file_id":      fileID,
		"file_name":    header.Filename,
		"mime_type":    mimeType,
		"size":         header.Size,
		"sha256":       sha256Hash,
		"download_url": downloadURL,
		"expires_at":   time.Now().Add(time.Duration(fileConfig.TTL) * time.Second).Unix(),
	}, nil)
}

// ===== 知识库 API =====

func (rt *Router) aiAssistantKnowledgeQuery(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	var req struct {
		Query          string `json:"query"`
		ConversationID string `json:"conversation_id"`
		BotID          string `json:"bot_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	userID := c.GetInt64("userid")

	// 检查知识库是否配置
	knowledgeConfig, err := aiConfigLoader.GetKnowledgeConfig()
	if err != nil || knowledgeConfig == nil || knowledgeConfig.APIKey == "" {
		ginx.NewRender(c).Data(gin.H{
			"answer": "知识库服务未配置，请联系管理员在 AI 配置中设置知识库。",
			"status": "error",
		}, nil)
		return
	}

	// 创建知识库提供者
	provider := knowledge.NewCozeProvider(knowledge.CozeConfig{
		BaseURL:      knowledgeConfig.BaseURL,
		APIKey:       knowledgeConfig.APIKey,
		DefaultBotID: knowledgeConfig.DefaultBotID,
	})

	// 查询知识库
	botID := req.BotID
	if botID == "" {
		botID = knowledgeConfig.DefaultBotID
	}

	queryResp, err := provider.Query(c.Request.Context(), &knowledge.QueryRequest{
		UserID:         fmt.Sprintf("%d", userID),
		ConversationID: req.ConversationID,
		Message:        req.Query,
		BotID:          botID,
	})

	if err != nil {
		ginx.NewRender(c).Data(gin.H{
			"answer": "知识库查询失败: " + err.Error(),
			"status": "error",
		}, nil)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"answer":          queryResp.Answer,
		"conversation_id": queryResp.ConversationID,
		"status":          queryResp.Status,
	}, nil)
}

// ===== 文件下载 =====

func (rt *Router) aiAssistantFileDownload(c *gin.Context) {
	path := c.Param("path")
	token := c.Query("token")

	// 验证路径穿越
	if strings.Contains(path, "..") || strings.Contains(path, "\\") || strings.HasPrefix(path, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
		return
	}

	// 验证下载 token（如果提供）
	if token != "" {
		// 简单的 token 验证逻辑，实际应该使用更安全的方式
		// 这里可以验证 token 的有效性和过期时间
		if !rt.validateDownloadToken(token, path) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired download token"})
			return
		}
	}

	// 延迟初始化 AI 助手组件
	rt.initAIAssistant()

	// 获取文件配置
	if aiConfigLoader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI assistant not initialized"})
		return
	}

	fileConfig, err := aiConfigLoader.GetFileConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get file config"})
		return
	}

	// 构建完整文件路径
	fullPath := filepath.Join(fileConfig.StoragePath, path)

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// 获取文件信息
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get file info"})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(path)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// 发送文件
	c.File(fullPath)
}

// ===== 会话管理 =====

func (rt *Router) aiAssistantSessionStats(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	// 如果 Redis 不可用，返回空统计
	if rt.Redis == nil {
		ginx.NewRender(c).Data(gin.H{
			"active_count":     0,
			"last24h_created":  0,
			"last24h_active":   0,
			"storage_estimate": "Redis not configured",
		}, nil)
		return
	}

	// 创建临时 session manager 获取统计
	sessionMgr := session.NewManager(rt.Redis, session.DefaultConfig())
	stats, err := sessionMgr.GetStats(c.Request.Context())
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(stats, nil)
}

func (rt *Router) aiAssistantSessionGet(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	sessionID := c.Param("session_id")

	if rt.Redis == nil {
		ginx.NewRender(c).Data(gin.H{
			"session_id": sessionID,
			"messages":   []interface{}{},
			"error":      "Redis not configured",
		}, nil)
		return
	}

	sessionMgr := session.NewManager(rt.Redis, session.DefaultConfig())

	// 获取会话信息
	sess, err := sessionMgr.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 获取消息
	messages, _ := sessionMgr.GetMessages(c.Request.Context(), sessionID, 100)

	ginx.NewRender(c).Data(gin.H{
		"session_id":     sess.ID,
		"user_id":        sess.UserID,
		"mode":           sess.Mode,
		"created_at":     sess.CreatedAt,
		"last_active_at": sess.LastActiveAt,
		"message_count":  sess.MessageCount,
		"messages":       messages,
	}, nil)
}

func (rt *Router) aiAssistantSessionDelete(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	sessionID := c.Param("session_id")
	username := c.GetString("username")
	userID := c.GetInt64("userid")

	if rt.Redis == nil {
		ginx.NewRender(c).Data(gin.H{
			"message": "Redis not configured",
		}, nil)
		return
	}

	sessionMgr := session.NewManager(rt.Redis, session.DefaultConfig())

	// 检查用户是否是 Admin
	user, err := models.UserGetByUsername(rt.Ctx, username)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	isAdmin := user != nil && user.IsAdmin()

	// 如果不是 Admin，需要验证会话所有权
	if !isAdmin {
		isOwner, err := sessionMgr.CheckSessionOwner(c.Request.Context(), sessionID, userID)
		if err != nil {
			ginx.NewRender(c).Data(nil, err)
			return
		}
		if !isOwner {
			ginx.NewRender(c).Data(nil, gin.Error{
				Err:  nil,
				Type: gin.ErrorTypePublic,
				Meta: "无权删除此会话",
			})
			return
		}
	}

	// 删除会话
	err = sessionMgr.DeleteSession(c.Request.Context(), sessionID)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":    "session deleted",
		"session_id": sessionID,
	}, nil)
}

func (rt *Router) aiAssistantSessionArchive(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	if rt.Redis == nil {
		ginx.NewRender(c).Data(gin.H{
			"archived_count": 0,
			"message":        "Redis not configured",
		}, nil)
		return
	}

	var req struct {
		BeforeDays int `json:"before_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.BeforeDays = 7 // 默认归档 7 天前的会话
	}

	sessionMgr := session.NewManager(rt.Redis, session.DefaultConfig())

	// 获取不活跃会话
	threshold := time.Duration(req.BeforeDays) * 24 * time.Hour
	inactiveSessions, err := sessionMgr.GetInactiveSessions(c.Request.Context(), threshold)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 归档会话（这里简化处理，只删除 Redis 中的数据）
	// TODO: 实际应该先保存到数据库再删除
	archivedCount := 0
	for _, sessionID := range inactiveSessions {
		if err := sessionMgr.DeleteSession(c.Request.Context(), sessionID); err == nil {
			archivedCount++
		}
	}

	ginx.NewRender(c).Data(gin.H{
		"archived_count": archivedCount,
	}, nil)
}

// ===== MCP Server 管理 =====

func (rt *Router) mcpServerList(c *gin.Context) {
	servers, err := models.MCPServerGets(rt.Ctx, "")
	ginx.NewRender(c).Data(servers, err)
}

func (rt *Router) mcpServerCreate(c *gin.Context) {
	var server models.MCPServer
	if err := c.ShouldBindJSON(&server); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	server.CreateBy = c.GetString("username")
	server.UpdateBy = c.GetString("username")

	err := server.Create(rt.Ctx)
	ginx.NewRender(c).Data(server.Id, err)
}

func (rt *Router) mcpServerUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	server, err := models.MCPServerGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if server == nil {
		ginx.NewRender(c).Data(nil, nil)
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	updates["update_by"] = c.GetString("username")
	err = server.Update(rt.Ctx, updates)
	ginx.NewRender(c).Data("ok", err)
}

func (rt *Router) mcpServerDelete(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	server, err := models.MCPServerGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if server == nil {
		ginx.NewRender(c).Data(nil, nil)
		return
	}

	err = server.Delete(rt.Ctx)
	ginx.NewRender(c).Data("ok", err)
}

// ===== MCP 模板管理 =====

func (rt *Router) mcpTemplateList(c *gin.Context) {
	templates, err := models.MCPTemplateGets(rt.Ctx, "")
	ginx.NewRender(c).Data(templates, err)
}

func (rt *Router) mcpTemplateCreate(c *gin.Context) {
	var template models.MCPTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	template.CreateBy = c.GetString("username")
	template.UpdateBy = c.GetString("username")

	err := template.Create(rt.Ctx)
	ginx.NewRender(c).Data(template.Id, err)
}

func (rt *Router) mcpTemplateUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	template, err := models.MCPTemplateGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if template == nil {
		ginx.NewRender(c).Data(nil, nil)
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	updates["update_by"] = c.GetString("username")
	err = template.Update(rt.Ctx, updates)
	ginx.NewRender(c).Data("ok", err)
}

func (rt *Router) mcpTemplateDelete(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	template, err := models.MCPTemplateGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if template == nil {
		ginx.NewRender(c).Data(nil, nil)
		return
	}

	err = template.Delete(rt.Ctx)
	ginx.NewRender(c).Data("ok", err)
}

// ===== AI 配置管理 =====

func (rt *Router) aiConfigList(c *gin.Context) {
	configs, err := models.AIConfigGets(rt.Ctx, "enabled = 1")
	ginx.NewRender(c).Data(configs, err)
}

func (rt *Router) aiConfigGet(c *gin.Context) {
	key := c.Param("key")
	config, err := models.AIConfigGetByKey(rt.Ctx, key)
	ginx.NewRender(c).Data(config, err)
}

func (rt *Router) aiConfigUpdate(c *gin.Context) {
	key := c.Param("key")

	var req struct {
		ConfigValue string `json:"config_value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证 JSON 格式
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue), &jsonTest); err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid JSON format: %v", err))
		return
	}

	// 更新配置
	err := models.AIConfigUpdateValue(rt.Ctx, key, req.ConfigValue, c.GetString("username"))
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 自动重新加载配置缓存
	if aiConfigLoader != nil {
		aiConfigLoader.ReloadAll()
	}

	// 重置初始化状态，确保下次请求使用新配置
	aiInitialized = false
	aiChatHandler = nil

	ginx.NewRender(c).Data(gin.H{
		"message":   "config updated and reloaded successfully",
		"timestamp": time.Now().Unix(),
	}, nil)
}

func (rt *Router) aiConfigReload(c *gin.Context) {
	// 重置初始化状态，下次请求时重新初始化
	aiInitialized = false
	aiChatHandler = nil

	// 如果配置加载器存在，重新加载配置
	if aiConfigLoader != nil {
		aiConfigLoader.ReloadAll()
	}

	ginx.NewRender(c).Data(gin.H{
		"message":   "config reload triggered, AI assistant will reinitialize on next request",
		"timestamp": time.Now().Unix(),
	}, nil)
}
func (rt *Router) aiConfigTest(c *gin.Context) {
	var req struct {
		ConfigKey   string `json:"config_key"`
		ConfigValue string `json:"config_value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证 JSON 格式
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue), &jsonTest); err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid JSON format: %v", err))
		return
	}

	// 根据配置类型进行测试
	switch req.ConfigKey {
	case models.AIConfigKeyDefaultModel:
		// 测试 AI 模型配置
		result, err := rt.testAIModelConfig(req.ConfigValue)
		ginx.NewRender(c).Data(result, err)
	case models.AIConfigKeyKnowledge:
		// 测试知识库配置
		result, err := rt.testKnowledgeConfig(req.ConfigValue)
		ginx.NewRender(c).Data(result, err)
	default:
		ginx.NewRender(c).Data(gin.H{
			"status":  "success",
			"message": "Configuration format is valid",
		}, nil)
	}
}

// testAIModelConfig 测试 AI 模型配置
func (rt *Router) testAIModelConfig(configValue string) (gin.H, error) {
	var modelConfig config.AIModelConfig
	if err := json.Unmarshal([]byte(configValue), &modelConfig); err != nil {
		return nil, fmt.Errorf("invalid AI model config format: %v", err)
	}

	// 验证必要字段
	if modelConfig.APIKey == "" {
		return gin.H{
			"status":  "error",
			"message": "API Key is required",
		}, nil
	}

	if modelConfig.BaseURL == "" {
		return gin.H{
			"status":  "error",
			"message": "Base URL is required",
		}, nil
	}

	if modelConfig.Model == "" {
		return gin.H{
			"status":  "error",
			"message": "Model name is required",
		}, nil
	}

	// 创建临时 AI 客户端进行测试
	aiClient, err := ai.NewOpenAIClient(ai.OpenAIClientConfig{
		Model:   modelConfig.Model,
		APIKey:  modelConfig.APIKey,
		BaseURL: modelConfig.BaseURL,
		Timeout: time.Duration(30) * time.Second,
	})
	if err != nil {
		return gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to create AI client: %v", err),
		}, nil
	}

	// 发送测试请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testMessage := "Hello, this is a connection test. Please respond with 'OK'."
	response, err := aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
		Model: modelConfig.Model,
		Messages: []ai.Message{
			{Role: "user", Content: testMessage},
		},
		MaxTokens:   100,
		Temperature: 0.1,
	})

	if err != nil {
		return gin.H{
			"status":  "error",
			"message": fmt.Sprintf("AI model test failed: %v", err),
		}, nil
	}

	return gin.H{
		"status":   "success",
		"message":  "AI model connection test successful",
		"response": response.Choices[0].Message.Content,
		"model":    response.Model,
		"usage":    response.Usage,
	}, nil
}

// testKnowledgeConfig 测试知识库配置
func (rt *Router) testKnowledgeConfig(configValue string) (gin.H, error) {
	var knowledgeConfig config.KnowledgeConfig
	if err := json.Unmarshal([]byte(configValue), &knowledgeConfig); err != nil {
		return nil, fmt.Errorf("invalid knowledge config format: %v", err)
	}

	// 验证必要字段
	if knowledgeConfig.Type == "" {
		return gin.H{
			"status":  "error",
			"message": "Type is required",
		}, nil
	}

	if knowledgeConfig.APIKey == "" {
		return gin.H{
			"status":  "error",
			"message": "API Key is required",
		}, nil
	}

	if knowledgeConfig.BaseURL == "" {
		return gin.H{
			"status":  "error",
			"message": "Base URL is required",
		}, nil
	}

	// 创建知识库提供者进行测试
	provider := knowledge.NewCozeProvider(knowledge.CozeConfig{
		APIKey:       knowledgeConfig.APIKey,
		BaseURL:      knowledgeConfig.BaseURL,
		DefaultBotID: knowledgeConfig.DefaultBotID,
	})

	// 测试健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		return gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Knowledge provider health check failed: %v", err),
		}, nil
	}

	return gin.H{
		"status":  "success",
		"message": "Knowledge provider connection test successful",
	}, nil
}

// validateDownloadToken 验证下载 token
func (rt *Router) validateDownloadToken(token, path string) bool {
	// 简单的 token 验证逻辑
	// 实际应该使用更安全的方式，比如 JWT 或 Redis 存储

	// 这里使用简单的格式：dl_<timestamp>_<hash>
	if !strings.HasPrefix(token, "dl_") {
		return false
	}

	// 可以添加更复杂的验证逻辑，比如：
	// 1. 解析 token 中的时间戳，检查是否过期
	// 2. 验证 token 的签名
	// 3. 检查 token 是否在 Redis 中存在

	// 目前简单返回 true，实际使用时需要实现完整的验证逻辑
	return true
}

// generateDownloadToken 生成下载 token
func (rt *Router) generateDownloadToken(path string) string {
	// 生成带时间戳的下载 token
	timestamp := time.Now().Unix()
	// 这里应该添加签名或哈希来确保安全性
	return fmt.Sprintf("dl_%d_%s", timestamp, uuid.New().String()[:8])
}

// ===== 知识库 Provider 管理 =====

func (rt *Router) knowledgeProviderList(c *gin.Context) {
	providers, err := models.KnowledgeProviderGets(rt.Ctx, "")
	ginx.NewRender(c).Data(providers, err)
}

func (rt *Router) knowledgeProviderCreate(c *gin.Context) {
	var provider models.KnowledgeProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证 JSON 格式
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(provider.Config), &jsonTest); err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid config JSON format: %v", err))
		return
	}

	provider.CreateBy = c.GetString("username")
	provider.UpdateBy = c.GetString("username")

	err := provider.Create(rt.Ctx)
	ginx.NewRender(c).Data(provider.Id, err)
}

func (rt *Router) knowledgeProviderUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	provider, err := models.KnowledgeProviderGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if provider == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("provider not found"))
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 如果更新了 config，验证 JSON 格式
	if configStr, ok := updates["config"].(string); ok {
		var jsonTest interface{}
		if err := json.Unmarshal([]byte(configStr), &jsonTest); err != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("invalid config JSON format: %v", err))
			return
		}
	}

	updates["update_by"] = c.GetString("username")
	err = provider.Update(rt.Ctx, updates)
	ginx.NewRender(c).Data("ok", err)
}

func (rt *Router) knowledgeProviderDelete(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	provider, err := models.KnowledgeProviderGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if provider == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("provider not found"))
		return
	}

	// 检查是否有工具引用此 Provider
	tools, err := models.KnowledgeToolGetByProviderID(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if len(tools) > 0 {
		ginx.NewRender(c).Data(nil, fmt.Errorf("cannot delete provider: %d tools are using it", len(tools)))
		return
	}

	err = provider.Delete(rt.Ctx)
	ginx.NewRender(c).Data("ok", err)
}

// knowledgeProviderTest 测试知识库提供者连接
func (rt *Router) knowledgeProviderTest(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	provider, err := models.KnowledgeProviderGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if provider == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("provider not found"))
		return
	}

	// 根据 Provider 类型进行测试
	switch provider.ProviderType {
	case models.ProviderTypeCloudflareAutoRAG:
		result, err := rt.testCloudflareRAGProvider(provider)
		if err != nil {
			ginx.NewRender(c).Data(gin.H{
				"status":  "error",
				"message": err.Error(),
			}, nil)
			return
		}
		ginx.NewRender(c).Data(result, nil)
	default:
		ginx.NewRender(c).Data(gin.H{
			"status":  "error",
			"message": fmt.Sprintf("unsupported provider type: %s", provider.ProviderType),
		}, nil)
	}
}

// testCloudflareRAGProvider 测试 Cloudflare AutoRAG 连接
func (rt *Router) testCloudflareRAGProvider(provider *models.KnowledgeProvider) (gin.H, error) {
	// 解析配置
	config, err := provider.GetCloudflareConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	// 验证必要字段
	if config.AccountID == "" {
		return gin.H{"status": "error", "message": "Account ID is required"}, nil
	}
	if config.RAGName == "" {
		return gin.H{"status": "error", "message": "RAG Name is required"}, nil
	}
	if config.APIToken == "" {
		return gin.H{"status": "error", "message": "API Token is required"}, nil
	}

	// 创建 Provider 实例并测试
	ragConfig := &knowledge.CloudflareRAGConfig{
		AccountID:      config.AccountID,
		RAGName:        config.RAGName,
		APIToken:       config.APIToken,
		Model:          config.Model,
		RewriteQuery:   config.RewriteQuery,
		MaxNumResults:  config.MaxNumResults,
		ScoreThreshold: config.ScoreThreshold,
		Timeout:        config.Timeout,
	}
	ragProvider := knowledge.NewCloudflareRAGProvider(provider.Name, ragConfig)

	// 执行健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ragProvider.Health(ctx); err != nil {
		return gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Connection test failed: %v", err),
		}, nil
	}

	return gin.H{
		"status":  "success",
		"message": "Connection test successful",
	}, nil
}

// ===== 知识库工具管理 =====

func (rt *Router) knowledgeToolList(c *gin.Context) {
	tools, err := models.KnowledgeToolGetEnabledWithProvider(rt.Ctx)
	ginx.NewRender(c).Data(tools, err)
}

func (rt *Router) knowledgeToolCreate(c *gin.Context) {
	var tool models.KnowledgeTool
	if err := c.ShouldBindJSON(&tool); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证 Provider 存在
	provider, err := models.KnowledgeProviderGetById(rt.Ctx, tool.ProviderID)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if provider == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("provider not found: %d", tool.ProviderID))
		return
	}

	tool.CreateBy = c.GetString("username")
	tool.UpdateBy = c.GetString("username")

	err = tool.Create(rt.Ctx)
	ginx.NewRender(c).Data(tool.Id, err)
}

func (rt *Router) knowledgeToolUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	tool, err := models.KnowledgeToolGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if tool == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("tool not found"))
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 如果更新了 provider_id，验证 Provider 存在
	if providerID, ok := updates["provider_id"].(float64); ok {
		provider, err := models.KnowledgeProviderGetById(rt.Ctx, int64(providerID))
		if err != nil {
			ginx.NewRender(c).Data(nil, err)
			return
		}
		if provider == nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("provider not found: %d", int64(providerID)))
			return
		}
	}

	updates["update_by"] = c.GetString("username")
	err = tool.Update(rt.Ctx, updates)
	ginx.NewRender(c).Data("ok", err)
}

func (rt *Router) knowledgeToolDelete(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	tool, err := models.KnowledgeToolGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if tool == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("tool not found"))
		return
	}

	err = tool.Delete(rt.Ctx)
	ginx.NewRender(c).Data("ok", err)
}

// ===== 知识库配置重载 =====

func (rt *Router) knowledgeReload(c *gin.Context) {
	// 重置初始化状态，强制重新初始化
	aiInitialized = false
	aiChatHandler = nil

	// 重新初始化 AI 助手组件
	rt.initAIAssistant()

	ginx.NewRender(c).Data(gin.H{
		"message":        "knowledge config reloaded successfully",
		"provider_count": aiKnowledgeRegistry.GetProviderCount(),
		"tool_count":     aiKnowledgeRegistry.GetToolCount(),
		"timestamp":      time.Now().Unix(),
	}, nil)
}

// ===== 优化配置管理 =====

// optimizationConfigList 获取所有优化配置
func (rt *Router) optimizationConfigList(c *gin.Context) {
	configs, err := models.AIOptimizationConfigGets(rt.Ctx, "enabled = 1")
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 按类型分组返回
	result := make(map[string]interface{})
	for _, cfg := range configs {
		var value interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigValue), &value); err != nil {
			value = cfg.ConfigValue
		}
		result[cfg.ConfigType] = gin.H{
			"id":          cfg.Id,
			"config_key":  cfg.ConfigKey,
			"value":       value,
			"description": cfg.Description,
			"enabled":     cfg.Enabled,
			"update_at":   cfg.UpdateAt,
			"update_by":   cfg.UpdateBy,
		}
	}

	ginx.NewRender(c).Data(result, nil)
}

// optimizationConfigUpdate 更新指定类型的优化配置
func (rt *Router) optimizationConfigUpdate(c *gin.Context) {
	configType := c.Param("type")

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

	var req struct {
		ConfigValue string `json:"config_value"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证 JSON 格式
	var jsonTest interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue), &jsonTest); err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid JSON format: %v", err))
		return
	}

	// 根据类型验证配置结构
	if err := rt.validateOptimizationConfig(configType, req.ConfigValue); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	username := c.GetString("username")

	// 更新配置
	err := models.AIOptimizationConfigUpsert(rt.Ctx, configType, models.OptConfigKeyDefault, req.ConfigValue, req.Description, username)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":     "config updated successfully",
		"config_type": configType,
		"timestamp":   time.Now().Unix(),
	}, nil)
}

// validateOptimizationConfig 验证优化配置结构
func (rt *Router) validateOptimizationConfig(configType, configValue string) error {
	switch configType {
	case models.OptConfigTypeRateLimit:
		var cfg models.RateLimitConfig
		return json.Unmarshal([]byte(configValue), &cfg)
	case models.OptConfigTypeCache:
		var cfg models.CacheConfig
		return json.Unmarshal([]byte(configValue), &cfg)
	case models.OptConfigTypeModelRouter:
		var cfg models.ModelRouterConfig
		return json.Unmarshal([]byte(configValue), &cfg)
	case models.OptConfigTypeRetry:
		var cfg models.RetryConfig
		return json.Unmarshal([]byte(configValue), &cfg)
	case models.OptConfigTypeCost:
		var cfg models.CostConfig
		return json.Unmarshal([]byte(configValue), &cfg)
	case models.OptConfigTypeConcurrent:
		var cfg models.ConcurrentConfig
		return json.Unmarshal([]byte(configValue), &cfg)
	default:
		return fmt.Errorf("unknown config type: %s", configType)
	}
}

// optimizationReload 重新加载所有优化配置
func (rt *Router) optimizationReload(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	if aiOptManager == nil {
		ginx.NewRender(c).Data(gin.H{
			"message":   "optimization manager not initialized",
			"timestamp": time.Now().Unix(),
		}, nil)
		return
	}

	// 重新加载配置
	err := aiOptManager.Reload()
	if err != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("failed to reload optimization config: %v", err))
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":   "optimization config reloaded successfully",
		"timestamp": time.Now().Unix(),
	}, nil)
}

// optimizationStats 获取优化统计信息
func (rt *Router) optimizationStats(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	if aiOptManager == nil {
		ginx.NewRender(c).Data(gin.H{
			"message": "optimization manager not initialized",
			"stats":   nil,
		}, nil)
		return
	}

	stats := aiOptManager.GetStats(c.Request.Context())
	ginx.NewRender(c).Data(stats, nil)
}

// ===== 成本统计查询 =====

// costDailyStats 获取每日成本统计
func (rt *Router) costDailyStats(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	// 获取日期参数，默认今天
	date := c.Query("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 支持日期范围查询
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if aiOptManager == nil || aiOptManager.CostTracker == nil {
		ginx.NewRender(c).Data(gin.H{
			"message": "cost tracker not initialized",
			"date":    date,
		}, nil)
		return
	}

	// 如果指定了日期范围
	if startDate != "" && endDate != "" {
		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("invalid start_date format: %v", err))
			return
		}
		end, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("invalid end_date format: %v", err))
			return
		}

		// 获取日期范围内的成本
		var results []interface{}
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			dateStr := d.Format("2006-01-02")
			dailyCost, err := aiOptManager.CostTracker.GetDailyCost(c.Request.Context(), dateStr)
			if err != nil {
				continue
			}
			results = append(results, dailyCost)
		}

		ginx.NewRender(c).Data(gin.H{
			"start_date": startDate,
			"end_date":   endDate,
			"data":       results,
		}, nil)
		return
	}

	// 单日查询
	dailyCost, err := aiOptManager.CostTracker.GetDailyCost(c.Request.Context(), date)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(dailyCost, nil)
}

// costUserStats 获取用户成本统计
func (rt *Router) costUserStats(c *gin.Context) {
	// 延迟初始化
	rt.initAIAssistant()

	userID := c.Param("user_id")
	if userID == "" {
		ginx.NewRender(c).Data(nil, fmt.Errorf("user_id is required"))
		return
	}

	// 获取日期参数
	date := c.Query("date")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if aiOptManager == nil || aiOptManager.CostTracker == nil {
		ginx.NewRender(c).Data(gin.H{
			"message": "cost tracker not initialized",
			"user_id": userID,
		}, nil)
		return
	}

	// 如果指定了日期范围
	if startDate != "" && endDate != "" {
		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("invalid start_date format: %v", err))
			return
		}
		end, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("invalid end_date format: %v", err))
			return
		}

		totalCost, totalCalls, err := aiOptManager.CostTracker.GetUserCostRange(c.Request.Context(), userID, start, end)
		if err != nil {
			ginx.NewRender(c).Data(nil, err)
			return
		}

		ginx.NewRender(c).Data(gin.H{
			"user_id":     userID,
			"start_date":  startDate,
			"end_date":    endDate,
			"total_cost":  totalCost,
			"total_calls": totalCalls,
		}, nil)
		return
	}

	// 单日查询
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	cost, calls, err := aiOptManager.CostTracker.GetUserCost(c.Request.Context(), userID, date)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"user_id":     userID,
		"date":        date,
		"total_cost":  cost,
		"total_calls": calls,
	}, nil)
}

// ===== 动态 Agent 管理 =====

// aiAgentList 获取 Agent 列表（用于 @Mention 联想）
func (rt *Router) aiAgentList(c *gin.Context) {
	// 查询参数
	agentType := c.Query("type")
	enabledOnly := c.Query("enabled") != "false" // 默认只返回启用的

	var agents []models.AIAgent
	var err error

	if agentType != "" {
		agents, err = models.AIAgentGetByType(rt.Ctx, agentType)
	} else if enabledOnly {
		agents, err = models.AIAgentGetEnabled(rt.Ctx)
	} else {
		agents, err = models.AIAgentGets(rt.Ctx, "")
	}

	ginx.NewRender(c).Data(agents, err)
}

// aiAgentGet 获取单个 Agent
func (rt *Router) aiAgentGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	agent, err := models.AIAgentGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if agent == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("agent not found"))
		return
	}

	// 获取绑定的工具
	tools, _ := agent.GetTools(rt.Ctx)

	ginx.NewRender(c).Data(gin.H{
		"agent": agent,
		"tools": tools,
	}, nil)
}

// aiAgentCreate 创建 Agent
func (rt *Router) aiAgentCreate(c *gin.Context) {
	var agent models.AIAgent
	if err := c.ShouldBindJSON(&agent); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证名称唯一性
	existing, _ := models.AIAgentGetByName(rt.Ctx, agent.Name)
	if existing != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("agent name already exists: %s", agent.Name))
		return
	}

	agent.CreateBy = c.GetString("username")
	agent.UpdateBy = c.GetString("username")

	err := agent.Create(rt.Ctx)
	ginx.NewRender(c).Data(agent.Id, err)
}

// aiAgentUpdate 更新 Agent
func (rt *Router) aiAgentUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	agent, err := models.AIAgentGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if agent == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("agent not found"))
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 如果更新名称，检查唯一性
	if name, ok := updates["name"].(string); ok && name != agent.Name {
		existing, _ := models.AIAgentGetByName(rt.Ctx, name)
		if existing != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("agent name already exists: %s", name))
			return
		}
	}

	updates["update_by"] = c.GetString("username")
	err = agent.Update(rt.Ctx, updates)
	ginx.NewRender(c).Data("ok", err)
}

// aiAgentDelete 删除 Agent
func (rt *Router) aiAgentDelete(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	agent, err := models.AIAgentGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if agent == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("agent not found"))
		return
	}

	// 不允许删除系统级 Agent
	if agent.AgentType == models.AIAgentTypeSystem {
		ginx.NewRender(c).Data(nil, fmt.Errorf("cannot delete system agent"))
		return
	}

	err = agent.Delete(rt.Ctx)
	ginx.NewRender(c).Data("ok", err)
}

// aiAgentBindTools 绑定工具到 Agent
func (rt *Router) aiAgentBindTools(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	agent, err := models.AIAgentGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if agent == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("agent not found"))
		return
	}

	var req struct {
		ToolIds []int64 `json:"tool_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	err = agent.BindTools(rt.Ctx, req.ToolIds)
	ginx.NewRender(c).Data("ok", err)
}

// ===== 动态 Tool 管理 =====

// aiToolList 获取 Tool 列表
func (rt *Router) aiToolList(c *gin.Context) {
	// 查询参数
	implType := c.Query("type")
	enabledOnly := c.Query("enabled") != "false" // 默认只返回启用的

	var tools []models.AITool
	var err error

	if implType != "" {
		tools, err = models.AIToolGetByType(rt.Ctx, implType)
	} else if enabledOnly {
		tools, err = models.AIToolGetEnabled(rt.Ctx)
	} else {
		tools, err = models.AIToolGets(rt.Ctx, "")
	}

	ginx.NewRender(c).Data(tools, err)
}

// aiToolGet 获取单个 Tool
func (rt *Router) aiToolGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	tool, err := models.AIToolGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if tool == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("tool not found"))
		return
	}

	ginx.NewRender(c).Data(tool, nil)
}

// aiToolCreate 创建 Tool
func (rt *Router) aiToolCreate(c *gin.Context) {
	var tool models.AITool
	if err := c.ShouldBindJSON(&tool); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 验证名称唯一性
	existing, _ := models.AIToolGetByName(rt.Ctx, tool.Name)
	if existing != nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("tool name already exists: %s", tool.Name))
		return
	}

	// 验证 implementation_type
	validTypes := map[string]bool{
		models.AIToolTypeNative:    true,
		models.AIToolTypeAPI:       true,
		models.AIToolTypeMCP:       true,
		models.AIToolTypeKnowledge: true,
	}
	if !validTypes[tool.ImplementationType] {
		ginx.NewRender(c).Data(nil, fmt.Errorf("invalid implementation_type: %s", tool.ImplementationType))
		return
	}

	tool.CreateBy = c.GetString("username")
	tool.UpdateBy = c.GetString("username")

	err := tool.Create(rt.Ctx)
	ginx.NewRender(c).Data(tool.Id, err)
}

// aiToolUpdate 更新 Tool
func (rt *Router) aiToolUpdate(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	tool, err := models.AIToolGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if tool == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("tool not found"))
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 如果更新名称，检查唯一性
	if name, ok := updates["name"].(string); ok && name != tool.Name {
		existing, _ := models.AIToolGetByName(rt.Ctx, name)
		if existing != nil {
			ginx.NewRender(c).Data(nil, fmt.Errorf("tool name already exists: %s", name))
			return
		}
	}

	updates["update_by"] = c.GetString("username")
	err = tool.Update(rt.Ctx, updates)
	ginx.NewRender(c).Data("ok", err)
}

// aiToolDelete 删除 Tool
func (rt *Router) aiToolDelete(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	tool, err := models.AIToolGetById(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}
	if tool == nil {
		ginx.NewRender(c).Data(nil, fmt.Errorf("tool not found"))
		return
	}

	err = tool.Delete(rt.Ctx)
	ginx.NewRender(c).Data("ok", err)
}
