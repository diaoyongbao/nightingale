// Package knowledge 知识库 Provider 接口
// n9e-2kai: AI 助手模块 - 知识库接口（Function Calling 架构）
package knowledge

import "context"

// Provider 知识库查询抽象接口
type Provider interface {
	// Query 查询知识库
	Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error)

	// Health 健康检查
	Health(ctx context.Context) error

	// GetProviderName 获取 Provider 名称
	GetProviderName() string

	// GetProviderType 获取 Provider 类型
	GetProviderType() string
}

// QueryRequest 查询请求（支持 Function Calling）
type QueryRequest struct {
	Query          string                 `json:"query"`            // 查询内容（从 Function Calling 参数提取）
	UserID         string                 `json:"user_id"`          // 用户 ID
	SessionID      string                 `json:"session_id"`       // 夜莺的 session_id
	ConversationID string                 `json:"conversation_id"`  // 知识库的 conversation_id（用于上下文保持）
	MaxResults     int                    `json:"max_num_results"`  // 最大返回数量
	ScoreThreshold float64                `json:"score_threshold"`  // 相关性阈值
	Extra          map[string]interface{} `json:"extra,omitempty"`  // Provider 特定参数

	// 兼容旧版字段
	Message string `json:"message,omitempty"` // 兼容旧版，等同于 Query
	BotID   string `json:"bot_id,omitempty"`  // Bot ID（Coze 等使用）
}

// QueryResponse 查询响应
type QueryResponse struct {
	Results        []QueryResult `json:"results,omitempty"`  // 查询结果列表
	ConversationID string        `json:"conversation_id"`    // 用于后续对话
	Answer         string        `json:"answer"`             // 汇总答案（可选）
	Status         string        `json:"status"`             // completed/failed
	Error          string        `json:"error,omitempty"`    // 错误信息
}

// QueryResult 单条查询结果
type QueryResult struct {
	Content  string                 `json:"content"`            // 文档内容
	Score    float64                `json:"score"`              // 相关性分数
	Source   string                 `json:"source,omitempty"`   // 来源文档
	Metadata map[string]interface{} `json:"metadata,omitempty"` // 元数据
}

// GetQueryText 获取查询文本（兼容新旧字段）
func (r *QueryRequest) GetQueryText() string {
	if r.Query != "" {
		return r.Query
	}
	return r.Message
}
