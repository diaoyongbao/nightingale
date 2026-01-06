package dbm

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ArcherySessionListRequest 会话列表请求
type ArcherySessionListRequest struct {
	InstanceID int    `json:"instance_id"`
	Schema     string `json:"schema,omitempty"`  // 可选:数据库过滤
	User       string `json:"user,omitempty"`    // 可选:用户过滤
	Command    string `json:"command,omitempty"` // 可选:命令类型过滤
}

// ArcherySession 会话信息
type ArcherySession struct {
	ID      int64  `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	Time    int    `json:"time"`   // 执行时间(秒)
	State   string `json:"state"`  // 状态
	Info    string `json:"info"`   // SQL语句
	TrxID   string `json:"trx_id"` // 事务ID(如果有)
}

// ArcherySessionListResponse 会话列表响应
type ArcherySessionListResponse struct {
	Status int              `json:"status"`
	Msg    string           `json:"msg"`
	Data   []ArcherySession `json:"data"`
}

// GetSessions 获取会话列表(processlist)
func (c *ArcheryClient) GetSessions(req ArcherySessionListRequest) ([]ArcherySession, error) {
	// Archery API路径: /api/v1/instance/{id}/processlist/
	// 注意：Archery 可能没有开放此 API，需要检查响应类型
	url := fmt.Sprintf("%s/api/v1/instance/%d/processlist/", c.config.Address, req.InstanceID)

	// 构造请求body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发起请求
	resp, err := c.doRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		// 检查是否返回了 HTML（通常是 404 页面）
		errStr := err.Error()
		if len(errStr) > 100 && (bytes.Contains([]byte(errStr), []byte("<!DOCTYPE")) || bytes.Contains([]byte(errStr), []byte("<html"))) {
			return nil, fmt.Errorf("Archery 不支持会话管理 API，请检查 Archery 版本或配置")
		}
		return nil, err
	}

	// 检查响应是否为 HTML
	if len(resp) > 0 && (resp[0] == '<' || bytes.HasPrefix(resp, []byte("<!DOCTYPE"))) {
		return nil, fmt.Errorf("Archery 返回了 HTML 页面，可能不支持此 API")
	}

	// 解析响应
	var result ArcherySessionListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, raw: %s", err, string(resp[:min(200, len(resp))]))
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("archery api error: %s", result.Msg)
	}

	return result.Data, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ArcheryKillSessionsRequest 批量Kill会话请求
type ArcheryKillSessionsRequest struct {
	InstanceID int     `json:"instance_id"`
	ThreadIDs  []int64 `json:"thread_ids"` // 进程ID列表
}

// KillSessions 批量Kill会话
func (c *ArcheryClient) KillSessions(req ArcheryKillSessionsRequest) error {
	// Archery API路径: /api/v1/instance/{id}/kill_session/
	url := fmt.Sprintf("%s/api/v1/instance/%d/kill_session/", c.config.Address, req.InstanceID)

	// 构造请求body
	body, err := json.Marshal(map[string]interface{}{
		"thread_ids": req.ThreadIDs,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发起请求
	resp, err := c.doRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	// 解析响应
	var result ArcheryResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return fmt.Errorf("kill sessions failed: %s", result.Msg)
	}

	return nil
}

// ArcheryUncommittedTrxRequest 未提交事务查询请求
type ArcheryUncommittedTrxRequest struct {
	InstanceID int `json:"instance_id"`
}

// ArcheryUncommittedTrx 未提交事务信息
type ArcheryUncommittedTrx struct {
	ProcesslistID int64  `json:"processlist_id"`
	KillCommand   string `json:"kill_command"`
	ThreadID      int64  `json:"thread_id"`
	TrxID         string `json:"trx_id"`
	SQLText       string `json:"sql_text"`
	TrxStarted    string `json:"trx_started"`
	RuntimeSec    int    `json:"runtime_sec"` // 运行时长(秒)
	User          string `json:"user"`
	DB            string `json:"db"`
}

// GetUncommittedTransactions 获取未提交事务列表
func (c *ArcheryClient) GetUncommittedTransactions(req ArcheryUncommittedTrxRequest) ([]ArcheryUncommittedTrx, error) {
	// Archery API路径: /api/v1/instance/{id}/trx_status/
	url := fmt.Sprintf("%s/api/v1/instance/%d/trx_status/", c.config.Address, req.InstanceID)

	// 发起请求
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var result struct {
		Status int                     `json:"status"`
		Msg    string                  `json:"msg"`
		Data   []ArcheryUncommittedTrx `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("archery api error: %s", result.Msg)
	}

	return result.Data, nil
}

// ArcheryLockWaitRequest 锁等待查询请求
type ArcheryLockWaitRequest struct {
	InstanceID int `json:"instance_id"`
}

// ArcheryLockWait 锁等待信息
type ArcheryLockWait struct {
	// 等待锁的进程信息
	WaitingThreadID int64  `json:"waiting_thread_id"`
	WaitingQuery    string `json:"waiting_query"`
	WaitingUser     string `json:"waiting_user"`
	WaitingHost     string `json:"waiting_host"`
	WaitingDB       string `json:"waiting_db"`
	WaitingTime     int    `json:"waiting_time"` // 等待时间(秒)

	// 持有锁的进程信息
	BlockingThreadID int64  `json:"blocking_thread_id"`
	BlockingQuery    string `json:"blocking_query"`
	BlockingUser     string `json:"blocking_user"`
	BlockingHost     string `json:"blocking_host"`
	BlockingDB       string `json:"blocking_db"`
	BlockingTime     int    `json:"blocking_time"` // 持有时间(秒)

	// 锁信息
	LockType  string `json:"lock_type"`  // 锁类型: RECORD, TABLE
	LockMode  string `json:"lock_mode"`  // 锁模式: S, X, IS, IX
	LockTable string `json:"lock_table"` // 锁定的表
	LockIndex string `json:"lock_index"` // 锁定的索引
	LockData  string `json:"lock_data"`  // 锁定的数据
}

// GetLockWaits 获取锁等待信息
// 注意: Archery 可能不直接支持此 API，这里通过 SQL 查询实现
func (c *ArcheryClient) GetLockWaits(req ArcheryLockWaitRequest) ([]ArcheryLockWait, error) {
	// 尝试调用 Archery 的锁等待 API
	// 如果 Archery 没有此 API，则返回空列表
	url := fmt.Sprintf("%s/api/v1/instance/%d/lock_waits/", c.config.Address, req.InstanceID)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		// API 不存在或请求失败，返回空列表
		return []ArcheryLockWait{}, nil
	}

	var result struct {
		Status int               `json:"status"`
		Msg    string            `json:"msg"`
		Data   []ArcheryLockWait `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return []ArcheryLockWait{}, nil
	}

	if result.Status != 0 {
		return []ArcheryLockWait{}, nil
	}

	return result.Data, nil
}

// ArcheryInnoDBLock InnoDB 锁信息
type ArcheryInnoDBLock struct {
	LockID    string `json:"lock_id"`
	LockTrxID string `json:"lock_trx_id"`
	LockMode  string `json:"lock_mode"` // S, X, IS, IX
	LockType  string `json:"lock_type"` // RECORD, TABLE
	LockTable string `json:"lock_table"`
	LockIndex string `json:"lock_index"`
	LockSpace int64  `json:"lock_space"`
	LockPage  int64  `json:"lock_page"`
	LockRec   int64  `json:"lock_rec"`
	LockData  string `json:"lock_data"`
	ThreadID  int64  `json:"thread_id"`
	User      string `json:"user"`
	Host      string `json:"host"`
	DB        string `json:"db"`
	Command   string `json:"command"`
	Time      int    `json:"time"`
	State     string `json:"state"`
	Query     string `json:"query"`
}

// GetInnoDBLocks 获取 InnoDB 锁信息
func (c *ArcheryClient) GetInnoDBLocks(instanceID int) ([]ArcheryInnoDBLock, error) {
	// 尝试调用 Archery 的锁信息 API
	url := fmt.Sprintf("%s/api/v1/instance/%d/innodb_locks/", c.config.Address, instanceID)

	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return []ArcheryInnoDBLock{}, nil
	}

	var result struct {
		Status int                 `json:"status"`
		Msg    string              `json:"msg"`
		Data   []ArcheryInnoDBLock `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return []ArcheryInnoDBLock{}, nil
	}

	if result.Status != 0 {
		return []ArcheryInnoDBLock{}, nil
	}

	return result.Data, nil
}
