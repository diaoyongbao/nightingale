package dbm

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ArcherySQLQueryRequest SQL查询请求
type ArcherySQLQueryRequest struct {
	InstanceID int    `json:"instance_id"`
	DBName     string `json:"db_name"`
	SQL        string `json:"sql"`
	Limit      int    `json:"limit,omitempty"` // 默认1000
}

// ArcherySQLQueryResponse SQL查询响应
type ArcherySQLQueryResponse struct {
	Status int                    `json:"status"`
	Msg    string                 `json:"msg"`
	Data   map[string]interface{} `json:"data"` // 包含rows, column_list等
}

// ExecuteQuery 执行SQL查询
func (c *ArcheryClient) ExecuteQuery(req ArcherySQLQueryRequest) (*ArcherySQLQueryResponse, error) {
	// Archery API路径: /api/v1/sql/query/
	url := fmt.Sprintf("%s/api/v1/sql/query/", c.config.Address)

	// 设置默认limit
	if req.Limit == 0 {
		req.Limit = 1000
	}

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
	var result ArcherySQLQueryResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("query failed: %s", result.Msg)
	}

	return &result, nil
}

// ArcherySQLCheckRequest SQL检测请求
type ArcherySQLCheckRequest struct {
	InstanceID int    `json:"instance_id"`
	DBName     string `json:"db_name"`
	SQL        string `json:"sql"`
}

// ArcherySQLCheckResponse SQL检测响应
type ArcherySQLCheckResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Data   struct {
		CheckResult  string `json:"check_result"`  // 检测结果
		AffectedRows int64  `json:"affected_rows"` // 影响行数
		SQLType      string `json:"sql_type"`      // SQL类型:SELECT/UPDATE/DELETE等
		Warnings     []struct {
			Level   string `json:"level"`   // warning/error
			Message string `json:"message"` // 告警信息
		} `json:"warnings"`
	} `json:"data"`
}

// CheckSQL SQL语法检测
func (c *ArcheryClient) CheckSQL(req ArcherySQLCheckRequest) (*ArcherySQLCheckResponse, error) {
	// Archery API路径: /api/v1/sql/check/
	url := fmt.Sprintf("%s/api/v1/sql/check/", c.config.Address)

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
	var result ArcherySQLCheckResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("check failed: %s", result.Msg)
	}

	return &result, nil
}

// ArcherySQLWorkflowRequest SQL工单提交请求
type ArcherySQLWorkflowRequest struct {
	InstanceID int    `json:"instance_id"`
	DBName     string `json:"db_name"`
	SQL        string `json:"sql"`
	Title      string `json:"title"`                 // 工单标题
	Reason     string `json:"reason"`                // 变更原因
	BackupType string `json:"backup_type,omitempty"` // 备份类型:none/full
}

// ArcherySQLWorkflowResponse SQL工单响应
type ArcherySQLWorkflowResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Data   struct {
		WorkflowID int64  `json:"workflow_id"` // 工单ID
		Status     string `json:"status"`      // 工单状态
	} `json:"data"`
}

// SubmitSQLWorkflow 提交SQL工单
func (c *ArcheryClient) SubmitSQLWorkflow(req ArcherySQLWorkflowRequest) (*ArcherySQLWorkflowResponse, error) {
	// Archery API路径: /api/v1/workflow/
	url := fmt.Sprintf("%s/api/v1/workflow/", c.config.Address)

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
	var result ArcherySQLWorkflowResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("submit workflow failed: %s", result.Msg)
	}

	return &result, nil
}
