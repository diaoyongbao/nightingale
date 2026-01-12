// Package models 知识库工具配置表模型
// n9e-2kai: AI 助手模块 - 知识库工具模型
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// KnowledgeTool 知识库工具配置表
type KnowledgeTool struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"` // 工具名称，如 search_ops_kb
	Description string `json:"description" gorm:"type:varchar(1000);not null"`     // 工具描述，供 LLM 理解何时调用
	ProviderID  int64  `json:"provider_id" gorm:"not null"`                        // 关联的 Provider ID

	// 工具参数配置
	Parameters string `json:"parameters" gorm:"type:text"` // 额外参数配置（JSON）
	Keywords   string `json:"keywords" gorm:"type:text"`   // 触发关键词（JSON 数组，辅助判断）

	// 状态
	Enabled  bool `json:"enabled" gorm:"default:true"`
	Priority int  `json:"priority" gorm:"default:0"` // 优先级，多个工具匹配时使用

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (KnowledgeTool) TableName() string {
	return "knowledge_tool"
}

// ToolParameters 工具参数结构
type ToolParameters struct {
	MaxResults     int     `json:"max_results,omitempty"`
	ScoreThreshold float64 `json:"score_threshold,omitempty"`
}

// CRUD 方法

// KnowledgeToolGets 查询工具列表
func KnowledgeToolGets(c *ctx.Context, where string, args ...interface{}) ([]KnowledgeTool, error) {
	var tools []KnowledgeTool
	session := DB(c).Order("priority desc, id asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&tools).Error
	return tools, err
}

// KnowledgeToolGetById 根据 ID 查询
func KnowledgeToolGetById(c *ctx.Context, id int64) (*KnowledgeTool, error) {
	var tool KnowledgeTool
	err := DB(c).Where("id = ?", id).First(&tool).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tool, err
}

// KnowledgeToolGetByName 根据名称查询
func KnowledgeToolGetByName(c *ctx.Context, name string) (*KnowledgeTool, error) {
	var tool KnowledgeTool
	err := DB(c).Where("name = ?", name).First(&tool).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tool, err
}

// KnowledgeToolGetEnabled 获取所有启用的工具
func KnowledgeToolGetEnabled(c *ctx.Context) ([]KnowledgeTool, error) {
	return KnowledgeToolGets(c, "enabled = ?", true)
}

// KnowledgeToolGetByProviderID 根据 Provider ID 获取工具
func KnowledgeToolGetByProviderID(c *ctx.Context, providerID int64) ([]KnowledgeTool, error) {
	return KnowledgeToolGets(c, "provider_id = ?", providerID)
}

// Create 创建工具
func (t *KnowledgeTool) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	t.CreateAt = now
	t.UpdateAt = now
	return DB(c).Create(t).Error
}

// Update 更新工具
func (t *KnowledgeTool) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(t).Updates(updates).Error
}

// Delete 删除工具
func (t *KnowledgeTool) Delete(c *ctx.Context) error {
	return DB(c).Delete(t).Error
}

// GetKeywords 获取关键词列表
func (t *KnowledgeTool) GetKeywords() ([]string, error) {
	if t.Keywords == "" {
		return []string{}, nil
	}
	var keywords []string
	if err := json.Unmarshal([]byte(t.Keywords), &keywords); err != nil {
		return nil, err
	}
	return keywords, nil
}

// GetParameters 获取参数配置
func (t *KnowledgeTool) GetParameters() (*ToolParameters, error) {
	if t.Parameters == "" {
		return &ToolParameters{}, nil
	}
	var params ToolParameters
	if err := json.Unmarshal([]byte(t.Parameters), &params); err != nil {
		return nil, err
	}
	return &params, nil
}

// SetKeywords 设置关键词列表
func (t *KnowledgeTool) SetKeywords(keywords []string) error {
	data, err := json.Marshal(keywords)
	if err != nil {
		return err
	}
	t.Keywords = string(data)
	return nil
}

// SetParameters 设置参数配置
func (t *KnowledgeTool) SetParameters(params *ToolParameters) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	t.Parameters = string(data)
	return nil
}

// KnowledgeToolWithProvider 带 Provider 信息的工具
type KnowledgeToolWithProvider struct {
	KnowledgeTool
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
}

// KnowledgeToolGetEnabledWithProvider 获取启用的工具及其 Provider 信息
func KnowledgeToolGetEnabledWithProvider(c *ctx.Context) ([]KnowledgeToolWithProvider, error) {
	var results []KnowledgeToolWithProvider
	err := DB(c).Table("knowledge_tool").
		Select("knowledge_tool.*, knowledge_provider.name as provider_name, knowledge_provider.provider_type").
		Joins("LEFT JOIN knowledge_provider ON knowledge_tool.provider_id = knowledge_provider.id").
		Where("knowledge_tool.enabled = ? AND knowledge_provider.enabled = ?", true, true).
		Order("knowledge_tool.priority desc, knowledge_tool.id asc").
		Scan(&results).Error
	return results, err
}
