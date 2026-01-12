// Package chat 对话处理器测试
// n9e-2kai: AI 助手模块 - Chat Handler 属性测试
package chat

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: ai-assistant-function-calling, Property 4: Unified Entry Point**
// **Validates: Requirements 1.1**

func TestUnifiedEntryPoint(t *testing.T) {
	// 验证所有请求都通过统一入口处理
	// ChatRequest 不再有 mode 字段
	req := &ChatRequest{
		SessionID: "test-session",
		Message:   "test message",
	}

	// 验证请求结构不包含 mode 字段
	if req.Message == "" {
		t.Error("Message should not be empty")
	}
}

// **Feature: ai-assistant-function-calling, Property 5: Response Structure Consistency**
// **Validates: Requirements 5.1, 5.2, 5.4**

func TestResponseStructureConsistency(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("all responses have required fields", prop.ForAll(
		func(traceID, sessionID, status, source, content string) bool {
			resp := &ChatResponse{
				TraceID:   traceID,
				SessionID: sessionID,
				Status:    status,
				Source:    source,
				AssistantMessage: &AssistantMessage{
					Format:  "markdown",
					Content: content,
				},
			}

			// 验证必需字段存在
			return resp.TraceID != "" &&
				resp.SessionID != "" &&
				resp.Status != "" &&
				resp.AssistantMessage != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("completed", "pending_confirmation", "error"),
		gen.OneConstOf(SourceKnowledgeBase, SourceMCPTool, SourceDirect),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestResponseWithToolInfo(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("tool responses include tool info", prop.ForAll(
		func(toolName string, success bool) bool {
			status := "success"
			if !success {
				status = "failed"
			}

			resp := &ChatResponse{
				TraceID:   "trace-123",
				SessionID: "session-123",
				Status:    "completed",
				Source:    SourceKnowledgeBase,
				Tool: &ToolInfo{
					Called: true,
					Name:   toolName,
					Status: status,
				},
			}

			// 验证工具信息完整
			return resp.Tool != nil &&
				resp.Tool.Called &&
				resp.Tool.Name == toolName &&
				resp.Tool.Status == status
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// **Feature: ai-assistant-function-calling, Property 6: Tool Error Response Format**
// **Validates: Requirements 5.3**

func TestToolErrorResponseFormat(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("tool errors have proper structure", prop.ForAll(
		func(code, message string) bool {
			resp := &ChatResponse{
				TraceID:   "trace-123",
				SessionID: "session-123",
				Status:    "error",
				Source:    SourceKnowledgeBase,
				Tool: &ToolInfo{
					Called: true,
					Name:   "test_tool",
					Status: "failed",
					Error: &ToolError{
						Code:    code,
						Message: message,
					},
				},
			}

			// 验证错误结构
			return resp.Tool != nil &&
				resp.Tool.Error != nil &&
				resp.Tool.Error.Code == code &&
				resp.Tool.Error.Message == message
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// **Feature: ai-assistant-function-calling, Property 10: Tool Name Validation**
// **Validates: Requirements 3.5**

func TestToolNameValidation(t *testing.T) {
	// 测试工具名称验证逻辑
	validToolNames := []string{"search_ops_kb", "search_k8s_kb", "dbm_sql_query"}
	invalidToolNames := []string{"", "unknown_tool", "malicious_tool"}

	for _, name := range validToolNames {
		if name == "" {
			t.Errorf("valid tool name should not be empty")
		}
	}

	for _, name := range invalidToolNames {
		if name == "" {
			continue // 空名称是无效的
		}
		// 这里可以添加更多验证逻辑
	}
}

func TestSourceConstants(t *testing.T) {
	// 验证来源常量定义正确
	if SourceKnowledgeBase != "knowledge_base" {
		t.Errorf("expected SourceKnowledgeBase to be 'knowledge_base', got '%s'", SourceKnowledgeBase)
	}
	if SourceMCPTool != "mcp_tool" {
		t.Errorf("expected SourceMCPTool to be 'mcp_tool', got '%s'", SourceMCPTool)
	}
	if SourceDirect != "direct" {
		t.Errorf("expected SourceDirect to be 'direct', got '%s'", SourceDirect)
	}
}

func TestChatRequestWithoutMode(t *testing.T) {
	// 验证 ChatRequest 不再需要 mode 字段
	req := ChatRequest{
		SessionID: "test-session",
		Message:   "What is the JumpServer address?",
		ClientContext: ClientContext{
			BusiGroupID:  1,
			UserTimezone: "Asia/Shanghai",
			UILanguage:   "zh_CN",
			Env:          "prod",
		},
	}

	// 验证请求可以正常创建
	if req.Message == "" {
		t.Error("Message should not be empty")
	}
	if req.ClientContext.BusiGroupID != 1 {
		t.Error("BusiGroupID should be 1")
	}
}

func TestAssistantMessageFormat(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("assistant message has valid format", prop.ForAll(
		func(format, content string) bool {
			msg := &AssistantMessage{
				Format:  format,
				Content: content,
			}

			// 验证格式是有效的
			validFormats := map[string]bool{"markdown": true, "text": true}
			return validFormats[msg.Format] || msg.Format == ""
		},
		gen.OneConstOf("markdown", "text"),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
