// Package mcp HTTP MCP 客户端实现
// n9e-2kai: AI 助手模块 - HTTP MCP 客户端
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// HTTPMCPClient HTTP MCP 客户端
type HTTPMCPClient struct {
	endpoint   string
	httpClient *http.Client
	requestID  int64 // 自增的 JSON-RPC 请求 ID
}

// HTTPMCPClientConfig HTTP MCP 客户端配置
type HTTPMCPClientConfig struct {
	Endpoint            string
	HealthCheckURL      string
	HealthCheckInterval int
	Timeout             time.Duration
}

// NewHTTPMCPClient 创建 HTTP MCP 客户端
func NewHTTPMCPClient(config HTTPMCPClientConfig) *HTTPMCPClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &HTTPMCPClient{
		endpoint: config.Endpoint,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ListTools 实现 tools/list（JSON-RPC 方法）
func (c *HTTPMCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	reqID := atomic.AddInt64(&c.requestID, 1)

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int64  `json:"id"`
		Result  *struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"result,omitempty"`
		Error *JSONRPCError `json:"error,omitempty"`
	}

	if err := c.doJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
	}

	if response.Result == nil {
		return nil, fmt.Errorf("empty result")
	}

	return response.Result.Tools, nil
}

// CallTool 实现 tools/call（JSON-RPC 方法）
func (c *HTTPMCPClient) CallTool(ctx context.Context, req *ToolRequest) (*ToolResponse, error) {
	reqID := atomic.AddInt64(&c.requestID, 1)

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      req.Name,
			"arguments": req.Arguments,
		},
	}

	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int64  `json:"id"`
		Result  *struct {
			Content []ContentBlock `json:"content"`
			IsError bool           `json:"isError"`
		} `json:"result,omitempty"`
		Error *JSONRPCError `json:"error,omitempty"`
	}

	if err := c.doJSONRPCRequest(ctx, request, &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
	}

	if response.Result == nil {
		return nil, fmt.Errorf("empty result")
	}

	return &ToolResponse{
		Content: response.Result.Content,
		IsError: response.Result.IsError,
	}, nil
}

// Health 健康检查
func (c *HTTPMCPClient) Health(ctx context.Context) error {
	// 通过调用 tools/list 来检查健康状态
	_, err := c.ListTools(ctx)
	return err
}

// Close 关闭连接
func (c *HTTPMCPClient) Close() error {
	// HTTP 客户端无需显式关闭
	return nil
}

// doJSONRPCRequest 发送 JSON-RPC 请求
func (c *HTTPMCPClient) doJSONRPCRequest(ctx context.Context, request interface{}, response interface{}) error {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", httpResp.StatusCode)
	}

	return json.NewDecoder(httpResp.Body).Decode(response)
}
