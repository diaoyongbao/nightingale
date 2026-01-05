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
	url := fmt.Sprintf("%s/api/v1/instance/%d/processlist/", c.config.Address, req.InstanceID)

	// 构造请求body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发起请求
	resp, err := c.doRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 解析响应
	var result ArcherySessionListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("archery api error: %s", result.Msg)
	}

	return result.Data, nil
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
