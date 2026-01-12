// Package executor 工具分发器
// n9e-2kai: AI 助手模块 - 统一工具执行与分发
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant/agent"
	"github.com/ccfos/nightingale/v6/center/aiassistant/knowledge"
	"github.com/ccfos/nightingale/v6/center/aiassistant/mcp"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// Dispatcher 工具分发器
type Dispatcher struct {
	appCtx            *ctx.Context
	knowledgeRegistry *knowledge.KnowledgeToolRegistry
	mcpManager        *mcp.Manager
	nativeHandlers    map[string]NativeHandler
	httpClient        *http.Client
	baseURL           string // 内部 API 基础 URL
}

// NativeHandler 原生工具处理器
type NativeHandler func(ctx context.Context, args map[string]any) (any, error)

// NewDispatcher 创建工具分发器
func NewDispatcher(appCtx *ctx.Context, knowledgeRegistry *knowledge.KnowledgeToolRegistry, mcpManager *mcp.Manager, baseURL string) *Dispatcher {
	return &Dispatcher{
		appCtx:            appCtx,
		knowledgeRegistry: knowledgeRegistry,
		mcpManager:        mcpManager,
		nativeHandlers:    make(map[string]NativeHandler),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// RegisterNativeHandler 注册原生工具处理器
func (d *Dispatcher) RegisterNativeHandler(name string, handler NativeHandler) {
	d.nativeHandlers[name] = handler
}

// DispatchResult 分发结果
type DispatchResult struct {
	ToolName string
	Result   any
	Error    error
	Duration time.Duration
}

// Dispatch 分发工具调用
func (d *Dispatcher) Dispatch(ctx context.Context, toolConfig *agent.ToolConfig, args map[string]any) *DispatchResult {
	start := time.Now()
	result := &DispatchResult{
		ToolName: toolConfig.Name,
	}

	switch toolConfig.ImplementationType {
	case models.AIToolTypeNative:
		result.Result, result.Error = d.executeNative(ctx, toolConfig, args)
	case models.AIToolTypeAPI:
		result.Result, result.Error = d.executeAPI(ctx, toolConfig, args)
	case models.AIToolTypeMCP:
		result.Result, result.Error = d.executeMCP(ctx, toolConfig, args)
	case models.AIToolTypeKnowledge:
		result.Result, result.Error = d.executeKnowledge(ctx, toolConfig, args)
	default:
		result.Error = fmt.Errorf("unknown tool implementation type: %s", toolConfig.ImplementationType)
	}

	result.Duration = time.Since(start)
	return result
}

// executeNative 执行原生工具
func (d *Dispatcher) executeNative(ctx context.Context, toolConfig *agent.ToolConfig, args map[string]any) (any, error) {
	handler, ok := d.nativeHandlers[toolConfig.NativeHandler]
	if !ok {
		return nil, fmt.Errorf("native handler not found: %s", toolConfig.NativeHandler)
	}
	return handler(ctx, args)
}

// executeAPI 执行 API 工具
func (d *Dispatcher) executeAPI(ctx context.Context, toolConfig *agent.ToolConfig, args map[string]any) (any, error) {
	if toolConfig.URLPath == "" {
		return nil, fmt.Errorf("API URL path not configured for tool: %s", toolConfig.Name)
	}

	url := d.baseURL + toolConfig.URLPath
	method := toolConfig.Method
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	if method == "POST" || method == "PUT" {
		jsonData, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 对于 GET 请求，将参数添加到查询字符串
	if method == "GET" && len(args) > 0 {
		q := req.URL.Query()
		for k, v := range args {
			q.Add(k, fmt.Sprintf("%v", v))
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	// 尝试解析 JSON 响应
	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		// 如果不是 JSON，返回原始字符串
		return string(body), nil
	}

	return result, nil
}

// executeMCP 执行 MCP 工具
func (d *Dispatcher) executeMCP(ctx context.Context, toolConfig *agent.ToolConfig, args map[string]any) (any, error) {
	if d.mcpManager == nil {
		return nil, fmt.Errorf("MCP manager not configured")
	}

	if toolConfig.MCPServerID == 0 {
		return nil, fmt.Errorf("MCP server ID not configured for tool: %s", toolConfig.Name)
	}

	client, exists := d.mcpManager.GetClient(toolConfig.MCPServerID)
	if !exists {
		return nil, fmt.Errorf("MCP client not found for server ID: %d", toolConfig.MCPServerID)
	}

	toolName := toolConfig.MCPToolName
	if toolName == "" {
		toolName = toolConfig.Name
	}

	// 转换参数类型
	argsInterface := make(map[string]interface{})
	for k, v := range args {
		argsInterface[k] = v
	}

	result, err := client.CallTool(ctx, &mcp.ToolRequest{
		Name:      toolName,
		Arguments: argsInterface,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	if result.IsError {
		return nil, fmt.Errorf("MCP tool returned error")
	}

	// 提取文本内容
	var contents []string
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			contents = append(contents, block.Text)
		}
	}

	if len(contents) == 1 {
		return contents[0], nil
	}
	return contents, nil
}

// executeKnowledge 执行知识库工具
func (d *Dispatcher) executeKnowledge(ctx context.Context, toolConfig *agent.ToolConfig, args map[string]any) (any, error) {
	if d.knowledgeRegistry == nil {
		return nil, fmt.Errorf("knowledge registry not configured")
	}

	// 使用知识库注册表执行
	result, err := d.knowledgeRegistry.ExecuteTool(ctx, toolConfig.Name, args)
	if err != nil {
		return nil, fmt.Errorf("knowledge tool execution failed: %w", err)
	}

	return result, nil
}

// DispatchMultiple 并发执行多个工具
func (d *Dispatcher) DispatchMultiple(ctx context.Context, tools []*agent.ToolConfig, argsList []map[string]any) []*DispatchResult {
	if len(tools) != len(argsList) {
		logger.Errorf("tools and argsList length mismatch")
		return nil
	}

	results := make([]*DispatchResult, len(tools))
	done := make(chan int, len(tools))

	for i := range tools {
		go func(idx int) {
			results[idx] = d.Dispatch(ctx, tools[idx], argsList[idx])
			done <- idx
		}(i)
	}

	// 等待所有完成
	for range tools {
		<-done
	}

	return results
}

// GetToolByName 从 Agent 配置中获取工具
func GetToolByName(agentConfig *agent.AgentConfig, toolName string) *agent.ToolConfig {
	for _, tool := range agentConfig.Tools {
		if tool.Name == toolName {
			return tool
		}
	}
	return nil
}
