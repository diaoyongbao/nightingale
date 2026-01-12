// Package models MCP 模板表模型
// n9e-2kai: AI 助手模块 - MCP 模板模型
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// MCPTemplate MCP 工具模板表
type MCPTemplate struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Description string `json:"description" gorm:"type:varchar(500)"`

	// 模板内容
	ServerConfig string `json:"server_config" gorm:"type:text;not null"` // JSON
	Category     string `json:"category" gorm:"type:varchar(64);default:'custom'"`

	// 默认模板标记
	IsDefault bool `json:"is_default" gorm:"default:false"`
	IsPublic  bool `json:"is_public" gorm:"default:false"`

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (MCPTemplate) TableName() string {
	return "mcp_template"
}

// 模板分类常量
const (
	MCPTemplateCategoryK8s     = "k8s"
	MCPTemplateCategoryDB      = "db"
	MCPTemplateCategoryMonitor = "monitor"
	MCPTemplateCategoryCustom  = "custom"
)

// CRUD 方法

// MCPTemplateGets 查询模板列表
func MCPTemplateGets(c *ctx.Context, where string, args ...interface{}) ([]MCPTemplate, error) {
	var templates []MCPTemplate
	session := DB(c).Order("id desc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&templates).Error
	return templates, err
}

// MCPTemplateGetById 根据 ID 查询
func MCPTemplateGetById(c *ctx.Context, id int64) (*MCPTemplate, error) {
	var template MCPTemplate
	err := DB(c).Where("id = ?", id).First(&template).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &template, err
}

// MCPTemplateGetByName 根据名称查询
func MCPTemplateGetByName(c *ctx.Context, name string) (*MCPTemplate, error) {
	var template MCPTemplate
	err := DB(c).Where("name = ?", name).First(&template).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &template, err
}

// Create 创建模板
func (m *MCPTemplate) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	m.CreateAt = now
	m.UpdateAt = now
	return DB(c).Create(m).Error
}

// Update 更新模板
func (m *MCPTemplate) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(m).Updates(updates).Error
}

// Delete 删除模板
func (m *MCPTemplate) Delete(c *ctx.Context) error {
	return DB(c).Delete(m).Error
}
