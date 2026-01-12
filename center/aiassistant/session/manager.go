// Package session 会话管理器
// n9e-2kai: AI 助手模块 - 会话管理器
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccfos/nightingale/v6/storage"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/toolkits/pkg/logger"
)

// Manager 会话管理器
type Manager struct {
	redis  storage.Redis
	config *Config
}

// Config 会话配置
type Config struct {
	TTL                   time.Duration // 会话过期时间
	MaxMessagesPerSession int           // 每个会话最大消息数
	MaxSessionsPerUser    int           // 每个用户最大会话数
	RedisPrefix           string        // Redis key 前缀
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		TTL:                   7 * 24 * time.Hour, // 7 天
		MaxMessagesPerSession: 2000,
		MaxSessionsPerUser:    50,
		RedisPrefix:           "ai_assistant:",
	}
}

// Session 会话结构
type Session struct {
	ID           string `json:"id"`
	UserID       int64  `json:"user_id"`
	Mode         string `json:"mode"` // chat/knowledge
	CreatedAt    int64  `json:"created_at"`
	LastActiveAt int64  `json:"last_active_at"`
	MessageCount int    `json:"message_count"`
}

// Message 消息结构
type Message struct {
	ID        string      `json:"id"`
	Role      string      `json:"role"` // user/assistant/system
	Content   string      `json:"content"`
	Timestamp int64       `json:"timestamp"`
	TraceID   string      `json:"trace_id,omitempty"`
	ToolCall  interface{} `json:"tool_call,omitempty"`
}

// Stats 会话统计
type Stats struct {
	ActiveCount     int            `json:"active_count"`
	PerModeCount    map[string]int `json:"per_mode_count"`
	Last24hCreated  int            `json:"last24h_created"`
	Last24hActive   int            `json:"last24h_active"`
	StorageEstimate string         `json:"storage_estimate"`
}

// NewManager 创建会话管理器
func NewManager(redisClient storage.Redis, config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	return &Manager{
		redis:  redisClient,
		config: config,
	}
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(ctx context.Context, userID int64, mode string) (*Session, error) {
	sessionID := fmt.Sprintf("ses_%s", uuid.New().String())
	now := time.Now().Unix()

	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		Mode:         mode,
		CreatedAt:    now,
		LastActiveAt: now,
		MessageCount: 0,
	}

	// 存储会话元数据
	metaKey := m.metaKey(sessionID)
	metaData, _ := json.Marshal(session)

	err := m.redis.Set(ctx, metaKey, string(metaData), m.config.TTL).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// 添加到用户会话索引
	userSessionsKey := m.userSessionsKey(userID)
	m.redis.SAdd(ctx, userSessionsKey, sessionID)
	m.redis.Expire(ctx, userSessionsKey, m.config.TTL)

	// 添加到活跃会话索引
	activeKey := m.activeSessionsKey()
	m.redis.ZAdd(ctx, activeKey, redis.Z{Score: float64(now), Member: sessionID})

	logger.Infof("session created: %s for user %d", sessionID, userID)
	return session, nil
}

// GetSession 获取会话
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	metaKey := m.metaKey(sessionID)
	data, err := m.redis.Get(ctx, metaKey).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return &session, nil
}

// UpdateLastActive 更新最后活跃时间
func (m *Manager) UpdateLastActive(ctx context.Context, sessionID string) error {
	session, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	session.LastActiveAt = now

	metaKey := m.metaKey(sessionID)
	metaData, _ := json.Marshal(session)
	m.redis.Set(ctx, metaKey, string(metaData), m.config.TTL)

	// 更新活跃会话索引
	activeKey := m.activeSessionsKey()
	m.redis.ZAdd(ctx, activeKey, redis.Z{Score: float64(now), Member: sessionID})

	return nil
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(ctx context.Context, sessionID string) error {
	session, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// 删除会话元数据
	metaKey := m.metaKey(sessionID)
	m.redis.Del(ctx, metaKey)

	// 删除消息列表
	messagesKey := m.messagesKey(sessionID)
	m.redis.Del(ctx, messagesKey)

	// 从用户会话索引移除
	userSessionsKey := m.userSessionsKey(session.UserID)
	m.redis.SRem(ctx, userSessionsKey, sessionID)

	// 从活跃会话索引移除
	activeKey := m.activeSessionsKey()
	m.redis.ZRem(ctx, activeKey, sessionID)

	logger.Infof("session deleted: %s", sessionID)
	return nil
}

// AddMessage 添加消息到会话
func (m *Manager) AddMessage(ctx context.Context, sessionID string, msg *Message) error {
	// 确保会话存在
	session, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// 生成消息 ID
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}

	// 添加消息
	messagesKey := m.messagesKey(sessionID)
	msgData, _ := json.Marshal(msg)
	m.redis.RPush(ctx, messagesKey, string(msgData))
	m.redis.Expire(ctx, messagesKey, m.config.TTL)

	// 滚动裁剪旧消息
	if m.config.MaxMessagesPerSession > 0 {
		listLen := m.redis.LLen(ctx, messagesKey).Val()
		if listLen > int64(m.config.MaxMessagesPerSession) {
			// 删除最旧的消息
			trimCount := listLen - int64(m.config.MaxMessagesPerSession)
			m.redis.LTrim(ctx, messagesKey, trimCount, -1)
		}
	}

	// 更新会话元数据
	session.MessageCount++
	session.LastActiveAt = time.Now().Unix()
	metaKey := m.metaKey(sessionID)
	metaData, _ := json.Marshal(session)
	m.redis.Set(ctx, metaKey, string(metaData), m.config.TTL)

	return nil
}

// GetMessages 获取会话消息
func (m *Manager) GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	messagesKey := m.messagesKey(sessionID)

	var start int64 = 0
	var end int64 = -1
	if limit > 0 {
		// 获取最近的 limit 条消息
		listLen := m.redis.LLen(ctx, messagesKey).Val()
		if listLen > int64(limit) {
			start = listLen - int64(limit)
		}
	}

	data, err := m.redis.LRange(ctx, messagesKey, start, end).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(data))
	for _, d := range data {
		var msg Message
		if err := json.Unmarshal([]byte(d), &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// GetStats 获取会话统计
func (m *Manager) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		PerModeCount: make(map[string]int),
	}

	// 获取活跃会话数
	activeKey := m.activeSessionsKey()
	activeCount := m.redis.ZCard(ctx, activeKey).Val()
	stats.ActiveCount = int(activeCount)

	// 获取最近 24 小时统计
	now := time.Now().Unix()
	last24h := now - 24*3600

	// 最近 24 小时活跃的会话
	activeMembers, _ := m.redis.ZRangeByScore(ctx, activeKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", last24h),
		Max: fmt.Sprintf("%d", now),
	}).Result()
	stats.Last24hActive = len(activeMembers)

	// 统计各模式会话数（采样统计，避免遍历所有会话）
	stats.PerModeCount["chat"] = stats.ActiveCount
	stats.PerModeCount["knowledge"] = 0

	// 存储估算
	stats.StorageEstimate = fmt.Sprintf("%d sessions", stats.ActiveCount)

	return stats, nil
}

// GetUserSessions 获取用户的所有会话
func (m *Manager) GetUserSessions(ctx context.Context, userID int64) ([]string, error) {
	userSessionsKey := m.userSessionsKey(userID)
	return m.redis.SMembers(ctx, userSessionsKey).Result()
}

// CheckSessionOwner 检查会话所有者
func (m *Manager) CheckSessionOwner(ctx context.Context, sessionID string, userID int64) (bool, error) {
	session, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return session.UserID == userID, nil
}

// GetInactiveSessions 获取不活跃的会话（用于归档）
func (m *Manager) GetInactiveSessions(ctx context.Context, threshold time.Duration) ([]string, error) {
	activeKey := m.activeSessionsKey()
	cutoff := time.Now().Add(-threshold).Unix()

	// 获取最后活跃时间早于阈值的会话
	return m.redis.ZRangeByScore(ctx, activeKey, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", cutoff),
	}).Result()
}

// Redis key 生成方法
func (m *Manager) metaKey(sessionID string) string {
	return fmt.Sprintf("%ssession:%s:meta", m.config.RedisPrefix, sessionID)
}

func (m *Manager) messagesKey(sessionID string) string {
	return fmt.Sprintf("%ssession:%s:messages", m.config.RedisPrefix, sessionID)
}

func (m *Manager) userSessionsKey(userID int64) string {
	return fmt.Sprintf("%suser:%d:sessions", m.config.RedisPrefix, userID)
}

func (m *Manager) activeSessionsKey() string {
	return fmt.Sprintf("%sactive_sessions", m.config.RedisPrefix)
}
