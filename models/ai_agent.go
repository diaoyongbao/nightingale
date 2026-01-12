// Package models AI Agent 模型
// n9e-2kai: AI 助手模块 - 动态 Agent 定义
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// AIAgent AI Agent 定义表
type AIAgent struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Description string `json:"description" gorm:"type:varchar(500)"`

	// System Prompt
	SystemPrompt string `json:"system_prompt" gorm:"type:text;not null"`

	// 模型配置 (JSON)
	ModelConfig string `json:"model_config" gorm:"type:text"` // {"model": "gpt-4", "temperature": 0.5, "max_tokens": 4096}

	// 路由配置
	Keywords string `json:"keywords" gorm:"type:text"` // JSON 数组: ["k8s", "pod", "容器"]
	Priority int    `json:"priority" gorm:"default:0"` // 路由优先级，数值越大优先级越高

	// Agent 类型
	AgentType string `json:"agent_type" gorm:"type:varchar(32);default:'expert'"` // system/expert/knowledge

	// 状态
	Enabled bool `json:"enabled" gorm:"default:true"`

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (AIAgent) TableName() string {
	return "ai_agent"
}

// Agent 类型常量
const (
	AIAgentTypeSystem    = "system"    // 系统级 Agent (router, summary, knowledge)
	AIAgentTypeExpert    = "expert"    // 专家 Agent (k8s, database, alert)
	AIAgentTypeKnowledge = "knowledge" // 知识库 Agent
)

// 系统级 Agent 名称常量
const (
	AIAgentNameRouter    = "sys_router"    // 路由决策 Agent
	AIAgentNameSummary   = "sys_summary"   // 结果汇总 Agent
	AIAgentNameKnowledge = "sys_knowledge" // 知识库 Agent
	AIAgentNameGeneral   = "sys_general"   // 通用 Agent
)

// ModelConfig 模型配置结构
type AgentModelConfig struct {
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// GetModelConfig 解析模型配置
func (a *AIAgent) GetModelConfig() (*AgentModelConfig, error) {
	if a.ModelConfig == "" {
		return nil, nil
	}
	var config AgentModelConfig
	if err := json.Unmarshal([]byte(a.ModelConfig), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetKeywords 解析关键词列表
func (a *AIAgent) GetKeywords() ([]string, error) {
	if a.Keywords == "" {
		return nil, nil
	}
	var keywords []string
	if err := json.Unmarshal([]byte(a.Keywords), &keywords); err != nil {
		return nil, err
	}
	return keywords, nil
}

// SetKeywords 设置关键词列表
func (a *AIAgent) SetKeywords(keywords []string) error {
	data, err := json.Marshal(keywords)
	if err != nil {
		return err
	}
	a.Keywords = string(data)
	return nil
}

// SetModelConfig 设置模型配置
func (a *AIAgent) SetModelConfig(config *AgentModelConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	a.ModelConfig = string(data)
	return nil
}

// CRUD 方法

// AIAgentGets 查询 Agent 列表
func AIAgentGets(c *ctx.Context, where string, args ...interface{}) ([]AIAgent, error) {
	var agents []AIAgent
	session := DB(c).Order("priority desc, name asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&agents).Error
	return agents, err
}

// AIAgentGetEnabled 查询所有启用的 Agent
func AIAgentGetEnabled(c *ctx.Context) ([]AIAgent, error) {
	return AIAgentGets(c, "enabled = ?", true)
}

// AIAgentGetByType 根据类型查询 Agent
func AIAgentGetByType(c *ctx.Context, agentType string) ([]AIAgent, error) {
	return AIAgentGets(c, "agent_type = ? AND enabled = ?", agentType, true)
}

// AIAgentGetById 根据 ID 查询
func AIAgentGetById(c *ctx.Context, id int64) (*AIAgent, error) {
	var agent AIAgent
	err := DB(c).Where("id = ?", id).First(&agent).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &agent, err
}

// AIAgentGetByName 根据名称查询
func AIAgentGetByName(c *ctx.Context, name string) (*AIAgent, error) {
	var agent AIAgent
	err := DB(c).Where("name = ?", name).First(&agent).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &agent, err
}

// Create 创建 Agent
func (a *AIAgent) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	a.CreateAt = now
	a.UpdateAt = now
	return DB(c).Create(a).Error
}

// Update 更新 Agent
func (a *AIAgent) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(a).Updates(updates).Error
}

// Delete 删除 Agent
func (a *AIAgent) Delete(c *ctx.Context) error {
	// 删除关联的工具绑定
	if err := DB(c).Where("agent_id = ?", a.Id).Delete(&AIAgentToolRel{}).Error; err != nil {
		return err
	}
	return DB(c).Delete(a).Error
}

// GetTools 获取 Agent 绑定的工具
func (a *AIAgent) GetTools(c *ctx.Context) ([]AITool, error) {
	var tools []AITool
	err := DB(c).Table("ai_tool").
		Joins("JOIN ai_agent_tool_rel ON ai_tool.id = ai_agent_tool_rel.tool_id").
		Where("ai_agent_tool_rel.agent_id = ? AND ai_tool.enabled = ?", a.Id, true).
		Find(&tools).Error
	return tools, err
}

// BindTools 绑定工具
func (a *AIAgent) BindTools(c *ctx.Context, toolIds []int64) error {
	// 先删除现有绑定
	if err := DB(c).Where("agent_id = ?", a.Id).Delete(&AIAgentToolRel{}).Error; err != nil {
		return err
	}

	// 创建新绑定
	for _, toolId := range toolIds {
		rel := &AIAgentToolRel{
			AgentId: a.Id,
			ToolId:  toolId,
		}
		if err := DB(c).Create(rel).Error; err != nil {
			return err
		}
	}
	return nil
}

// InitAIAgentSeed 初始化系统级 Agent 种子数据
func InitAIAgentSeed(c *ctx.Context) error {
	now := time.Now().Unix()

	// 定义系统级 Agent 种子数据
	seedAgents := []AIAgent{
		{
			Name:        AIAgentNameGeneral,
			Description: "通用运维助手，处理一般性问题和闲聊",
			SystemPrompt: `你是夜莺监控系统的运维助手。

## 核心职责
- 回答系统使用方法
- 提供配置说明
- 给出最佳实践建议

## 回答原则
- 如果知识库返回了相关结果，基于结果回答
- 如果知识库没有相关信息，用你的知识直接回答用户问题
- 禁止编造不存在的信息
- 保持专业、友好的语气`,
			ModelConfig: `{"model": "gpt-4o", "temperature": 0.7, "max_tokens": 4096}`,
			Keywords:    `[]`,
			Priority:    0,
			AgentType:   AIAgentTypeSystem,
			Enabled:     true,
			CreateAt:    now,
			CreateBy:    "system",
			UpdateAt:    now,
			UpdateBy:    "system",
		},
		{
			Name:        AIAgentNameKnowledge,
			Description: "知识库专家，回答关于系统使用说明、运维文档、故障排查手册、API 文档等静态知识的问题",
			SystemPrompt: `你是知识库助手，专门从运维知识库中检索信息来回答用户问题。

## 职责
- 使用知识库工具搜索相关文档
- 基于搜索结果回答用户问题
- 标注信息来源

## 工具使用原则
当调用知识库工具时，query 参数必须：
- 只包含用户问题中的核心关键词（通常 1-3 个词）
- 不要添加任何额外词汇
- 使用用户原话中的关键词

## 回答原则
- 优先使用知识库检索结果
- 如果未找到相关信息，明确告知用户
- 不要编造不存在的信息`,
			ModelConfig: `{"model": "gpt-4o", "temperature": 0.3, "max_tokens": 4096}`,
			Keywords:    `["文档", "手册", "说明", "指南", "教程", "怎么", "如何", "是什么"]`,
			Priority:    100,
			AgentType:   AIAgentTypeSystem,
			Enabled:     true,
			CreateAt:    now,
			CreateBy:    "system",
			UpdateAt:    now,
			UpdateBy:    "system",
		},
		{
			Name:        AIAgentNameRouter,
			Description: "路由决策专家，负责分析用户意图并选择合适的 Agent",
			SystemPrompt: `你是一个任务分发员。请根据用户问题，从候选专家中选择最合适的一个。

## 决策规则
1. 如果问题是询问信息、文档、配置方法，选择 "sys_knowledge"
2. 如果问题涉及 K8s/容器/Pod，选择 "k8s_expert"（如果存在）
3. 如果问题涉及数据库/SQL，选择 "db_expert"（如果存在）
4. 如果问题涉及告警/屏蔽，选择 "alert_expert"（如果存在）
5. 如果是闲聊或通用问题，选择 "sys_general"

## 输出格式
只返回 Agent 名称，不要其他内容。`,
			ModelConfig: `{"model": "gpt-4o-mini", "temperature": 0.1, "max_tokens": 100}`,
			Keywords:    `[]`,
			Priority:    1000,
			AgentType:   AIAgentTypeSystem,
			Enabled:     true,
			CreateAt:    now,
			CreateBy:    "system",
			UpdateAt:    now,
			UpdateBy:    "system",
		},
		{
			Name:        AIAgentNameSummary,
			Description: "结果汇总专家，负责整合工具执行结果并生成用户友好的回答",
			SystemPrompt: `你是结果汇总助手。根据工具返回的结果，为用户生成清晰、有条理的回答。

## 汇总原则
- 提取关键信息
- 使用 Markdown 格式化输出
- 如果工具执行失败，解释原因并提供替代建议
- 保持简洁，避免冗余`,
			ModelConfig: `{"model": "gpt-4o", "temperature": 0.5, "max_tokens": 4096}`,
			Keywords:    `[]`,
			Priority:    999,
			AgentType:   AIAgentTypeSystem,
			Enabled:     true,
			CreateAt:    now,
			CreateBy:    "system",
			UpdateAt:    now,
			UpdateBy:    "system",
		},
	}

	// 插入种子数据（忽略已存在的记录）
	for _, agent := range seedAgents {
		existing, err := AIAgentGetByName(c, agent.Name)
		if err != nil {
			return err
		}
		if existing == nil {
			if err := agent.Create(c); err != nil {
				return err
			}
		}
	}

	return nil
}
