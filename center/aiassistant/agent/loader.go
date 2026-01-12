// Package agent Agent 加载器
// n9e-2kai: AI 助手模块 - 从数据库动态加载 Agent 配置
package agent

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// AgentConfig Agent 运行时配置
type AgentConfig struct {
	ID           int64
	Name         string
	Description  string
	SystemPrompt string
	AgentType    string
	Keywords     []string
	Priority     int
	Enabled      bool

	// 模型配置
	Model       string
	Temperature float64
	MaxTokens   int

	// 绑定的工具
	Tools []*ToolConfig
}

// ToolConfig 工具运行时配置
type ToolConfig struct {
	ID                 int64
	Name               string
	Description        string
	ImplementationType string
	RiskLevel          string

	// API 配置
	Method          string
	URLPath         string
	ParameterSchema map[string]any

	// MCP 配置
	MCPServerID int64
	MCPToolName string

	// Native 配置
	NativeHandler string

	// Knowledge 配置
	KnowledgeProviderID int64
}

// Loader Agent 加载器
type Loader struct {
	ctx    *ctx.Context
	cache  map[string]*AgentConfig // name -> config
	mu     sync.RWMutex
	loaded bool
}

// NewLoader 创建 Agent 加载器
func NewLoader(c *ctx.Context) *Loader {
	return &Loader{
		ctx:   c,
		cache: make(map[string]*AgentConfig),
	}
}

// Load 从数据库加载所有启用的 Agent
func (l *Loader) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 加载所有启用的 Agent
	agents, err := models.AIAgentGetEnabled(l.ctx)
	if err != nil {
		return err
	}

	// 清空缓存
	l.cache = make(map[string]*AgentConfig)

	for _, agent := range agents {
		config, err := l.buildAgentConfig(&agent)
		if err != nil {
			logger.Warningf("Failed to build agent config for %s: %v", agent.Name, err)
			continue
		}
		l.cache[agent.Name] = config
	}

	l.loaded = true
	logger.Infof("Loaded %d agents from database", len(l.cache))
	return nil
}

// buildAgentConfig 构建 Agent 运行时配置
func (l *Loader) buildAgentConfig(agent *models.AIAgent) (*AgentConfig, error) {
	config := &AgentConfig{
		ID:           agent.Id,
		Name:         agent.Name,
		Description:  agent.Description,
		SystemPrompt: agent.SystemPrompt,
		AgentType:    agent.AgentType,
		Priority:     agent.Priority,
		Enabled:      agent.Enabled,
	}

	// 解析模型配置
	if agent.ModelConfig != "" {
		var modelConfig models.AgentModelConfig
		if err := json.Unmarshal([]byte(agent.ModelConfig), &modelConfig); err == nil {
			config.Model = modelConfig.Model
			config.Temperature = modelConfig.Temperature
			config.MaxTokens = modelConfig.MaxTokens
		}
	}

	// 解析关键词
	if agent.Keywords != "" {
		var keywords []string
		if err := json.Unmarshal([]byte(agent.Keywords), &keywords); err == nil {
			config.Keywords = keywords
		}
	}

	// 加载绑定的工具
	tools, err := agent.GetTools(l.ctx)
	if err != nil {
		logger.Warningf("Failed to load tools for agent %s: %v", agent.Name, err)
	} else {
		for _, tool := range tools {
			toolConfig := l.buildToolConfig(&tool)
			config.Tools = append(config.Tools, toolConfig)
		}
	}

	return config, nil
}

// buildToolConfig 构建工具运行时配置
func (l *Loader) buildToolConfig(tool *models.AITool) *ToolConfig {
	config := &ToolConfig{
		ID:                  tool.Id,
		Name:                tool.Name,
		Description:         tool.Description,
		ImplementationType:  tool.ImplementationType,
		RiskLevel:           tool.RiskLevel,
		Method:              tool.Method,
		URLPath:             tool.URLPath,
		MCPServerID:         tool.MCPServerID,
		MCPToolName:         tool.MCPToolName,
		NativeHandler:       tool.NativeHandler,
		KnowledgeProviderID: tool.KnowledgeProviderID,
	}

	// 解析参数 Schema
	if tool.ParameterSchema != "" {
		var schema map[string]any
		if err := json.Unmarshal([]byte(tool.ParameterSchema), &schema); err == nil {
			config.ParameterSchema = schema
		}
	}

	return config
}

// GetAgent 获取 Agent 配置
func (l *Loader) GetAgent(name string) *AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cache[name]
}

// GetAgentByID 根据 ID 获取 Agent 配置
func (l *Loader) GetAgentByID(id int64) *AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, agent := range l.cache {
		if agent.ID == id {
			return agent
		}
	}
	return nil
}

// GetAllAgents 获取所有 Agent 配置
func (l *Loader) GetAllAgents() []*AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	agents := make([]*AgentConfig, 0, len(l.cache))
	for _, agent := range l.cache {
		agents = append(agents, agent)
	}
	return agents
}

// GetAgentsByType 根据类型获取 Agent
func (l *Loader) GetAgentsByType(agentType string) []*AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var agents []*AgentConfig
	for _, agent := range l.cache {
		if agent.AgentType == agentType {
			agents = append(agents, agent)
		}
	}
	return agents
}

// GetExpertAgents 获取所有专家 Agent
func (l *Loader) GetExpertAgents() []*AgentConfig {
	return l.GetAgentsByType(models.AIAgentTypeExpert)
}

// GetSystemAgent 获取系统 Agent
func (l *Loader) GetSystemAgent(name string) *AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	agent := l.cache[name]
	if agent != nil && agent.AgentType == models.AIAgentTypeSystem {
		return agent
	}
	return nil
}

// MatchAgentByKeywords 根据关键词匹配 Agent
func (l *Loader) MatchAgentByKeywords(message string) *AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	lowerMsg := strings.ToLower(message)
	var matched *AgentConfig
	maxPriority := -1

	for _, agent := range l.cache {
		if !agent.Enabled || len(agent.Keywords) == 0 {
			continue
		}

		for _, keyword := range agent.Keywords {
			if strings.Contains(lowerMsg, strings.ToLower(keyword)) {
				if agent.Priority > maxPriority {
					maxPriority = agent.Priority
					matched = agent
				}
				break
			}
		}
	}

	return matched
}

// Reload 重新加载 Agent 配置
func (l *Loader) Reload() error {
	return l.Load()
}

// IsLoaded 检查是否已加载
func (l *Loader) IsLoaded() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loaded
}

// GetAgentCount 获取 Agent 数量
func (l *Loader) GetAgentCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.cache)
}

// GetAgentNames 获取所有 Agent 名称（用于 @Mention）
func (l *Loader) GetAgentNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.cache))
	for name := range l.cache {
		names = append(names, name)
	}
	return names
}

// GetAgentsForMention 获取可被 @Mention 的 Agent 列表
func (l *Loader) GetAgentsForMention() []*AgentConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var agents []*AgentConfig
	for _, agent := range l.cache {
		// 系统级路由和汇总 Agent 不暴露给用户
		if agent.Name == models.AIAgentNameRouter || agent.Name == models.AIAgentNameSummary {
			continue
		}
		agents = append(agents, agent)
	}
	return agents
}
