// Package models AI Tool 模型
// n9e-2kai: AI 助手模块 - 动态工具定义
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// AITool AI 工具定义表
type AITool struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Description string `json:"description" gorm:"type:varchar(500);not null"` // 给 LLM 看的工具描述

	// 工具类型
	ImplementationType string `json:"implementation_type" gorm:"type:varchar(32);not null"` // native/api/mcp/knowledge

	// API 映射配置 (当 implementation_type='api' 时)
	Method          string `json:"method" gorm:"type:varchar(10)"`    // GET/POST/PUT/DELETE
	URLPath         string `json:"url_path" gorm:"type:varchar(256)"` // 例如: "/api/n9e/alert-mutes"
	ParameterSchema string `json:"parameter_schema" gorm:"type:text"` // JSON Schema，定义 LLM 需要提取的参数
	ResponseMapping string `json:"response_mapping" gorm:"type:text"` // 响应字段映射配置

	// MCP 配置 (当 implementation_type='mcp' 时)
	MCPServerID int64  `json:"mcp_server_id" gorm:"default:0"`         // 关联到 mcp_server 表
	MCPToolName string `json:"mcp_tool_name" gorm:"type:varchar(128)"` // MCP Server 暴露的工具名

	// Native 配置 (当 implementation_type='native' 时)
	NativeHandler string `json:"native_handler" gorm:"type:varchar(128)"` // Go 代码中注册的 handler 名称

	// Knowledge 配置 (当 implementation_type='knowledge' 时)
	KnowledgeProviderID int64 `json:"knowledge_provider_id" gorm:"default:0"` // 关联到 knowledge_provider 表

	// 风险等级
	RiskLevel string `json:"risk_level" gorm:"type:varchar(16);default:'low'"` // low/medium/high

	// 状态
	Enabled bool `json:"enabled" gorm:"default:true"`

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (AITool) TableName() string {
	return "ai_tool"
}

// 工具类型常量
const (
	AIToolTypeNative    = "native"    // Go 代码实现
	AIToolTypeAPI       = "api"       // HTTP API 调用
	AIToolTypeMCP       = "mcp"       // MCP Server 调用
	AIToolTypeKnowledge = "knowledge" // 知识库查询
)

// 风险等级常量
const (
	AIToolRiskLow    = "low"
	AIToolRiskMedium = "medium"
	AIToolRiskHigh   = "high"
)

// ParameterSchemaStruct JSON Schema 结构
type ParameterSchemaStruct struct {
	Type       string                       `json:"type"`
	Properties map[string]ParameterProperty `json:"properties"`
	Required   []string                     `json:"required,omitempty"`
}

// ParameterProperty 参数属性
type ParameterProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// GetParameterSchema 解析参数 Schema
func (t *AITool) GetParameterSchema() (*ParameterSchemaStruct, error) {
	if t.ParameterSchema == "" {
		return nil, nil
	}
	var schema ParameterSchemaStruct
	if err := json.Unmarshal([]byte(t.ParameterSchema), &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

// SetParameterSchema 设置参数 Schema
func (t *AITool) SetParameterSchema(schema *ParameterSchemaStruct) error {
	data, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	t.ParameterSchema = string(data)
	return nil
}

// ToFunctionDefinition 转换为 OpenAI Function Calling 格式
func (t *AITool) ToFunctionDefinition() map[string]any {
	result := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
		},
	}

	if t.ParameterSchema != "" {
		var schema map[string]any
		if err := json.Unmarshal([]byte(t.ParameterSchema), &schema); err == nil {
			result["function"].(map[string]any)["parameters"] = schema
		}
	}

	return result
}

// CRUD 方法

// AIToolGets 查询工具列表
func AIToolGets(c *ctx.Context, where string, args ...interface{}) ([]AITool, error) {
	var tools []AITool
	session := DB(c).Order("name asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&tools).Error
	return tools, err
}

// AIToolGetEnabled 查询所有启用的工具
func AIToolGetEnabled(c *ctx.Context) ([]AITool, error) {
	return AIToolGets(c, "enabled = ?", true)
}

// AIToolGetByType 根据类型查询工具
func AIToolGetByType(c *ctx.Context, implType string) ([]AITool, error) {
	return AIToolGets(c, "implementation_type = ? AND enabled = ?", implType, true)
}

// AIToolGetById 根据 ID 查询
func AIToolGetById(c *ctx.Context, id int64) (*AITool, error) {
	var tool AITool
	err := DB(c).Where("id = ?", id).First(&tool).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tool, err
}

// AIToolGetByName 根据名称查询
func AIToolGetByName(c *ctx.Context, name string) (*AITool, error) {
	var tool AITool
	err := DB(c).Where("name = ?", name).First(&tool).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tool, err
}

// Create 创建工具
func (t *AITool) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	t.CreateAt = now
	t.UpdateAt = now
	return DB(c).Create(t).Error
}

// Update 更新工具
func (t *AITool) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(t).Updates(updates).Error
}

// Delete 删除工具
func (t *AITool) Delete(c *ctx.Context) error {
	// 删除关联的 Agent 绑定
	if err := DB(c).Where("tool_id = ?", t.Id).Delete(&AIAgentToolRel{}).Error; err != nil {
		return err
	}
	return DB(c).Delete(t).Error
}

// AIAgentToolRel Agent 与工具的关联表
type AIAgentToolRel struct {
	AgentId int64 `json:"agent_id" gorm:"primaryKey"`
	ToolId  int64 `json:"tool_id" gorm:"primaryKey"`
}

func (AIAgentToolRel) TableName() string {
	return "ai_agent_tool_rel"
}

// AIAgentToolRelCreate 创建关联
func AIAgentToolRelCreate(c *ctx.Context, agentId, toolId int64) error {
	rel := &AIAgentToolRel{
		AgentId: agentId,
		ToolId:  toolId,
	}
	return DB(c).Create(rel).Error
}

// AIAgentToolRelDelete 删除关联
func AIAgentToolRelDelete(c *ctx.Context, agentId, toolId int64) error {
	return DB(c).Where("agent_id = ? AND tool_id = ?", agentId, toolId).Delete(&AIAgentToolRel{}).Error
}

// AIAgentToolRelDeleteByAgent 删除 Agent 的所有关联
func AIAgentToolRelDeleteByAgent(c *ctx.Context, agentId int64) error {
	return DB(c).Where("agent_id = ?", agentId).Delete(&AIAgentToolRel{}).Error
}

// AIAgentToolRelGetByAgent 查询 Agent 绑定的工具 ID
func AIAgentToolRelGetByAgent(c *ctx.Context, agentId int64) ([]int64, error) {
	var rels []AIAgentToolRel
	err := DB(c).Where("agent_id = ?", agentId).Find(&rels).Error
	if err != nil {
		return nil, err
	}
	toolIds := make([]int64, len(rels))
	for i, rel := range rels {
		toolIds[i] = rel.ToolId
	}
	return toolIds, nil
}
