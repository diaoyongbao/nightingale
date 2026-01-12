// Package models AI 配置表模型
// n9e-2kai: AI 助手模块 - AI 配置模型
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// AIConfig AI 助手配置表
type AIConfig struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ConfigKey   string `json:"config_key" gorm:"type:varchar(128);not null;uniqueIndex"`
	ConfigValue string `json:"config_value" gorm:"type:text;not null"` // JSON 格式
	ConfigType  string `json:"config_type" gorm:"type:varchar(32);not null"`
	Description string `json:"description" gorm:"type:varchar(500)"`

	// 作用范围
	Scope   string `json:"scope" gorm:"type:varchar(32);default:'global'"` // global/busi_group
	ScopeId int64  `json:"scope_id" gorm:"default:0"`                      // 业务组 ID

	// 状态
	Enabled bool `json:"enabled" gorm:"default:true"`

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (AIConfig) TableName() string {
	return "ai_config"
}

// 配置类型常量
const (
	AIConfigTypeAIModel   = "ai_model"
	AIConfigTypeKnowledge = "knowledge"
	AIConfigTypeSession   = "session"
	AIConfigTypeFile      = "file"
	AIConfigTypeGeneral   = "general"
)

// 配置 Key 常量
const (
	AIConfigKeyDefaultModel   = "ai.default_model"
	AIConfigKeyExpertK8s      = "ai.expert.k8s"
	AIConfigKeyExpertDatabase = "ai.expert.database"
	AIConfigKeyExpertAlert    = "ai.expert.alert"
	AIConfigKeyKnowledge      = "knowledge.provider"
	AIConfigKeySession        = "session.config"
	AIConfigKeyConfirmation   = "confirmation.config"
	AIConfigKeyFile           = "file.config"
	AIConfigKeyArchive        = "archive.config"
	AIConfigKeyTool           = "tool.config"
)

// CRUD 方法

// AIConfigGets 查询配置列表
func AIConfigGets(c *ctx.Context, where string, args ...interface{}) ([]AIConfig, error) {
	var configs []AIConfig
	session := DB(c).Order("config_key asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&configs).Error
	return configs, err
}

// AIConfigGetByKey 根据 key 查询
func AIConfigGetByKey(c *ctx.Context, key string) (*AIConfig, error) {
	var config AIConfig
	err := DB(c).Where("config_key = ? AND enabled = 1", key).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

// AIConfigGetByKeyAndScope 根据 key 和 scope 查询
func AIConfigGetByKeyAndScope(c *ctx.Context, key, scope string, scopeId int64) (*AIConfig, error) {
	var config AIConfig
	err := DB(c).Where("config_key = ? AND scope = ? AND scope_id = ? AND enabled = 1", key, scope, scopeId).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

// Create 创建配置
func (a *AIConfig) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	a.CreateAt = now
	a.UpdateAt = now
	return DB(c).Create(a).Error
}

// Update 更新配置
func (a *AIConfig) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(a).Updates(updates).Error
}

// Delete 删除配置
func (a *AIConfig) Delete(c *ctx.Context) error {
	return DB(c).Delete(a).Error
}

// AIConfigUpdateValue 更新配置值（如果不存在则创建）
func AIConfigUpdateValue(c *ctx.Context, key, value, updateBy string) error {
	now := time.Now().Unix()
	
	// 先尝试更新
	result := DB(c).Model(&AIConfig{}).
		Where("config_key = ?", key).
		Updates(map[string]interface{}{
			"config_value": value,
			"update_at":    now,
			"update_by":    updateBy,
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	// 如果没有更新到任何记录，说明记录不存在，需要创建
	if result.RowsAffected == 0 {
		config := &AIConfig{
			ConfigKey:   key,
			ConfigValue: value,
			ConfigType:  "ai_model",
			Description: "AI 模型配置",
			Scope:       "global",
			ScopeId:     0,
			Enabled:     true,
			CreateAt:    now,
			CreateBy:    updateBy,
			UpdateAt:    now,
			UpdateBy:    updateBy,
		}
		return DB(c).Create(config).Error
	}
	
	return nil
}
