// Package models MCP Server 配置表模型
// n9e-2kai: AI 助手模块 - MCP Server 模型
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"gorm.io/gorm"
)

// MCPServer MCP Server 配置表
type MCPServer struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Description string `json:"description" gorm:"type:varchar(500)"`

	// 连接配置（仅 http/sse）
	ServerType string `json:"server_type" gorm:"type:varchar(32);not null;default:'http'"` // http 或 sse
	Endpoint   string `json:"endpoint" gorm:"type:varchar(256);not null"`

	// 健康检查
	HealthCheckURL      string `json:"health_check_url" gorm:"type:varchar(256)"`
	HealthCheckInterval int    `json:"health_check_interval" gorm:"default:60"` // 秒

	// 安全配置 - JSON 数组存储
	AllowedEnvs     string `json:"allowed_envs" gorm:"type:text"`     // JSON 数组
	AllowedPrefixes string `json:"allowed_prefixes" gorm:"type:text"` // JSON 数组
	AllowedIPs      string `json:"allowed_ips" gorm:"type:text"`      // JSON 数组（可选）

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

func (MCPServer) TableName() string {
	return "mcp_server"
}

// MCP 健康状态常量
const (
	MCPHealthStatusUnknown = 0
	MCPHealthStatusHealthy = 1
	MCPHealthStatusFailed  = 2
)

// GetAllowedEnvs 获取允许的环境列表
func (m *MCPServer) GetAllowedEnvs() []string {
	if m.AllowedEnvs == "" {
		return nil
	}
	var envs []string
	json.Unmarshal([]byte(m.AllowedEnvs), &envs)
	return envs
}

// SetAllowedEnvs 设置允许的环境列表
func (m *MCPServer) SetAllowedEnvs(envs []string) {
	if len(envs) == 0 {
		m.AllowedEnvs = ""
		return
	}
	data, _ := json.Marshal(envs)
	m.AllowedEnvs = string(data)
}

// GetAllowedPrefixes 获取允许的工具前缀列表
func (m *MCPServer) GetAllowedPrefixes() []string {
	if m.AllowedPrefixes == "" {
		return nil
	}
	var prefixes []string
	json.Unmarshal([]byte(m.AllowedPrefixes), &prefixes)
	return prefixes
}

// SetAllowedPrefixes 设置允许的工具前缀列表
func (m *MCPServer) SetAllowedPrefixes(prefixes []string) {
	if len(prefixes) == 0 {
		m.AllowedPrefixes = ""
		return
	}
	data, _ := json.Marshal(prefixes)
	m.AllowedPrefixes = string(data)
}

// CRUD 方法

// MCPServerGets 查询 MCP Server 列表
func MCPServerGets(c *ctx.Context, where string, args ...interface{}) ([]MCPServer, error) {
	var servers []MCPServer
	session := DB(c).Order("id desc")
	if where != "" {
		session = session.Where(where, args...)
	}
	err := session.Find(&servers).Error
	return servers, err
}

// MCPServerGetById 根据 ID 查询
func MCPServerGetById(c *ctx.Context, id int64) (*MCPServer, error) {
	var server MCPServer
	err := DB(c).Where("id = ?", id).First(&server).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &server, err
}

// MCPServerGetByName 根据名称查询
func MCPServerGetByName(c *ctx.Context, name string) (*MCPServer, error) {
	var server MCPServer
	err := DB(c).Where("name = ?", name).First(&server).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &server, err
}

// Create 创建 MCP Server
func (m *MCPServer) Create(c *ctx.Context) error {
	now := time.Now().Unix()
	m.CreateAt = now
	m.UpdateAt = now
	return DB(c).Create(m).Error
}

// Update 更新 MCP Server
func (m *MCPServer) Update(c *ctx.Context, updates map[string]interface{}) error {
	updates["update_at"] = time.Now().Unix()
	return DB(c).Model(m).Updates(updates).Error
}

// Delete 删除 MCP Server
func (m *MCPServer) Delete(c *ctx.Context) error {
	return DB(c).Delete(m).Error
}

// UpdateHealthStatus 更新健康状态
func (m *MCPServer) UpdateHealthStatus(c *ctx.Context, status int, errMsg string) error {
	return DB(c).Model(m).Updates(map[string]interface{}{
		"health_status":    status,
		"last_check_time":  time.Now().Unix(),
		"last_check_error": errMsg,
	}).Error
}
