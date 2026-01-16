// Package models AI LLM 模型配置表
// n9e-2kai: AI 助手模块 - 自定义 LLM 模型管理
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// AILLMModel AI LLM 模型配置表
type AILLMModel struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"` // 模型名称（显示用）
	ModelId     string `json:"model_id" gorm:"type:varchar(128);not null"`         // 模型 ID（API 调用用）
	Provider    string `json:"provider" gorm:"type:varchar(64);not null"`          // 提供商（如 openai, anthropic, deepseek 等）
	APIKey      string `json:"api_key" gorm:"type:varchar(256);not null"`          // API 密钥
	BaseURL     string `json:"base_url" gorm:"type:varchar(256);not null"`         // API 基础地址
	Temperature float64 `json:"temperature" gorm:"type:decimal(3,2);default:0.7"`  // 温度参数
	MaxTokens   int    `json:"max_tokens" gorm:"default:4096"`                     // 最大 Token 数
	Timeout     int    `json:"timeout" gorm:"default:60"`                          // 超时时间（秒）
	Description string `json:"description" gorm:"type:varchar(500)"`               // 模型描述

	// 状态
	IsDefault bool `json:"is_default" gorm:"default:false"` // 是否为默认模型
	Enabled   bool `json:"enabled" gorm:"default:true"`     // 是否启用

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (AILLMModel) TableName() string {
	return "ai_llm_model"
}

// CRUD 方法

// AILLMModelGets 查询 LLM 模型列表
func AILLMModelGets(c *ctx.Context, where string, args ...interface{}) ([]AILLMModel, error) {
	var models []AILLMModel
	session := DB(c).Order("is_default desc, name asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&models).Error
	return models, err
}

// AILLMModelGetEnabled 查询所有启用的 LLM 模型
func AILLMModelGetEnabled(c *ctx.Context) ([]AILLMModel, error) {
	return AILLMModelGets(c, "enabled = ?", true)
}

// AILLMModelGetDefault 获取默认 LLM 模型
func AILLMModelGetDefault(c *ctx.Context) (*AILLMModel, error) {
	var model AILLMModel
	err := DB(c).Where("is_default = ? AND enabled = ?", true, true).First(&model).Error
	if err == gorm.ErrRecordNotFound {
		// 如果没有默认模型，返回第一个启用的模型
		err = DB(c).Where("enabled = ?", true).Order("id asc").First(&model).Error
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
	}
	return &model, err
}

// AILLMModelGetById 根据 ID 查询
func AILLMModelGetById(c *ctx.Context, id int64) (*AILLMModel, error) {
	var model AILLMModel
	err := DB(c).Where("id = ?", id).First(&model).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &model, err
}

// AILLMModelGetByName 根据名称查询
func AILLMModelGetByName(c *ctx.Context, name string) (*AILLMModel, error) {
	var model AILLMModel
	err := DB(c).Where("name = ?", name).First(&model).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &model, err
}

// Create 创建 LLM 模型
func (m *AILLMModel) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	m.CreateAt = now
	m.UpdateAt = now

	// 如果设置为默认模型，先取消其他默认模型
	if m.IsDefault {
		DB(c).Model(&AILLMModel{}).Where("is_default = ?", true).Update("is_default", false)
	}

	return DB(c).Create(m).Error
}

// Update 更新 LLM 模型
func (m *AILLMModel) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()

	// 如果设置为默认模型，先取消其他默认模型
	if isDefault, ok := updates["is_default"]; ok && isDefault == true {
		DB(c).Model(&AILLMModel{}).Where("id != ? AND is_default = ?", m.Id, true).Update("is_default", false)
	}

	return DB(c).Model(m).Updates(updates).Error
}

// Delete 删除 LLM 模型
func (m *AILLMModel) Delete(c *ctx.Context) error {
	return DB(c).Delete(m).Error
}

// SetAsDefault 设置为默认模型
func (m *AILLMModel) SetAsDefault(c *ctx.Context, username string) error {
	// 先取消所有默认模型
	if err := DB(c).Model(&AILLMModel{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return err
	}

	// 设置当前模型为默认
	return DB(c).Model(m).Updates(map[string]interface{}{
		"is_default": true,
		"update_at":  time.Now().Unix(),
		"update_by":  username,
	}).Error
}
