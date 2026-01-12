// Package aiassistant AI 助手模块
// n9e-2kai: AI 助手模块 - 错误码定义
package aiassistant

import "fmt"

// 错误码定义
const (
	// 通用错误 (1000-1099)
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeInvalidRequest = "INVALID_REQUEST"

	// 会话错误 (1100-1199)
	ErrCodeSessionNotFound      = "SESSION_NOT_FOUND"
	ErrCodeSessionExpired       = "SESSION_EXPIRED"
	ErrCodeSessionLimitExceeded = "SESSION_LIMIT_EXCEEDED"

	// 工具错误 (1200-1299)
	ErrCodeToolNotFound   = "TOOL_NOT_FOUND"
	ErrCodeToolCallFailed = "TOOL_CALL_FAILED"
	ErrCodeToolTimeout    = "TOOL_TIMEOUT"
	ErrCodeUpstreamError  = "UPSTREAM_ERROR"

	// 权限错误 (1300-1399)
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeEnvNotAllowed    = "ENV_NOT_ALLOWED"
	ErrCodeIPNotAllowed     = "IP_NOT_ALLOWED"

	// 确认错误 (1400-1499)
	ErrCodeConfirmationExpired  = "CONFIRMATION_EXPIRED"
	ErrCodeConfirmationNotFound = "CONFIRMATION_NOT_FOUND"
	ErrCodeRiskRejected         = "RISK_REJECTED"

	// 文件错误 (1500-1599)
	ErrCodeFileNotFound    = "FILE_NOT_FOUND"
	ErrCodeFileTooLarge    = "FILE_TOO_LARGE"
	ErrCodeInvalidFileType = "INVALID_FILE_TYPE"

	// MCP 错误 (1600-1699)
	ErrCodeMCPServerNotFound    = "MCP_SERVER_NOT_FOUND"
	ErrCodeMCPConnectionFailed  = "MCP_CONNECTION_FAILED"
	ErrCodeMCPHealthCheckFailed = "MCP_HEALTH_CHECK_FAILED"
)

// Error 定义结构化错误
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewError 创建错误的辅助函数
func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// NewErrorWithDetails 创建带详情的错误
func NewErrorWithDetails(code, message, details string) *Error {
	return &Error{Code: code, Message: message, Details: details}
}


// IsToolError 判断是否是工具级错误
func IsToolError(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		switch e.Code {
		case ErrCodeToolNotFound, ErrCodeToolCallFailed, ErrCodeToolTimeout, ErrCodeUpstreamError:
			return true
		}
	}
	return false
}

// WrapToolError 包装工具错误
func WrapToolError(code, message string, rawErr error) *Error {
	details := ""
	if rawErr != nil {
		details = rawErr.Error()
	}
	return &Error{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// ToolErrorResponse 工具错误响应结构（用于 HTTP 200 返回）
type ToolErrorResponse struct {
	TraceID   string     `json:"trace_id"`
	SessionID string     `json:"session_id"`
	Status    string     `json:"status"` // always "error"
	Tool      *ToolError `json:"tool"`
}

// ToolError 工具错误详情
type ToolError struct {
	Called bool   `json:"called"`
	Name   string `json:"name,omitempty"`
	Error  *Error `json:"error"`
}

// NewToolErrorResponse 创建工具错误响应
func NewToolErrorResponse(traceID, sessionID, toolName string, err *Error) *ToolErrorResponse {
	return &ToolErrorResponse{
		TraceID:   traceID,
		SessionID: sessionID,
		Status:    "error",
		Tool: &ToolError{
			Called: true,
			Name:   toolName,
			Error:  err,
		},
	}
}
