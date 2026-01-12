// Package config 配置加载器
// n9e-2kai: AI 助手模块 - 配置类型定义
package config

// AIModelConfig AI 模型配置
type AIModelConfig struct {
	Provider    string  `json:"provider"` // openai/gemini/azure
	BaseURL     string  `json:"base_url"`
	APIKey      string  `json:"api_key"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// KnowledgeConfig 知识库配置
type KnowledgeConfig struct {
	Type         string `json:"type"` // coze/dify/custom
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	DefaultBotID string `json:"default_bot_id"`
}

// SessionConfig 会话管理配置
type SessionConfig struct {
	TTL                   int64 `json:"ttl"`
	MaxMessagesPerSession int   `json:"max_messages_per_session"`
	MaxSessionsPerUser    int   `json:"max_sessions_per_user"`
}

// FileConfig 文件管理配置
type FileConfig struct {
	MaxSize        int64  `json:"max_size"`
	StorageBackend string `json:"storage_backend"`
	StoragePath    string `json:"storage_path"`
	TTL            int64  `json:"ttl"`
}

// ArchiveConfig 归档策略配置
type ArchiveConfig struct {
	Enabled           bool   `json:"enabled"`
	InactiveThreshold int64  `json:"inactive_threshold"`
	CronSchedule      string `json:"cron_schedule"`
}

// ToolConfig 工具调用配置
type ToolConfig struct {
	CallTimeout        int `json:"call_timeout"`
	MaxCallsPerSession int `json:"max_calls_per_session"`
}

// ConfirmationConfig 确认机制配置
type ConfirmationConfig struct {
	TTL             int `json:"ttl"`
	CleanupInterval int `json:"cleanup_interval"`
}
