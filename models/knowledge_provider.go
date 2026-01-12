// Package models 知识库 Provider 配置表模型
// n9e-2kai: AI 助手模块 - 知识库 Provider 模型
package models

import (
	"encoding/json"
	"os"
	"regexp"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// KnowledgeProvider 知识库 Provider 配置表
type KnowledgeProvider struct {
	Id           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	ProviderType string `json:"provider_type" gorm:"type:varchar(64);not null"` // cloudflare_autorag/coze/elasticsearch/custom_http
	Description  string `json:"description" gorm:"type:varchar(500)"`
	Config       string `json:"config" gorm:"type:text;not null"` // JSON 格式的 Provider 配置

	// 状态
	Enabled        bool   `json:"enabled" gorm:"default:true"`
	HealthStatus   int    `json:"health_status" gorm:"default:0"` // 0=未知 1=健康 2=异常
	LastCheckTime  int64  `json:"last_check_time" gorm:"default:0"`
	LastCheckError string `json:"last_check_error" gorm:"type:text"`

	// 审计
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (KnowledgeProvider) TableName() string {
	return "knowledge_provider"
}

// Provider 类型常量
const (
	ProviderTypeCloudflareAutoRAG = "cloudflare_autorag"
	ProviderTypeCoze              = "coze"
	ProviderTypeElasticsearch     = "elasticsearch"
	ProviderTypeCustomHTTP        = "custom_http"
)

// 健康状态常量（使用 KP 前缀避免与其他模块冲突）
const (
	KPHealthStatusUnknown   = 0
	KPHealthStatusHealthy   = 1
	KPHealthStatusUnhealthy = 2
)

// CloudflareRAGConfig Cloudflare AutoRAG 配置结构
type CloudflareRAGConfig struct {
	AccountID      string  `json:"account_id"`
	RAGName        string  `json:"rag_name"`
	APIToken       string  `json:"api_token"`
	Model          string  `json:"model"`
	RewriteQuery   bool    `json:"rewrite_query"`
	MaxNumResults  int     `json:"max_num_results"`
	ScoreThreshold float64 `json:"score_threshold"`
	Timeout        int     `json:"timeout"`
}

// CRUD 方法

// KnowledgeProviderGets 查询 Provider 列表
func KnowledgeProviderGets(c *ctx.Context, where string, args ...interface{}) ([]KnowledgeProvider, error) {
	var providers []KnowledgeProvider
	session := DB(c).Order("id asc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&providers).Error
	return providers, err
}

// KnowledgeProviderGetById 根据 ID 查询
func KnowledgeProviderGetById(c *ctx.Context, id int64) (*KnowledgeProvider, error) {
	var provider KnowledgeProvider
	err := DB(c).Where("id = ?", id).First(&provider).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &provider, err
}

// KnowledgeProviderGetByName 根据名称查询
func KnowledgeProviderGetByName(c *ctx.Context, name string) (*KnowledgeProvider, error) {
	var provider KnowledgeProvider
	err := DB(c).Where("name = ?", name).First(&provider).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &provider, err
}

// KnowledgeProviderGetEnabled 获取所有启用的 Provider
func KnowledgeProviderGetEnabled(c *ctx.Context) ([]KnowledgeProvider, error) {
	return KnowledgeProviderGets(c, "enabled = ?", true)
}

// Create 创建 Provider
func (p *KnowledgeProvider) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	p.CreateAt = now
	p.UpdateAt = now
	return DB(c).Create(p).Error
}

// Update 更新 Provider
func (p *KnowledgeProvider) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(p).Updates(updates).Error
}

// Delete 删除 Provider
func (p *KnowledgeProvider) Delete(c *ctx.Context) error {
	return DB(c).Delete(p).Error
}

// UpdateHealthStatus 更新健康状态
func (p *KnowledgeProvider) UpdateHealthStatus(c *ctx.Context, status int, errMsg string) error {
	return DB(c).Model(p).Updates(map[string]interface{}{
		"health_status":    status,
		"last_check_time":  time.Now().Unix(),
		"last_check_error": errMsg,
	}).Error
}

// GetConfigParsed 解析配置并展开环境变量
func (p *KnowledgeProvider) GetConfigParsed() (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
		return nil, err
	}
	// 展开环境变量
	expandEnvVars(config)
	return config, nil
}

// GetCloudflareConfig 获取 Cloudflare 配置
func (p *KnowledgeProvider) GetCloudflareConfig() (*CloudflareRAGConfig, error) {
	expandedConfig := expandEnvString(p.Config)
	var config CloudflareRAGConfig
	if err := json.Unmarshal([]byte(expandedConfig), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// expandEnvVars 递归展开 map 中的环境变量
func expandEnvVars(m map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			m[k] = expandEnvString(val)
		case map[string]interface{}:
			expandEnvVars(val)
		}
	}
}

// expandEnvString 展开字符串中的环境变量 ${VAR_NAME}
func expandEnvString(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}
