package dbm

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/toolkits/pkg/logger"
)

// ArcheryClient Archery API 客户端
type ArcheryClient struct {
	config     *ArcheryConfig
	httpClient *http.Client
}

// NewArcheryClient 创建 Archery 客户端
func NewArcheryClient(config *ArcheryConfig) (*ArcheryClient, error) {
	if !config.Enable {
		return nil, fmt.Errorf("archery integration is disabled")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("archery address is required")
	}

	// 设置默认超时
	if config.Timeout == 0 {
		config.Timeout = 5000
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 2000
	}

	// 创建 HTTP 客户端
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(config.Timeout) * time.Millisecond,
	}

	return &ArcheryClient{
		config:     config,
		httpClient: client,
	}, nil
}

// GetInstances 获取实例列表
func (c *ArcheryClient) GetInstances() ([]ArcheryInstance, error) {
	url := fmt.Sprintf("%s/api/v1/instance/", c.config.Address)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result ArcheryInstanceListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("archery api error: %s", result.Msg)
	}

	return result.Data, nil
}

// HealthCheck 健康检查
func (c *ArcheryClient) HealthCheck() error {
	url := fmt.Sprintf("%s/api/health/", c.config.Address)

	_, err := c.doRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("archery health check failed: %w", err)
	}

	return nil
}

// doRequest 执行 HTTP 请求
func (c *ArcheryClient) doRequest(method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置认证
	switch c.config.AuthType {
	case "token":
		if c.config.AuthToken != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.AuthToken))
		}
	case "basic":
		if c.config.Username != "" && c.config.Password != "" {
			req.SetBasicAuth(c.config.Username, c.config.Password)
		}
	}

	// 设置通用 Header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	logger.Debugf("archery request: %s %s", method, url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	logger.Debugf("archery response: %s", string(respBody))

	return respBody, nil
}
