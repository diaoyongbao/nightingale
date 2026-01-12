// Package confirmation 二次确认管理器
// n9e-2kai: AI 助手模块 - 二次确认管理器
package confirmation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant"
	"github.com/ccfos/nightingale/v6/storage"
	"github.com/google/uuid"
)

// Manager 确认管理器
type Manager struct {
	redis  storage.Redis
	config *Config
}

// Config 确认配置
type Config struct {
	TTL             time.Duration // 确认过期时间
	CleanupInterval time.Duration // 清理间隔
	RedisPrefix     string        // Redis key 前缀
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		TTL:             5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		RedisPrefix:     "ai_assistant:",
	}
}

// PendingConfirmation 待确认信息
type PendingConfirmation struct {
	ConfirmID   string      `json:"confirm_id"`
	SessionID   string      `json:"session_id"`
	UserID      int64       `json:"user_id"`
	RiskLevel   string      `json:"risk_level"` // high/medium
	Summary     string      `json:"summary"`
	Operation   *Operation  `json:"operation"`
	CheckResult interface{} `json:"check_result,omitempty"`
	CreatedAt   int64       `json:"created_at"`
	ExpiresAt   int64       `json:"expires_at"`
}

// Operation 待执行的操作
type Operation struct {
	Type     string                 `json:"type"` // sql/k8s/alert_mute/mcp
	Name     string                 `json:"name"`
	Request  map[string]interface{} `json:"request"`
	ToolName string                 `json:"tool_name,omitempty"`
}

// ConfirmationResult 确认结果
type ConfirmationResult struct {
	Success   bool               `json:"success"`
	Operation *Operation         `json:"operation,omitempty"`
	Error     *aiassistant.Error `json:"error,omitempty"`
}

// NewManager 创建确认管理器
func NewManager(redisClient storage.Redis, config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &Manager{
		redis:  redisClient,
		config: config,
	}
}

// CreateConfirmation 创建待确认记录
func (m *Manager) CreateConfirmation(ctx context.Context, sessionID string, userID int64, riskLevel string, summary string, operation *Operation, checkResult interface{}) (*PendingConfirmation, error) {
	confirmID := fmt.Sprintf("confirm_%s", uuid.New().String())
	now := time.Now()

	confirmation := &PendingConfirmation{
		ConfirmID:   confirmID,
		SessionID:   sessionID,
		UserID:      userID,
		RiskLevel:   riskLevel,
		Summary:     summary,
		Operation:   operation,
		CheckResult: checkResult,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(m.config.TTL).Unix(),
	}

	// 存储到 Redis
	key := m.confirmKey(confirmID)
	data, _ := json.Marshal(confirmation)
	err := m.redis.Set(ctx, key, string(data), m.config.TTL).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to create confirmation: %w", err)
	}

	return confirmation, nil
}

// GetConfirmation 获取待确认记录
func (m *Manager) GetConfirmation(ctx context.Context, confirmID string) (*PendingConfirmation, error) {
	key := m.confirmKey(confirmID)
	data, err := m.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, aiassistant.NewError(aiassistant.ErrCodeConfirmationNotFound, "确认信息未找到或已过期")
	}

	var confirmation PendingConfirmation
	if err := json.Unmarshal([]byte(data), &confirmation); err != nil {
		return nil, fmt.Errorf("failed to parse confirmation: %w", err)
	}

	// 检查是否过期
	if time.Now().Unix() > confirmation.ExpiresAt {
		m.DeleteConfirmation(ctx, confirmID)
		return nil, aiassistant.NewError(aiassistant.ErrCodeConfirmationExpired, "确认已过期，请重新发起操作")
	}

	return &confirmation, nil
}

// ValidateAndConsume 验证并消费确认
func (m *Manager) ValidateAndConsume(ctx context.Context, confirmID string, userID int64, action string) (*ConfirmationResult, error) {
	// 获取确认信息
	confirmation, err := m.GetConfirmation(ctx, confirmID)
	if err != nil {
		if e, ok := err.(*aiassistant.Error); ok {
			return &ConfirmationResult{
				Success: false,
				Error:   e,
			}, nil
		}
		return nil, err
	}

	// 验证用户
	if confirmation.UserID != userID {
		return &ConfirmationResult{
			Success: false,
			Error:   aiassistant.NewError(aiassistant.ErrCodePermissionDenied, "无权确认此操作"),
		}, nil
	}

	// 处理拒绝
	if action == "reject" {
		m.DeleteConfirmation(ctx, confirmID)
		return &ConfirmationResult{
			Success: false,
			Error:   aiassistant.NewError(aiassistant.ErrCodeRiskRejected, "用户已拒绝执行"),
		}, nil
	}

	// 处理确认
	if action == "approve" {
		// 删除确认记录（一次性使用）
		m.DeleteConfirmation(ctx, confirmID)
		return &ConfirmationResult{
			Success:   true,
			Operation: confirmation.Operation,
		}, nil
	}

	return &ConfirmationResult{
		Success: false,
		Error:   aiassistant.NewError(aiassistant.ErrCodeInvalidRequest, "无效的确认动作"),
	}, nil
}

// DeleteConfirmation 删除确认记录
func (m *Manager) DeleteConfirmation(ctx context.Context, confirmID string) error {
	key := m.confirmKey(confirmID)
	return m.redis.Del(ctx, key).Err()
}

// confirmKey 生成确认 key
func (m *Manager) confirmKey(confirmID string) string {
	return fmt.Sprintf("%sconfirm:%s", m.config.RedisPrefix, confirmID)
}

// BuildSQLOperation 构建 SQL 操作
func BuildSQLOperation(sql string, instanceID int64, database string) *Operation {
	return &Operation{
		Type: "sql",
		Name: "执行 SQL",
		Request: map[string]interface{}{
			"sql":         sql,
			"instance_id": instanceID,
			"database":    database,
		},
		ToolName: "dbm.sql_query",
	}
}

// BuildK8sOperation 构建 K8s 操作
func BuildK8sOperation(action string, cluster string, namespace string, resource string, name string, params map[string]interface{}) *Operation {
	request := map[string]interface{}{
		"action":    action,
		"cluster":   cluster,
		"namespace": namespace,
		"resource":  resource,
		"name":      name,
	}
	for k, v := range params {
		request[k] = v
	}

	return &Operation{
		Type:     "k8s",
		Name:     fmt.Sprintf("K8s %s", action),
		Request:  request,
		ToolName: fmt.Sprintf("k8s_%s", action),
	}
}

// BuildAlertMuteOperation 构建告警屏蔽操作
func BuildAlertMuteOperation(action string, busiGroupID int64, muteConfig map[string]interface{}) *Operation {
	return &Operation{
		Type: "alert_mute",
		Name: fmt.Sprintf("告警屏蔽 %s", action),
		Request: map[string]interface{}{
			"action":        action,
			"busi_group_id": busiGroupID,
			"config":        muteConfig,
		},
		ToolName: fmt.Sprintf("alert_mute_%s", action),
	}
}
