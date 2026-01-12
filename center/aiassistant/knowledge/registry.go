// Package knowledge Provider 注册机制和工具注册表
// n9e-2kai: AI 助手模块 - Provider 和工具注册（Function Calling 架构）
package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// ============================================
// Provider 注册表
// ============================================

var providerRegistry = make(map[string]Provider)
var providerMu sync.RWMutex

// RegisterProvider 注册 Provider
func RegisterProvider(name string, provider Provider) {
	providerMu.Lock()
	defer providerMu.Unlock()
	providerRegistry[name] = provider
}

// GetProvider 获取 Provider
func GetProvider(name string) (Provider, error) {
	providerMu.RLock()
	defer providerMu.RUnlock()
	provider, exists := providerRegistry[name]
	if !exists {
		return nil, fmt.Errorf("knowledge provider not found: %s", name)
	}
	return provider, nil
}

// ListProviders 列出所有 Provider
func ListProviders() []string {
	providerMu.RLock()
	defer providerMu.RUnlock()
	names := make([]string, 0, len(providerRegistry))
	for name := range providerRegistry {
		names = append(names, name)
	}
	return names
}

// UnregisterProvider 注销 Provider
func UnregisterProvider(name string) {
	providerMu.Lock()
	defer providerMu.Unlock()
	delete(providerRegistry, name)
}

// ClearProviders 清空所有 Provider
func ClearProviders() {
	providerMu.Lock()
	defer providerMu.Unlock()
	providerRegistry = make(map[string]Provider)
}

// ============================================
// 知识库工具注册表（Function Calling）
// ============================================

// KnowledgeToolRegistry 知识库工具注册表
type KnowledgeToolRegistry struct {
	tools     map[string]*RegisteredTool
	providers map[int64]Provider
	mu        sync.RWMutex
	c         *ctx.Context
}

// RegisteredTool 已注册的工具
type RegisteredTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ProviderID  int64  `json:"provider_id"`
	Parameters  *models.ToolParameters
	Keywords    []string
	Enabled     bool
	Priority    int
}

// ToolDefinition OpenAI Function Calling 工具定义
type ToolDefinition struct {
	Type     string           `json:"type"`
	Function FunctionDef      `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// NewKnowledgeToolRegistry 创建工具注册表
func NewKnowledgeToolRegistry(c *ctx.Context) *KnowledgeToolRegistry {
	return &KnowledgeToolRegistry{
		tools:     make(map[string]*RegisteredTool),
		providers: make(map[int64]Provider),
		c:         c,
	}
}

// LoadFromConfig 从数据库加载工具配置
func (r *KnowledgeToolRegistry) LoadFromConfig() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 清空现有配置
	r.tools = make(map[string]*RegisteredTool)
	r.providers = make(map[int64]Provider)

	// 加载所有启用的 Provider
	providers, err := models.KnowledgeProviderGetEnabled(r.c)
	if err != nil {
		return fmt.Errorf("failed to load providers: %w", err)
	}

	// 记录已加载的 Provider 信息，用于自动创建默认工具
	providerInfos := make(map[int64]*models.KnowledgeProvider)

	for _, p := range providers {
		provider, err := r.createProvider(&p)
		if err != nil {
			logger.Errorf("failed to create provider %s: %v", p.Name, err)
			continue
		}
		r.providers[p.Id] = provider
		providerInfos[p.Id] = &p
		// 同时注册到全局 Provider 注册表
		RegisterProvider(p.Name, provider)
		logger.Infof("loaded knowledge provider: %s (type=%s)", p.Name, p.ProviderType)
	}

	// 加载所有启用的工具
	tools, err := models.KnowledgeToolGetEnabled(r.c)
	if err != nil {
		return fmt.Errorf("failed to load tools: %w", err)
	}

	// 记录哪些 Provider 已有工具
	providersWithTools := make(map[int64]bool)

	for _, t := range tools {
		// 检查 Provider 是否存在
		if _, exists := r.providers[t.ProviderID]; !exists {
			logger.Warningf("tool %s references non-existent provider %d, skipped", t.Name, t.ProviderID)
			continue
		}

		keywords, _ := t.GetKeywords()
		params, _ := t.GetParameters()

		r.tools[t.Name] = &RegisteredTool{
			Name:        t.Name,
			Description: t.Description,
			ProviderID:  t.ProviderID,
			Parameters:  params,
			Keywords:    keywords,
			Enabled:     t.Enabled,
			Priority:    t.Priority,
		}
		providersWithTools[t.ProviderID] = true
		logger.Infof("loaded knowledge tool: %s (provider_id=%d)", t.Name, t.ProviderID)
	}

	// 为没有配置工具的 Provider 自动创建默认工具
	for providerID, providerInfo := range providerInfos {
		if providersWithTools[providerID] {
			continue // 已有工具，跳过
		}

		// 自动创建默认工具
		defaultToolName := fmt.Sprintf("search_%s", providerInfo.Name)
		defaultDescription := r.getDefaultToolDescription(providerInfo)

		r.tools[defaultToolName] = &RegisteredTool{
			Name:        defaultToolName,
			Description: defaultDescription,
			ProviderID:  providerID,
			Parameters:  &models.ToolParameters{},
			Keywords:    []string{},
			Enabled:     true,
			Priority:    0,
		}
		logger.Infof("auto-created default tool: %s for provider %s", defaultToolName, providerInfo.Name)
	}

	return nil
}

// getDefaultToolDescription 根据 Provider 类型生成默认工具描述
func (r *KnowledgeToolRegistry) getDefaultToolDescription(p *models.KnowledgeProvider) string {
	if p.Description != "" {
		return p.Description
	}

	switch p.ProviderType {
	case models.ProviderTypeCloudflareAutoRAG:
		return `搜索内部知识库获取相关信息。
重要：query 参数只填写用户问题中的核心关键词（1-3个词），不要添加任何额外词汇。`
	case models.ProviderTypeCoze:
		return `搜索知识库获取文档信息。query 参数只填写核心关键词。`
	default:
		return "搜索知识库获取相关信息"
	}
}

// createProvider 根据配置创建 Provider 实例
func (r *KnowledgeToolRegistry) createProvider(p *models.KnowledgeProvider) (Provider, error) {
	switch p.ProviderType {
	case models.ProviderTypeCloudflareAutoRAG:
		modelConfig, err := p.GetCloudflareConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to parse cloudflare config: %w", err)
		}
		// 转换为 knowledge 包的配置类型
		config := &CloudflareRAGConfig{
			AccountID:      modelConfig.AccountID,
			RAGName:        modelConfig.RAGName,
			APIToken:       modelConfig.APIToken,
			Model:          modelConfig.Model,
			RewriteQuery:   modelConfig.RewriteQuery,
			MaxNumResults:  modelConfig.MaxNumResults,
			ScoreThreshold: modelConfig.ScoreThreshold,
			Timeout:        modelConfig.Timeout,
		}
		return NewCloudflareRAGProvider(p.Name, config), nil

	// 可扩展其他 Provider 类型
	// case models.ProviderTypeCoze:
	//     return NewCozeProvider(p.Name, config), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", p.ProviderType)
	}
}

// GetToolDefinitions 获取所有工具定义（OpenAI Function Calling 格式）
func (r *KnowledgeToolRegistry) GetToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		if !tool.Enabled {
			continue
		}
		definitions = append(definitions, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "搜索查询关键词，从用户问题中提取核心内容",
						},
					},
					"required": []string{"query"},
				},
			},
		})
	}
	return definitions
}

// IsKnowledgeTool 判断是否为知识库工具
func (r *KnowledgeToolRegistry) IsKnowledgeTool(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[toolName]
	return exists
}

// ExecuteTool 执行知识库查询
func (r *KnowledgeToolRegistry) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*QueryResponse, error) {
	r.mu.RLock()
	tool, exists := r.tools[toolName]
	if !exists {
		r.mu.RUnlock()
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	provider, exists := r.providers[tool.ProviderID]
	if !exists {
		r.mu.RUnlock()
		return nil, fmt.Errorf("provider not found for tool: %s", toolName)
	}
	r.mu.RUnlock()

	// 构建查询请求
	query, _ := args["query"].(string)
	conversationID, _ := args["conversation_id"].(string)
	req := &QueryRequest{
		Query:          query,
		ConversationID: conversationID,
	}

	// 应用工具参数
	if tool.Parameters != nil {
		if tool.Parameters.MaxResults > 0 {
			req.MaxResults = tool.Parameters.MaxResults
		}
		if tool.Parameters.ScoreThreshold > 0 {
			req.ScoreThreshold = tool.Parameters.ScoreThreshold
		}
	}

	// 执行查询
	return provider.Query(ctx, req)
}

// RegisterTool 动态注册工具
func (r *KnowledgeToolRegistry) RegisterTool(tool *RegisteredTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// UnregisterTool 注销工具
func (r *KnowledgeToolRegistry) UnregisterTool(toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, toolName)
}

// GetToolCount 获取工具数量
func (r *KnowledgeToolRegistry) GetToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, tool := range r.tools {
		if tool.Enabled {
			count++
		}
	}
	return count
}

// GetProviderCount 获取 Provider 数量
func (r *KnowledgeToolRegistry) GetProviderCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// FormatResultsForLLM 格式化查询结果供 LLM 使用
func FormatResultsForLLM(resp *QueryResponse) string {
	if resp.Status != "completed" {
		return fmt.Sprintf("查询失败: %s", resp.Error)
	}

	if len(resp.Results) == 0 {
		if resp.Answer != "" {
			return resp.Answer
		}
		return "未找到相关信息"
	}

	// 如果有汇总答案，优先使用
	if resp.Answer != "" {
		return resp.Answer
	}

	// 否则拼接结果
	var result string
	for i, r := range resp.Results {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += r.Content
		if r.Source != "" {
			result += fmt.Sprintf("\n(来源: %s)", r.Source)
		}
	}
	return result
}

// ToJSON 将工具定义转为 JSON
func (d *ToolDefinition) ToJSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
