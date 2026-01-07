package dbm

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/toolkits/pkg/logger"
)

// ConnectionManager 数据库连接池管理器
type ConnectionManager struct {
	pools map[int64]*sql.DB // instanceID -> connection pool
	mu    sync.RWMutex
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		pools: make(map[int64]*sql.DB),
	}
}

// GetConnection 获取数据库连接池
func (m *ConnectionManager) GetConnection(instanceID int64, dsn string, maxOpenConns, maxIdleConns int) (*sql.DB, error) {
	m.mu.RLock()
	pool, exists := m.pools[instanceID]
	m.mu.RUnlock()

	if exists {
		// 检查连接是否有效
		if err := pool.Ping(); err == nil {
			return pool, nil
		}
		// 连接失败,移除并重新创建
		m.RemoveConnection(instanceID)
	}

	// 创建新连接池
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check
	if pool, exists := m.pools[instanceID]; exists {
		return pool, nil
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池参数
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetConnMaxIdleTime(90 * time.Second)

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	m.pools[instanceID] = db
	logger.Infof("created connection pool for instance %d, max_open=%d, max_idle=%d", instanceID, maxOpenConns, maxIdleConns)
	return db, nil
}

// RemoveConnection 移除连接池
func (m *ConnectionManager) RemoveConnection(instanceID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, exists := m.pools[instanceID]; exists {
		delete(m.pools, instanceID)
		return pool.Close()
	}
	return nil
}

// Close 关闭所有连接池
func (m *ConnectionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for id, pool := range m.pools {
		if err := pool.Close(); err != nil {
			logger.Errorf("failed to close connection pool for instance %d: %v", id, err)
			lastErr = err
		}
	}
	m.pools = make(map[int64]*sql.DB)
	return lastErr
}

// GetStats 获取连接池统计信息
func (m *ConnectionManager) GetStats(instanceID int64) (sql.DBStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[instanceID]
	if !exists {
		return sql.DBStats{}, fmt.Errorf("connection pool not found for instance %d", instanceID)
	}
	return pool.Stats(), nil
}

// 全局连接管理器实例
var globalConnManager = NewConnectionManager()

// GetGlobalConnectionManager 获取全局连接管理器
func GetGlobalConnectionManager() *ConnectionManager {
	return globalConnManager
}
