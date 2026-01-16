// Package chat 对话处理器
// n9e-2kai: AI 助手模块 - 对话处理器（Function Calling 架构 + 动态 Agent 支持）
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant/agent"
	"github.com/ccfos/nightingale/v6/center/aiassistant/confirmation"
	"github.com/ccfos/nightingale/v6/center/aiassistant/executor"
	"github.com/ccfos/nightingale/v6/center/aiassistant/knowledge"
	"github.com/ccfos/nightingale/v6/center/aiassistant/optimization"
	"github.com/ccfos/nightingale/v6/center/aiassistant/risk"
	"github.com/ccfos/nightingale/v6/center/aiassistant/router"
	"github.com/ccfos/nightingale/v6/center/aiassistant/session"
	"github.com/ccfos/nightingale/v6/pkg/ai"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/google/uuid"
	"github.com/toolkits/pkg/logger"
)

// Handler 对话处理器
type Handler struct {
	aiClient          ai.AIClient
	sessionManager    *session.Manager
	confirmManager    *confirmation.Manager
	riskChecker       *risk.Checker
	knowledgeRegistry *knowledge.KnowledgeToolRegistry
	optManager        *optimization.Manager
	agentLoader       *agent.Loader        // n9e-2kai: 动态 Agent 加载器
	agentRouter       *agent.Router        // n9e-2kai: 动态 Agent 路由器
	toolDispatcher    *executor.Dispatcher // n9e-2kai: 工具分发器
	ctx               *ctx.Context
}

// NewHandler 创建对话处理器
func NewHandler(c *ctx.Context, aiClient ai.AIClient, sessionMgr *session.Manager, confirmMgr *confirmation.Manager, riskChecker *risk.Checker, knowledgeRegistry *knowledge.KnowledgeToolRegistry, optManager *optimization.Manager) *Handler {
	h := &Handler{
		aiClient:          aiClient,
		sessionManager:    sessionMgr,
		confirmManager:    confirmMgr,
		riskChecker:       riskChecker,
		knowledgeRegistry: knowledgeRegistry,
		optManager:        optManager,
		ctx:               c,
	}

	// 初始化动态 Agent 系统
	h.agentLoader = agent.NewLoader(c)
	if err := h.agentLoader.Load(); err != nil {
		logger.Warningf("Failed to load agents from database: %v", err)
	}

	if aiClient != nil {
		h.agentRouter = agent.NewRouter(h.agentLoader, aiClient)
	}

	// 初始化工具分发器
	h.toolDispatcher = executor.NewDispatcher(c, knowledgeRegistry, nil, "")

	return h
}

// SetToolDispatcher 设置工具分发器（用于外部注入 MCP Manager 等）
func (h *Handler) SetToolDispatcher(dispatcher *executor.Dispatcher) {
	h.toolDispatcher = dispatcher
}

// ReloadAgents 重新加载 Agent 配置
func (h *Handler) ReloadAgents() error {
	return h.agentLoader.Reload()
}

// GetAgentsForMention 获取可被 @Mention 的 Agent 列表
func (h *Handler) GetAgentsForMention() []*agent.AgentConfig {
	if h.agentLoader == nil {
		return nil
	}
	return h.agentLoader.GetAgentsForMention()
}

// HandleChat 处理对话请求（统一入口，Function Calling 架构）
func (h *Handler) HandleChat(ctx context.Context, req *ChatRequest, userID int64) (*ChatResponse, error) {
	traceID := uuid.New().String()

	sessionID := req.SessionID
	if sessionID == "" {
		if h.sessionManager != nil {
			sess, err := h.sessionManager.CreateSession(ctx, userID, "chat")
			if err == nil {
				sessionID = sess.ID
			} else {
				sessionID = fmt.Sprintf("ses_%s", uuid.New().String())
			}
		} else {
			sessionID = fmt.Sprintf("ses_%s", uuid.New().String())
		}
	} else {
		if h.sessionManager != nil {
			h.sessionManager.UpdateLastActive(ctx, sessionID)
		}
	}

	if req.Confirmation != nil {
		return h.handleConfirmation(ctx, req, traceID, sessionID, userID)
	}

	if h.sessionManager != nil {
		h.sessionManager.AddMessage(ctx, sessionID, &session.Message{
			Role:      "user",
			Content:   req.Message,
			Timestamp: time.Now().Unix(),
			TraceID:   traceID,
		})
	}

	response, err := h.handleWithFunctionCalling(ctx, req, traceID, sessionID, userID)

	if err == nil && response != nil && response.AssistantMessage != nil && h.sessionManager != nil {
		h.sessionManager.AddMessage(ctx, sessionID, &session.Message{
			Role:      "assistant",
			Content:   response.AssistantMessage.Content,
			Timestamp: time.Now().Unix(),
			TraceID:   traceID,
		})
	}

	return response, err
}

// handleWithFunctionCalling 使用 Function Calling 处理请求
func (h *Handler) handleWithFunctionCalling(ctx context.Context, req *ChatRequest, traceID, sessionID string, userID int64) (*ChatResponse, error) {
	if h.aiClient == nil {
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "error",
			Source:    SourceDirect,
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: "AI 服务未配置，请联系管理员配置 AI 模型。",
			},
		}, nil
	}

	tools := h.buildToolDefinitions(req.ClientContext)

	var historyMessages []ai.Message
	if h.sessionManager != nil {
		messages, _ := h.sessionManager.GetMessages(ctx, sessionID, 10)
		for _, msg := range messages {
			historyMessages = append(historyMessages, ai.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	historyMessages = append(historyMessages, ai.Message{
		Role:    "user",
		Content: req.Message,
	})

	// n9e-2kai: 使用模型路由获取路由阶段的模型配置
	chatReq := &ai.ChatCompletionRequest{
		Messages:     historyMessages,
		SystemPrompt: h.buildSystemPrompt(req.ClientContext),
		Tools:        tools,
	}

	// 应用模型路由配置（路由阶段使用 routing 任务类型）
	if h.optManager != nil {
		if modelConfig := h.optManager.GetModelConfig(router.TaskTypeRouting); modelConfig != nil {
			if modelConfig.Model != "" {
				chatReq.Model = modelConfig.Model
			}
			if modelConfig.MaxTokens > 0 {
				chatReq.MaxTokens = modelConfig.MaxTokens
			}
			if modelConfig.Temperature > 0 {
				chatReq.Temperature = modelConfig.Temperature
			}
		}
	}

	llmResp, err := h.aiClient.ChatCompletion(ctx, chatReq)

	// n9e-2kai: 记录 LLM 调用成本
	if err == nil && llmResp != nil && h.optManager != nil {
		model := chatReq.Model
		if model == "" && llmResp.Model != "" {
			model = llmResp.Model
		}
		if err := h.optManager.RecordUsage(ctx, fmt.Sprintf("%d", userID), sessionID, model, llmResp.Usage.PromptTokens, llmResp.Usage.CompletionTokens); err != nil {
			logger.Warningf("Failed to record LLM usage: %v", err)
		}
	}

	if err != nil {
		logger.Errorf("LLM call failed: %v", err)
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "error",
			Source:    SourceDirect,
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: fmt.Sprintf("AI 调用失败: %v", err),
			},
		}, nil
	}

	if len(llmResp.Choices) > 0 && len(llmResp.Choices[0].Message.ToolCalls) > 0 {
		toolCalls := llmResp.Choices[0].Message.ToolCalls

		// n9e-2kai: 如果有多个工具调用，使用并发执行器
		if len(toolCalls) > 1 && h.optManager != nil {
			return h.handleMultipleToolCalls(ctx, req, traceID, sessionID, userID, historyMessages, toolCalls)
		}

		// 单个工具调用
		toolCall := toolCalls[0]
		return h.handleToolCall(ctx, req, traceID, sessionID, userID, historyMessages, toolCall)
	}

	content := ""
	if len(llmResp.Choices) > 0 {
		if s, ok := llmResp.Choices[0].Message.Content.(string); ok {
			content = s
		}
	}

	return &ChatResponse{
		TraceID:   traceID,
		SessionID: sessionID,
		Status:    "completed",
		Source:    SourceDirect,
		AssistantMessage: &AssistantMessage{
			Format:  "markdown",
			Content: content,
		},
	}, nil
}

// handleToolCall 处理工具调用
func (h *Handler) handleToolCall(ctx context.Context, req *ChatRequest, traceID, sessionID string, userID int64, historyMessages []ai.Message, toolCall ai.ToolCall) (*ChatResponse, error) {
	toolName := toolCall.Function.Name
	logger.Infof("Tool call: %s, args: %s", toolName, toolCall.Function.Arguments)

	if h.knowledgeRegistry != nil && h.knowledgeRegistry.IsKnowledgeTool(toolName) {
		return h.executeKnowledgeTool(ctx, req, traceID, sessionID, userID, historyMessages, toolCall)
	}

	return &ChatResponse{
		TraceID:   traceID,
		SessionID: sessionID,
		Status:    "error",
		Source:    SourceMCPTool,
		AssistantMessage: &AssistantMessage{
			Format:  "text",
			Content: fmt.Sprintf("工具 %s 暂未实现", toolName),
		},
		Tool: &ToolInfo{
			Called: true,
			Name:   toolName,
			Status: "failed",
			Error: &ToolError{
				Code:    "TOOL_NOT_IMPLEMENTED",
				Message: fmt.Sprintf("工具 %s 暂未实现", toolName),
			},
		},
	}, nil
}

// handleMultipleToolCalls 并发处理多个工具调用
// n9e-2kai: 使用并发执行器处理多个工具调用
func (h *Handler) handleMultipleToolCalls(ctx context.Context, req *ChatRequest, traceID, sessionID string, userID int64, historyMessages []ai.Message, toolCalls []ai.ToolCall) (*ChatResponse, error) {
	logger.Infof("Handling %d tool calls concurrently", len(toolCalls))

	// 构建工具调用列表
	execToolCalls := make([]executor.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		execToolCalls[i] = executor.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
		}
		execToolCalls[i].Function.Name = tc.Function.Name
		execToolCalls[i].Function.Arguments = tc.Function.Arguments

		// 解析参数
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
			execToolCalls[i].Args = args
		}
	}

	// 定义工具执行函数
	toolExecutor := func(execCtx context.Context, tc executor.ToolCall) (interface{}, error) {
		// 目前只支持知识库工具
		if h.knowledgeRegistry != nil && h.knowledgeRegistry.IsKnowledgeTool(tc.Name) {
			return h.knowledgeRegistry.ExecuteTool(execCtx, tc.Name, tc.Args)
		}
		return nil, fmt.Errorf("tool %s not implemented", tc.Name)
	}

	// 并发执行
	results := h.optManager.ExecuteToolsConcurrently(ctx, execToolCalls, toolExecutor)

	// 聚合结果
	var toolResults []string
	var toolInfos []*ToolInfo
	for i, result := range results {
		toolInfo := &ToolInfo{
			Called:  true,
			Name:    result.ToolName,
			Status:  "success",
			Request: execToolCalls[i].Args,
		}

		if result.Error != nil {
			toolInfo.Status = "failed"
			toolInfo.Error = &ToolError{
				Code:    "TOOL_EXECUTION_FAILED",
				Message: result.Error.Error(),
			}
			toolResults = append(toolResults, fmt.Sprintf("工具 %s 执行失败: %v", result.ToolName, result.Error))
		} else {
			toolInfo.Result = result.Result
			if queryResp, ok := result.Result.(*knowledge.QueryResponse); ok {
				toolResults = append(toolResults, knowledge.FormatResultsForLLM(queryResp))
			} else {
				toolResults = append(toolResults, fmt.Sprintf("工具 %s 执行成功", result.ToolName))
			}
		}
		toolInfos = append(toolInfos, toolInfo)
	}

	// 构建工具结果消息
	toolResultMessages := historyMessages
	toolResultMessages = append(toolResultMessages, ai.Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCalls,
	})

	for i, tc := range toolCalls {
		toolResultMessages = append(toolResultMessages, ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    toolResults[i],
		})
	}

	// 使用模型路由获取汇总阶段的模型配置
	summaryReq := &ai.ChatCompletionRequest{
		Messages:     toolResultMessages,
		SystemPrompt: h.buildSystemPrompt(req.ClientContext),
	}

	if modelConfig := h.optManager.GetModelConfig(router.TaskTypeSummary); modelConfig != nil {
		if modelConfig.Model != "" {
			summaryReq.Model = modelConfig.Model
		}
		if modelConfig.MaxTokens > 0 {
			summaryReq.MaxTokens = modelConfig.MaxTokens
		}
		if modelConfig.Temperature > 0 {
			summaryReq.Temperature = modelConfig.Temperature
		}
	}

	finalResp, err := h.aiClient.ChatCompletion(ctx, summaryReq)

	// n9e-2kai: 记录汇总 LLM 调用成本
	if err == nil && finalResp != nil && h.optManager != nil {
		model := summaryReq.Model
		if model == "" && finalResp.Model != "" {
			model = finalResp.Model
		}
		if recordErr := h.optManager.RecordUsage(ctx, fmt.Sprintf("%d", userID), sessionID, model, finalResp.Usage.PromptTokens, finalResp.Usage.CompletionTokens); recordErr != nil {
			logger.Warningf("Failed to record LLM usage: %v", recordErr)
		}
	}

	if err != nil {
		logger.Errorf("Final LLM call failed: %v", err)
		// 返回原始工具结果
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "completed",
			Source:    SourceMCPTool,
			AssistantMessage: &AssistantMessage{
				Format:  "markdown",
				Content: fmt.Sprintf("工具执行完成，但汇总失败:\n\n%s", toolResults[0]),
			},
			Tool: toolInfos[0],
		}, nil
	}

	finalContent := ""
	if len(finalResp.Choices) > 0 {
		if s, ok := finalResp.Choices[0].Message.Content.(string); ok {
			finalContent = s
		}
	}

	return &ChatResponse{
		TraceID:   traceID,
		SessionID: sessionID,
		Status:    "completed",
		Source:    SourceMCPTool,
		AssistantMessage: &AssistantMessage{
			Format:  "markdown",
			Content: finalContent,
		},
		Tool: toolInfos[0], // 返回第一个工具信息
	}, nil
}

// executeKnowledgeTool 执行知识库工具
func (h *Handler) executeKnowledgeTool(ctx context.Context, req *ChatRequest, traceID, sessionID string, userID int64, historyMessages []ai.Message, toolCall ai.ToolCall) (*ChatResponse, error) {
	toolName := toolCall.Function.Name

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		logger.Errorf("Failed to parse tool arguments: %v", err)
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "error",
			Source:    SourceKnowledgeBase,
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: "工具参数解析失败",
			},
			Tool: &ToolInfo{
				Called: true,
				Name:   toolName,
				Status: "failed",
				Error: &ToolError{
					Code:    "INVALID_TOOL_ARGUMENTS",
					Message: "工具参数解析失败",
				},
			},
		}, nil
	}

	// 传递 conversation_id 以保持知识库对话上下文
	if req.ConversationID != "" {
		args["conversation_id"] = req.ConversationID
	}

	// n9e-2kai: 检查缓存
	var queryResult *knowledge.QueryResponse
	var cacheHit bool
	if h.optManager != nil {
		if cached, hit := h.optManager.GetCachedResult(ctx, toolName, args); hit && cached != nil {
			if result, ok := cached.Result.(*knowledge.QueryResponse); ok {
				queryResult = result
				cacheHit = true
				logger.Infof("Cache hit for tool %s", toolName)
			}
		}
	}

	// 缓存未命中，执行工具（带重试）
	if !cacheHit {
		// n9e-2kai: 使用重试处理器执行工具
		if h.optManager != nil {
			retryResult := h.optManager.ExecuteWithRetry(ctx, func() (interface{}, error) {
				return h.knowledgeRegistry.ExecuteTool(ctx, toolName, args)
			})

			if retryResult.Error != nil {
				logger.Errorf("Knowledge tool execution failed after %d attempts: %v", retryResult.Attempts, retryResult.Error)
				return &ChatResponse{
					TraceID:   traceID,
					SessionID: sessionID,
					Status:    "error",
					Source:    SourceKnowledgeBase,
					AssistantMessage: &AssistantMessage{
						Format:  "text",
						Content: fmt.Sprintf("知识库查询失败（重试 %d 次）: %v", retryResult.Attempts, retryResult.Error),
					},
					Tool: &ToolInfo{
						Called:  true,
						Name:    toolName,
						Status:  "failed",
						Request: args,
						Error: &ToolError{
							Code:    "KNOWLEDGE_QUERY_FAILED",
							Message: retryResult.Error.Error(),
						},
					},
				}, nil
			}

			if result, ok := retryResult.Result.(*knowledge.QueryResponse); ok {
				queryResult = result
			}
		} else {
			// 无优化管理器时直接执行
			var err error
			queryResult, err = h.knowledgeRegistry.ExecuteTool(ctx, toolName, args)
			if err != nil {
				logger.Errorf("Knowledge tool execution failed: %v", err)
				return &ChatResponse{
					TraceID:   traceID,
					SessionID: sessionID,
					Status:    "error",
					Source:    SourceKnowledgeBase,
					AssistantMessage: &AssistantMessage{
						Format:  "text",
						Content: fmt.Sprintf("知识库查询失败: %v", err),
					},
					Tool: &ToolInfo{
						Called:  true,
						Name:    toolName,
						Status:  "failed",
						Request: args,
						Error: &ToolError{
							Code:    "KNOWLEDGE_QUERY_FAILED",
							Message: err.Error(),
						},
					},
				}, nil
			}
		}

		// n9e-2kai: 存储到缓存
		if h.optManager != nil && queryResult.Status == "completed" {
			if err := h.optManager.SetCachedResult(ctx, toolName, args, queryResult); err != nil {
				logger.Warningf("Failed to cache tool result: %v", err)
			}
		}
	}

	if queryResult.Status == "failed" {
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "error",
			Source:    SourceKnowledgeBase,
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: fmt.Sprintf("知识库查询失败: %s", queryResult.Error),
			},
			Tool: &ToolInfo{
				Called:  true,
				Name:    toolName,
				Status:  "failed",
				Request: args,
				Error: &ToolError{
					Code:    "KNOWLEDGE_QUERY_FAILED",
					Message: queryResult.Error,
				},
			},
		}, nil
	}

	knowledgeContent := knowledge.FormatResultsForLLM(queryResult)

	// n9e-2kai: 知识库无结果或结果无意义时，直接返回，不调用 LLM 汇总
	noMeaningfulResults := len(queryResult.Results) == 0 ||
		knowledgeContent == "未找到相关信息" ||
		knowledgeContent == ""

	if noMeaningfulResults && queryResult.Answer == "" {
		logger.Infof("Knowledge base returned no meaningful results")

		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "completed",
			Source:    SourceKnowledgeBase,
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: "知识库中没有找到相关信息。",
			},
			Tool: &ToolInfo{
				Called:  true,
				Name:    toolName,
				Status:  "success",
				Request: args,
			},
		}, nil
	}

	toolResultMessages := append(historyMessages,
		ai.Message{
			Role:      "assistant",
			Content:   "",
			ToolCalls: []ai.ToolCall{toolCall},
		},
		ai.Message{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			Content:    knowledgeContent,
		},
	)

	// n9e-2kai: 使用模型路由获取汇总阶段的模型配置
	summaryReq := &ai.ChatCompletionRequest{
		Messages:     toolResultMessages,
		SystemPrompt: h.buildSystemPrompt(req.ClientContext),
	}

	// 应用模型路由配置（汇总阶段使用 summary 任务类型）
	if h.optManager != nil {
		if modelConfig := h.optManager.GetModelConfig(router.TaskTypeSummary); modelConfig != nil {
			if modelConfig.Model != "" {
				summaryReq.Model = modelConfig.Model
			}
			if modelConfig.MaxTokens > 0 {
				summaryReq.MaxTokens = modelConfig.MaxTokens
			}
			if modelConfig.Temperature > 0 {
				summaryReq.Temperature = modelConfig.Temperature
			}
		}
	}

	finalResp, err := h.aiClient.ChatCompletion(ctx, summaryReq)

	// n9e-2kai: 记录汇总 LLM 调用成本
	if err == nil && finalResp != nil && h.optManager != nil {
		model := summaryReq.Model
		if model == "" && finalResp.Model != "" {
			model = finalResp.Model
		}
		if recordErr := h.optManager.RecordUsage(ctx, fmt.Sprintf("%d", userID), sessionID, model, finalResp.Usage.PromptTokens, finalResp.Usage.CompletionTokens); recordErr != nil {
			logger.Warningf("Failed to record LLM usage: %v", recordErr)
		}
	}

	if err != nil {
		logger.Errorf("Final LLM call failed: %v", err)
		return &ChatResponse{
			TraceID:        traceID,
			SessionID:      sessionID,
			Status:         "completed",
			Source:         SourceKnowledgeBase,
			ConversationID: queryResult.ConversationID,
			AssistantMessage: &AssistantMessage{
				Format:  "markdown",
				Content: knowledgeContent,
			},
			Tool: &ToolInfo{
				Called:   true,
				Name:     toolName,
				Status:   "success",
				Request:  args,
				Result:   queryResult,
				CacheHit: cacheHit,
			},
		}, nil
	}

	finalContent := ""
	if len(finalResp.Choices) > 0 {
		if s, ok := finalResp.Choices[0].Message.Content.(string); ok {
			finalContent = s
		}
	}

	return &ChatResponse{
		TraceID:        traceID,
		SessionID:      sessionID,
		Status:         "completed",
		Source:         SourceKnowledgeBase,
		ConversationID: queryResult.ConversationID,
		AssistantMessage: &AssistantMessage{
			Format:  "markdown",
			Content: finalContent,
		},
		Tool: &ToolInfo{
			Called:   true,
			Name:     toolName,
			Status:   "success",
			Request:  args,
			CacheHit: cacheHit,
		},
	}, nil
}

// buildToolDefinitions 构建工具定义列表
func (h *Handler) buildToolDefinitions(clientCtx ClientContext) []ai.Tool {
	var tools []ai.Tool

	if h.knowledgeRegistry != nil {
		kbTools := h.knowledgeRegistry.GetToolDefinitions()
		for _, kt := range kbTools {
			tools = append(tools, ai.Tool{
				Type: kt.Type,
				Function: ai.ToolFunction{
					Name:        kt.Function.Name,
					Description: kt.Function.Description,
					Parameters:  kt.Function.Parameters,
				},
			})
		}
	}

	return tools
}

// handleConfirmation 处理确认操作
func (h *Handler) handleConfirmation(ctx context.Context, req *ChatRequest, traceID, sessionID string, userID int64) (*ChatResponse, error) {
	if h.confirmManager == nil {
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "error",
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: "确认服务未配置",
			},
		}, nil
	}

	result, err := h.confirmManager.ValidateAndConsume(ctx, req.Confirmation.ConfirmID, userID, req.Confirmation.Action)
	if err != nil {
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "error",
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: fmt.Sprintf("确认处理失败: %v", err),
			},
		}, nil
	}

	if !result.Success {
		errorMsg := "操作已取消"
		if result.Error != nil {
			errorMsg = result.Error.Message
		}
		return &ChatResponse{
			TraceID:   traceID,
			SessionID: sessionID,
			Status:    "completed",
			AssistantMessage: &AssistantMessage{
				Format:  "text",
				Content: errorMsg,
			},
		}, nil
	}

	return &ChatResponse{
		TraceID:   traceID,
		SessionID: sessionID,
		Status:    "completed",
		AssistantMessage: &AssistantMessage{
			Format:  "text",
			Content: fmt.Sprintf("操作已确认执行: %s", result.Operation.Name),
		},
		Tool: &ToolInfo{
			Called: true,
			Name:   result.Operation.ToolName,
			Status: "success",
		},
	}, nil
}

// buildSystemPrompt 构建系统提示词
func (h *Handler) buildSystemPrompt(clientCtx ClientContext) string {
	return fmt.Sprintf(`你是夜莺监控系统的运维助手。

当前环境: %s
时区: %s
语言: %s

## 工具使用原则

### 知识库查询规则
当调用知识库工具时，query 参数必须：
- **直接使用用户的完整原始问题**
- 不要自行提取关键词或改写问题
- 保持原句的完整性，以便知识库进行语义搜索

正确示例：
- 用户问"jumpserver地址是什么" → query: "jumpserver地址是什么"
- 用户问"K8s如何配置deployment" → query: "K8s如何配置deployment"

错误示例（不要这样做）：
- query: "jumpserver 地址" (这是提取的关键词) ❌
- query: "K8s deployment" (这是提取的关键词) ❌

## 回答原则
- 如果知识库返回了相关结果，基于结果回答
- **严禁修改知识库中返回的图片链接 (Markdown 格式)，必须原样保留**
- 如果知识库没有相关信息，用你的知识直接回答用户问题
- 禁止编造不存在的信息
- 所有写操作必须二次确认
`, clientCtx.Env, clientCtx.UserTimezone, clientCtx.UILanguage)
}

// BuildPendingConfirmation 构建待确认响应
func BuildPendingConfirmation(traceID, sessionID, summary string, tool *ProposedTool) *ChatResponse {
	confirmID := fmt.Sprintf("confirm_%s", uuid.New().String())
	return &ChatResponse{
		TraceID:   traceID,
		SessionID: sessionID,
		Status:    "pending_confirmation",
		AssistantMessage: &AssistantMessage{
			Format:  "markdown",
			Content: fmt.Sprintf("⚠️ 检测到高风险操作，请确认：\n\n%s", summary),
		},
		PendingConfirmation: &PendingConfirmation{
			ConfirmID:    confirmID,
			RiskLevel:    "high",
			Summary:      summary,
			ProposedTool: tool,
			ExpiresAt:    time.Now().Add(5 * time.Minute).Unix(),
		},
	}
}

// CheckSQLRisk 检查 SQL 风险并返回响应
func (h *Handler) CheckSQLRisk(ctx context.Context, sql string, traceID, sessionID string, userID int64, instanceID int64, database string) (*ChatResponse, bool) {
	if h.riskChecker == nil {
		return nil, false
	}

	riskLevel := h.riskChecker.CheckSQL(sql)
	if riskLevel != risk.LevelHigh {
		return nil, false
	}

	if h.confirmManager != nil {
		operation := confirmation.BuildSQLOperation(sql, instanceID, database)
		pending, err := h.confirmManager.CreateConfirmation(ctx, sessionID, userID, string(riskLevel), "执行高风险 SQL 操作", operation, nil)
		if err == nil {
			return &ChatResponse{
				TraceID:   traceID,
				SessionID: sessionID,
				Status:    "pending_confirmation",
				AssistantMessage: &AssistantMessage{
					Format:  "markdown",
					Content: fmt.Sprintf("⚠️ 检测到高风险 SQL 操作，请确认：\n\n```sql\n%s\n```", sql),
				},
				PendingConfirmation: &PendingConfirmation{
					ConfirmID: pending.ConfirmID,
					RiskLevel: "high",
					Summary:   "执行高风险 SQL 操作",
					ProposedTool: &ProposedTool{
						Name:    "dbm.sql_query",
						Request: operation.Request,
					},
					ExpiresAt: pending.ExpiresAt,
				},
			}, true
		}
	}

	return nil, false
}
