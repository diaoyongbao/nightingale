package dbm

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ArcherySlowQueryRequest 慢查询请求
type ArcherySlowQueryRequest struct {
	InstanceID int    `json:"instance_id"`
	StartTime  string `json:"start_time,omitempty"` // 格式: 2024-01-01 00:00:00
	EndTime    string `json:"end_time,omitempty"`
	Schema     string `json:"schema,omitempty"` // 数据库过滤
	Limit      int    `json:"limit,omitempty"`  // 默认100
}

// ArcherySlowQuery 慢查询记录
type ArcherySlowQuery struct {
	ID              int64   `json:"id"`
	Checksum        string  `json:"checksum"`          // SQL指纹hash
	Fingerprint     string  `json:"fingerprint"`       // SQL指纹
	Sample          string  `json:"sample"`            // 示例SQL
	DBMax           string  `json:"db_max"`            // 数据库名
	QueryTimeAvg    float64 `json:"query_time_avg"`    // 平均执行时间
	QueryTimeMax    float64 `json:"query_time_max"`    // 最大执行时间
	QueryTimeMin    float64 `json:"query_time_min"`    // 最小执行时间
	LockTimeAvg     float64 `json:"lock_time_avg"`     // 平均锁时间
	RowsSentAvg     int64   `json:"rows_sent_avg"`     // 平均返回行数
	RowsExaminedAvg int64   `json:"rows_examined_avg"` // 平均扫描行数
	TsMin           string  `json:"ts_min"`            // 首次出现时间
	TsMax           string  `json:"ts_max"`            // 最后出现时间
	TsCnt           int     `json:"ts_cnt"`            // 出现次数
}

// ArcherySlowQueryListResponse 慢查询列表响应
type ArcherySlowQueryListResponse struct {
	Status int                `json:"status"`
	Msg    string             `json:"msg"`
	Data   []ArcherySlowQuery `json:"data"`
}

// GetSlowQueries 获取慢查询列表
func (c *ArcheryClient) GetSlowQueries(req ArcherySlowQueryRequest) ([]ArcherySlowQuery, error) {
	// Archery API路径: /api/v1/instance/{id}/slow_query_review/
	url := fmt.Sprintf("%s/api/v1/instance/%d/slow_query_review/", c.config.Address, req.InstanceID)

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
	var result ArcherySlowQueryListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("archery api error: %s", result.Msg)
	}

	return result.Data, nil
}

// ArcherySlowQueryDetail 慢查询详情
type ArcherySlowQueryDetail struct {
	ID             int64                  `json:"id"`
	Checksum       string                 `json:"checksum"`
	Fingerprint    string                 `json:"fingerprint"`
	Sample         string                 `json:"sample"`
	Statistics     map[string]interface{} `json:"statistics"`      // 统计信息
	ExplainPlan    string                 `json:"explain_plan"`    // 执行计划
	OptimizeAdvice string                 `json:"optimize_advice"` // 优化建议(来自SQLAdvisor/SOAR)
}

// GetSlowQueryDetail 获取慢查询详情
func (c *ArcheryClient) GetSlowQueryDetail(instanceID int, checksum string) (*ArcherySlowQueryDetail, error) {
	// Archery API路径: /api/v1/instance/{id}/slow_query_review/{checksum}/
	url := fmt.Sprintf("%s/api/v1/instance/%d/slow_query_review/%s/", c.config.Address, instanceID, checksum)

	// 发起请求
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var result struct {
		Status int                     `json:"status"`
		Msg    string                  `json:"msg"`
		Data   *ArcherySlowQueryDetail `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != 0 {
		return nil, fmt.Errorf("archery api error: %s", result.Msg)
	}

	return result.Data, nil
}
