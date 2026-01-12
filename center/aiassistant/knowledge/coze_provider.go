// Package knowledge Coze Provider 实现
// n9e-2kai: AI 助手模块 - Coze Provider
package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// CozeProvider Coze 知识库 Provider
type CozeProvider struct {
	apiKey       string
	baseURL      string
	defaultBotID string
	client       *http.Client
}

// CozeConfig Coze 配置
type CozeConfig struct {
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	DefaultBotID string `json:"default_bot_id"`
}

// NewCozeProvider 创建 Coze Provider
func NewCozeProvider(config CozeConfig) *CozeProvider {
	return &CozeProvider{
		apiKey:       config.APIKey,
		baseURL:      config.BaseURL,
		defaultBotID: config.DefaultBotID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Query 查询知识库
func (p *CozeProvider) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	botID := req.BotID
	if botID == "" {
		botID = p.defaultBotID
	}

	if botID == "" {
		return &QueryResponse{
			Status: "failed",
			Error:  "bot_id is required",
		}, nil
	}

	// 构建 Coze API v3 请求
	cozeReq := map[string]interface{}{
		"bot_id":            botID,
		"user_id":           req.UserID,
		"stream":            false,
		"auto_save_history": true,
		"additional_messages": []map[string]interface{}{
			{
				"role":         "user",
				"content":      req.Message,
				"content_type": "text",
			},
		},
	}

	// 如果有 conversation_id，则继续之前的对话
	if req.ConversationID != "" {
		cozeReq["conversation_id"] = req.ConversationID
	}

	jsonData, _ := json.Marshal(cozeReq)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v3/chat", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	var cozeResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ConversationID string `json:"conversation_id"`
			Status         string `json:"status"`
		} `json:"data"`
		Messages []struct {
			Role        string `json:"role"`
			Content     string `json:"content"`
			ContentType string `json:"content_type"`
		} `json:"messages"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&cozeResp); err != nil {
		return nil, err
	}

	if cozeResp.Code != 0 {
		return &QueryResponse{
			Status: "failed",
			Error:  cozeResp.Msg,
		}, nil
	}

	// 提取 assistant 回复
	var answer string
	for _, msg := range cozeResp.Messages {
		if msg.Role == "assistant" && msg.ContentType == "text" {
			answer = msg.Content
			break
		}
	}

	return &QueryResponse{
		ConversationID: cozeResp.Data.ConversationID,
		Answer:         answer,
		Status:         cozeResp.Data.Status,
	}, nil
}

// Health 健康检查
func (p *CozeProvider) Health(ctx context.Context) error {
	// 简单的健康检查（可选实现）
	return nil
}

// GetProviderName 获取 Provider 名称
func (p *CozeProvider) GetProviderName() string {
	return "coze"
}
