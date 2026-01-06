package dbm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/toolkits/pkg/logger"
)

// ArcheryClient Archery API 客户端
type ArcheryClient struct {
	config      *ArcheryConfig
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
	tokenMutex  sync.RWMutex
}

// TokenResponse JWT token 响应
type TokenResponse struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
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

// login 使用用户名密码登录获取 JWT token
func (c *ArcheryClient) login() error {
	url := fmt.Sprintf("%s/api/auth/token/", c.config.Address)

	payload := map[string]string{
		"username": c.config.Username,
		"password": c.config.Password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	logger.Debugf("archery login request: POST %s", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	c.tokenMutex.Lock()
	c.accessToken = tokenResp.Access
	// Token 默认有效期 4 小时，提前 5 分钟刷新
	c.tokenExpiry = time.Now().Add(3*time.Hour + 55*time.Minute)
	c.tokenMutex.Unlock()

	logger.Debugf("archery login successful, token length: %d", len(c.accessToken))
	return nil
}

// ensureToken 确保有有效的 token
func (c *ArcheryClient) ensureToken() error {
	// 如果直接配置了 token，使用配置的 token
	if c.config.AuthType == "token" && c.config.AuthToken != "" {
		c.tokenMutex.Lock()
		c.accessToken = c.config.AuthToken
		c.tokenMutex.Unlock()
		return nil
	}

	// 使用用户名密码登录
	if c.config.Username == "" || c.config.Password == "" {
		return fmt.Errorf("username and password are required for session auth")
	}

	c.tokenMutex.RLock()
	needLogin := c.accessToken == "" || time.Now().After(c.tokenExpiry)
	c.tokenMutex.RUnlock()

	if needLogin {
		return c.login()
	}
	return nil
}

// getAccessToken 获取当前的 access token
func (c *ArcheryClient) getAccessToken() string {
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()
	return c.accessToken
}

// GetInstances 获取实例列表
func (c *ArcheryClient) GetInstances() ([]ArcheryInstance, error) {
	// Archery 使用 size 参数控制分页大小
	url := fmt.Sprintf("%s/api/v1/instance/?size=1000", c.config.Address)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Archery 返回分页格式: {count, next, previous, results}
	var result ArcheryInstanceListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(resp))
	}

	logger.Debugf("archery instances: count=%d, results=%d", result.Count, len(result.Results))

	return result.Results, nil
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
	// 确保有有效的 token (健康检查可能不需要)
	if err := c.ensureToken(); err != nil {
		logger.Warningf("failed to ensure token: %v", err)
		// 继续执行，可能某些接口不需要认证
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置认证头
	token := c.getAccessToken()
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	// 设置通用 Header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	logger.Debugf("archery request: %s %s (token_len=%d)", method, url, len(token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 如果返回 401，尝试重新登录
	if resp.StatusCode == http.StatusUnauthorized {
		logger.Debugf("received 401, attempting re-login")
		c.tokenMutex.Lock()
		c.accessToken = ""
		c.tokenMutex.Unlock()

		if err := c.ensureToken(); err != nil {
			return nil, fmt.Errorf("re-login failed: %w", err)
		}

		// 重试请求
		return c.doRequest(method, url, body)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	logger.Debugf("archery response: %s", string(respBody))

	return respBody, nil
}

