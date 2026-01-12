// Package ai 提供统一的 AI 模型调用接口
// n9e-2kai: AI 助手模块 - 共享 AI 客户端
package ai

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AIClient 统一的 AI 模型调用接口
type AIClient interface {
	ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
	StreamCompletion(ctx context.Context, req *ChatCompletionRequest) (chan StreamDelta, error)
}

// Message 消息结构
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string 或 []ContentPart
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentPart 多模态内容块
type ContentPart struct {
	Type     string    `json:"type"` // text, image_url
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL 图片 URL 结构
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // auto, low, high
}

// Tool 工具定义
type Tool struct {
	Type     string       `json:"type"` // function
	Function ToolFunction `json:"function"`
}

// ToolFunction 工具函数定义
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// ToolCall 工具调用请求
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // function
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction 工具调用函数详情
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// ChatCompletionRequest 对话补全请求
type ChatCompletionRequest struct {
	Model        string                 `json:"model"`
	Messages     []Message              `json:"messages"`
	Tools        []Tool                 `json:"tools,omitempty"`
	ToolChoice   interface{}            `json:"tool_choice,omitempty"` // "auto", "none", 或具体工具
	SystemPrompt string                 `json:"-"`                     // 转换为 system message
	Temperature  float64                `json:"temperature,omitempty"`
	MaxTokens    int                    `json:"max_tokens,omitempty"`
	CustomParams map[string]interface{} `json:"-"` // 自定义参数
}

// ChatCompletionResponse 对话补全响应
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 响应选项
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage 用量统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamDelta 流式响应增量
type StreamDelta struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Done      bool       `json:"done"`
	Error     error      `json:"error,omitempty"`
}

// OpenAIClientConfig OpenAI 客户端配置
type OpenAIClientConfig struct {
	Model         string                 `json:"model"`
	APIKey        string                 `json:"api_key"`
	BaseURL       string                 `json:"base_url"`
	Timeout       time.Duration          `json:"timeout"`
	SkipSSLVerify bool                   `json:"skip_ssl_verify"`
	Proxy         string                 `json:"proxy"`
	CustomParams  map[string]interface{} `json:"custom_params"`
}

// OpenAIClient OpenAI 兼容客户端实现
type OpenAIClient struct {
	config     OpenAIClientConfig
	httpClient *http.Client
}

// NewOpenAIClient 创建 OpenAI 客户端
func NewOpenAIClient(config OpenAIClientConfig) (*OpenAIClient, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipSSLVerify,
		},
	}

	client := &OpenAIClient{
		config: config,
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
		},
	}

	return client, nil
}

// ChatCompletion 执行对话补全
func (c *OpenAIClient) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 构建请求体
	body := map[string]interface{}{
		"model":    c.config.Model,
		"messages": c.buildMessages(req),
	}

	// 设置模型（请求中的优先）
	if req.Model != "" {
		body["model"] = req.Model
	}

	// 设置温度
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	// 设置最大 token
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	// 添加工具定义
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
		if req.ToolChoice != nil {
			body["tool_choice"] = req.ToolChoice
		} else {
			body["tool_choice"] = "auto"
		}
	}

	// 合并自定义参数
	for k, v := range c.config.CustomParams {
		body[k] = v
	}
	for k, v := range req.CustomParams {
		body[k] = v
	}

	// 发送请求
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &chatResp, nil
}

// buildMessages 构建消息列表（处理 SystemPrompt）
func (c *OpenAIClient) buildMessages(req *ChatCompletionRequest) []Message {
	var messages []Message

	// 添加 system prompt
	if req.SystemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// 添加对话消息
	messages = append(messages, req.Messages...)

	return messages
}

// StreamCompletion 流式对话补全（暂未实现）
func (c *OpenAIClient) StreamCompletion(ctx context.Context, req *ChatCompletionRequest) (chan StreamDelta, error) {
	// TODO: 实现流式响应
	return nil, fmt.Errorf("streaming not implemented yet")
}
