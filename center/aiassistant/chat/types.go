// Package chat 对话处理器
// n9e-2kai: AI 助手模块 - 对话处理器类型定义（Function Calling 架构）
package chat

// ChatRequest 对话请求（统一入口，无需 mode 字段）
type ChatRequest struct {
	SessionID   string       `json:"session_id"`
	Message     string       `json:"message"`
	Attachments []Attachment `json:"attachments,omitempty"`

	// 知识库上下文（用于连续对话）
	ConversationID string `json:"conversation_id,omitempty"` // 知识库对话 ID

	ClientContext ClientContext `json:"client_context"`
	Confirmation  *Confirmation `json:"confirmation,omitempty"`
}

// Attachment 附件
type Attachment struct {
	Type     string `json:"type"` // image/file
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
}

// ClientContext 客户端上下文
type ClientContext struct {
	BusiGroupID  int64  `json:"busi_group_id"`
	UserTimezone string `json:"user_timezone"`
	UILanguage   string `json:"ui_language"`
	Env          string `json:"env"`
}

// Confirmation 确认信息
type Confirmation struct {
	ConfirmID string `json:"confirm_id"`
	Action    string `json:"action"` // approve/reject
}

// ChatResponse 对话响应（统一格式）
type ChatResponse struct {
	TraceID        string `json:"trace_id"`
	SessionID      string `json:"session_id"`
	Status         string `json:"status"` // completed/pending_confirmation/error
	ConversationID string `json:"conversation_id,omitempty"` // 知识库对话 ID

	// 响应内容
	AssistantMessage *AssistantMessage `json:"assistant_message,omitempty"`

	// 内容来源标记（Function Calling 新增）
	Source string `json:"source,omitempty"` // knowledge_base/mcp_tool/direct

	// 工具调用信息
	Tool                *ToolInfo            `json:"tool,omitempty"`
	PendingConfirmation *PendingConfirmation `json:"pending_confirmation,omitempty"`
}

// AssistantMessage 助手消息
type AssistantMessage struct {
	Format  string `json:"format"` // markdown/text
	Content string `json:"content"`
}

// ToolInfo 工具调用信息
type ToolInfo struct {
	Called   bool        `json:"called"`
	Name     string      `json:"name,omitempty"`
	Status   string      `json:"status,omitempty"` // success/failed
	Request  interface{} `json:"request,omitempty"`
	Result   interface{} `json:"result,omitempty"`
	Error    *ToolError  `json:"error,omitempty"`
	CacheHit bool        `json:"cache_hit,omitempty"` // n9e-2kai: 缓存命中标记
}

// ToolError 工具错误
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Raw     string `json:"raw,omitempty"`
}

// PendingConfirmation 待确认信息
type PendingConfirmation struct {
	ConfirmID    string        `json:"confirm_id"`
	RiskLevel    string        `json:"risk_level"` // high/medium
	Summary      string        `json:"summary"`
	ProposedTool *ProposedTool `json:"proposed_tool"`
	CheckResult  interface{}   `json:"check_result,omitempty"`
	ExpiresAt    int64         `json:"expires_at"`
}

// ProposedTool 待执行的工具
type ProposedTool struct {
	Name    string      `json:"name"`
	Request interface{} `json:"request"`
}

// 内容来源常量
const (
	SourceKnowledgeBase = "knowledge_base"
	SourceMCPTool       = "mcp_tool"
	SourceDirect        = "direct"
)
