// Package mcp MCP 客户端接口定义
// n9e-2kai: AI 助手模块 - MCP 客户端接口
package mcp

import "context"

// MCPClientInterface 定义 MCP 客户端标准接口
type MCPClientInterface interface {
	// Health 健康检查
	Health(ctx context.Context) error

	// ListTools 列出可用工具（JSON-RPC: tools/list）
	ListTools(ctx context.Context) ([]Tool, error)

	// CallTool 调用工具（JSON-RPC: tools/call）
	CallTool(ctx context.Context, req *ToolRequest) (*ToolResponse, error)

	// Close 关闭连接
	Close() error
}

// Tool 定义工具结构（对应 MCP tools/list 返回）
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"` // JSON Schema
}

// ToolRequest 定义工具调用请求
type ToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
}

// ToolResponse 定义工具调用响应
type ToolResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// ContentBlock 内容块
type ContentBlock struct {
	Type string `json:"type"` // text/image/resource
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"` // base64 编码
}

// JSONRPCError JSON-RPC 错误结构
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
