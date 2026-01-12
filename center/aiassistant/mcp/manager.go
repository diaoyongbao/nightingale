// Package mcp MCP 管理器
// n9e-2kai: AI 助手模块 - MCP 管理器
package mcp

import (
	"context"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// Manager MCP 管理器
type Manager struct {
	clients map[int64]MCPClientInterface
	mu      sync.RWMutex
	ctx     *ctx.Context
}

// NewManager 创建 MCP 管理器
func NewManager(c *ctx.Context) *Manager {
	return &Manager{
		clients: make(map[int64]MCPClientInterface),
		ctx:     c,
	}
}

// InitMCPClients 初始化所有外部 MCP Server 连接
func (m *Manager) InitMCPClients(ctx context.Context) error {
	servers, err := models.MCPServerGets(m.ctx, "enabled = 1")
	if err != nil {
		return err
	}

	for _, server := range servers {
		if server.ServerType != "http" && server.ServerType != "sse" {
			logger.Warningf("unsupported server type: %s, skipped", server.ServerType)
			continue
		}

		// 创建 HTTP 客户端
		client := NewHTTPMCPClient(HTTPMCPClientConfig{
			Endpoint:            server.Endpoint,
			HealthCheckURL:      server.HealthCheckURL,
			HealthCheckInterval: server.HealthCheckInterval,
		})

		// 健康检查
		if err := client.Health(ctx); err != nil {
			logger.Errorf("MCP server %s health check failed: %v", server.Name, err)
			m.updateHealthStatus(server.Id, models.MCPHealthStatusFailed, err.Error())
			continue
		}

		m.mu.Lock()
		m.clients[server.Id] = client
		m.mu.Unlock()

		m.updateHealthStatus(server.Id, models.MCPHealthStatusHealthy, "")

		// 启动定期健康检查
		go m.startHealthCheck(ctx, server.Id, client, server.HealthCheckInterval)
	}

	return nil
}

// GetClient 获取 MCP 客户端
func (m *Manager) GetClient(serverID int64) (MCPClientInterface, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, exists := m.clients[serverID]
	return client, exists
}

// GetAllClients 获取所有 MCP 客户端
func (m *Manager) GetAllClients() map[int64]MCPClientInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[int64]MCPClientInterface)
	for k, v := range m.clients {
		result[k] = v
	}
	return result
}

// startHealthCheck 定期健康检查
func (m *Manager) startHealthCheck(ctx context.Context, serverID int64, client MCPClientInterface, interval int) {
	if interval <= 0 {
		interval = 60
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := client.Health(ctx); err != nil {
				logger.Errorf("MCP server %d health check failed: %v", serverID, err)
				m.updateHealthStatus(serverID, models.MCPHealthStatusFailed, err.Error())
			} else {
				m.updateHealthStatus(serverID, models.MCPHealthStatusHealthy, "")
			}
		}
	}
}

// updateHealthStatus 更新健康状态
func (m *Manager) updateHealthStatus(serverID int64, status int, errMsg string) {
	server, err := models.MCPServerGetById(m.ctx, serverID)
	if err != nil || server == nil {
		return
	}
	server.UpdateHealthStatus(m.ctx, status, errMsg)
}

// Close 关闭所有客户端
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[int64]MCPClientInterface)
	return nil
}
