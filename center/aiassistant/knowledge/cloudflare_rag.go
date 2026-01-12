// Package knowledge Cloudflare AutoRAG Provider 实现
// n9e-2kai: AI 助手模块 - Cloudflare AutoRAG Provider
package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/toolkits/pkg/logger"
)

// CloudflareRAGProvider Cloudflare AutoRAG Provider
type CloudflareRAGProvider struct {
	config *CloudflareRAGConfig
	client *http.Client
	name   string
}

// CloudflareRAGConfig Cloudflare AutoRAG 配置
type CloudflareRAGConfig struct {
	AccountID      string  `json:"account_id"`
	RAGName        string  `json:"rag_name"`
	APIToken       string  `json:"api_token"`
	Model          string  `json:"model"`
	RewriteQuery   bool    `json:"rewrite_query"`
	MaxNumResults  int     `json:"max_num_results"`
	ScoreThreshold float64 `json:"score_threshold"`
	Timeout        int     `json:"timeout"`
}

// cloudflareSearchRequest Cloudflare API 请求结构
type cloudflareSearchRequest struct {
	Query          string                 `json:"query"`
	Model          string                 `json:"model,omitempty"`
	RewriteQuery   bool                   `json:"rewrite_query"`
	MaxNumResults  int                    `json:"max_num_results"`
	RankingOptions map[string]interface{} `json:"ranking_options,omitempty"`
}

// cloudflareSearchResponse Cloudflare API 响应结构
type cloudflareSearchResponse struct {
	Success  bool                     `json:"success"`
	Errors   []cloudflareError        `json:"errors,omitempty"`
	Messages []string                 `json:"messages,omitempty"`
	Result   cloudflareSearchResult   `json:"result,omitempty"`
}

type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareSearchResult struct {
	Response    string                 `json:"response,omitempty"`
	SearchQuery string                 `json:"search_query,omitempty"`
	Data        []cloudflareSearchData `json:"data,omitempty"`
	HasMore     bool                   `json:"has_more,omitempty"`
}

type cloudflareSearchData struct {
	FileID     string                    `json:"file_id,omitempty"`
	Filename   string                    `json:"filename,omitempty"`
	Score      float64                   `json:"score"`
	Attributes map[string]interface{}    `json:"attributes,omitempty"`
	Content    []cloudflareContentItem   `json:"content"` // content 是数组
}

// cloudflareContentItem Cloudflare content 数组中的元素
type cloudflareContentItem struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// NewCloudflareRAGProvider 创建 Cloudflare AutoRAG Provider
func NewCloudflareRAGProvider(name string, config *CloudflareRAGConfig) *CloudflareRAGProvider {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	return &CloudflareRAGProvider{
		name:   name,
		config: config,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Query 执行知识库查询
func (p *CloudflareRAGProvider) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	queryText := req.GetQueryText()
	if queryText == "" {
		return &QueryResponse{
			Status: "failed",
			Error:  "query text is empty",
		}, nil
	}

	// 构建 API URL
	apiURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/autorag/rags/%s/search",
		p.config.AccountID,
		p.config.RAGName,
	)

	// 确定参数
	maxResults := p.config.MaxNumResults
	if req.MaxResults > 0 {
		maxResults = req.MaxResults
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	scoreThreshold := p.config.ScoreThreshold
	if req.ScoreThreshold > 0 {
		scoreThreshold = req.ScoreThreshold
	}
	if scoreThreshold <= 0 {
		scoreThreshold = 0.3
	}

	// 构建请求体
	searchReq := cloudflareSearchRequest{
		Query:         queryText,
		Model:         p.config.Model,
		RewriteQuery:  p.config.RewriteQuery,
		MaxNumResults: maxResults,
		RankingOptions: map[string]interface{}{
			"score_threshold": scoreThreshold,
		},
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIToken)

	// 发送请求
	logger.Debugf("CloudflareRAG query: %s", queryText)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return &QueryResponse{
			Status: "failed",
			Error:  fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &QueryResponse{
			Status: "failed",
			Error:  fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("CloudflareRAG error: status=%d, body=%s", resp.StatusCode, string(body))
		return &QueryResponse{
			Status: "failed",
			Error:  fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}, nil
	}

	// 解析响应
	var searchResp cloudflareSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return &QueryResponse{
			Status: "failed",
			Error:  fmt.Sprintf("failed to parse response: %v", err),
		}, nil
	}

	// 检查 API 错误
	if !searchResp.Success {
		errMsg := "unknown error"
		if len(searchResp.Errors) > 0 {
			errMsg = searchResp.Errors[0].Message
		}
		return &QueryResponse{
			Status: "failed",
			Error:  errMsg,
		}, nil
	}

	// 构建响应
	results := make([]QueryResult, 0, len(searchResp.Result.Data))
	for _, data := range searchResp.Result.Data {
		// 合并 content 数组中的文本
		var contentText string
		for _, item := range data.Content {
			if item.Text != "" {
				if contentText != "" {
					contentText += "\n"
				}
				contentText += item.Text
			}
		}

		results = append(results, QueryResult{
			Content:  contentText,
			Score:    data.Score,
			Source:   data.Filename,
			Metadata: data.Attributes,
		})
	}

	return &QueryResponse{
		Results:        results,
		Answer:         searchResp.Result.Response,
		Status:         "completed",
		ConversationID: req.ConversationID, // 保持 conversation_id
	}, nil
}

// Health 健康检查
func (p *CloudflareRAGProvider) Health(ctx context.Context) error {
	// 简单的连接测试：发送一个空查询
	apiURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/autorag/rags/%s",
		p.config.AccountID,
		p.config.RAGName,
	)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetProviderName 获取 Provider 名称
func (p *CloudflareRAGProvider) GetProviderName() string {
	return p.name
}

// GetProviderType 获取 Provider 类型
func (p *CloudflareRAGProvider) GetProviderType() string {
	return "cloudflare_autorag"
}
