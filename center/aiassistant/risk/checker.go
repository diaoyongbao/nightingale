// Package risk 风险检查器
// n9e-2kai: AI 助手模块 - 风险检查器
package risk

import (
	"regexp"
	"strings"
)

// Level 风险等级
type Level string

const (
	LevelLow    Level = "low"
	LevelMedium Level = "medium"
	LevelHigh   Level = "high"
)

// Checker 风险检查器
type Checker struct {
	config *Config
}

// Config 风险检查配置
type Config struct {
	// SQL 相关
	SQLWriteKeywords   []string // 写操作关键字
	SQLDangerKeywords  []string // 危险操作关键字
	SQLMaxRowsNoLimit  int      // SELECT 无 LIMIT 时的最大行数阈值

	// 告警屏蔽相关
	AlertMuteMaxMatch  int // 屏蔽规则最大匹配数阈值
	AlertMuteMaxHours  int // 屏蔽规则最大时长（小时）

	// 通用
	BatchOperationThreshold int // 批量操作阈值
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		SQLWriteKeywords: []string{
			"INSERT", "UPDATE", "DELETE", "REPLACE",
			"TRUNCATE", "ALTER", "DROP", "CREATE",
			"GRANT", "REVOKE", "SET", "CALL", "LOAD",
		},
		SQLDangerKeywords: []string{
			"DROP", "TRUNCATE", "DELETE FROM", "ALTER TABLE",
		},
		SQLMaxRowsNoLimit:       1000,
		AlertMuteMaxMatch:       50,
		AlertMuteMaxHours:       24,
		BatchOperationThreshold: 10,
	}
}

// NewChecker 创建风险检查器
func NewChecker(config *Config) *Checker {
	if config == nil {
		config = DefaultConfig()
	}
	return &Checker{config: config}
}

// CheckSQL 检查 SQL 风险
func (c *Checker) CheckSQL(sql string) Level {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))

	// 检查是否包含多条语句
	if c.isMultiStatement(sql) {
		return LevelHigh
	}

	// 检查危险关键字
	for _, keyword := range c.config.SQLDangerKeywords {
		if strings.Contains(upperSQL, keyword) {
			return LevelHigh
		}
	}

	// 检查写操作关键字
	for _, keyword := range c.config.SQLWriteKeywords {
		// 使用正则确保是完整的关键字匹配
		pattern := regexp.MustCompile(`\b` + keyword + `\b`)
		if pattern.MatchString(upperSQL) {
			return LevelHigh
		}
	}

	// 检查 SELECT 是否有 LIMIT
	if strings.HasPrefix(upperSQL, "SELECT") && !strings.Contains(upperSQL, "LIMIT") {
		return LevelMedium
	}

	// 只读操作
	if c.isReadOnlySQL(upperSQL) {
		return LevelLow
	}

	return LevelLow
}

// CheckOperation 检查通用操作风险
func (c *Checker) CheckOperation(operation string, params map[string]interface{}) Level {
	upperOp := strings.ToUpper(operation)

	// 写操作/状态变更
	writeOps := []string{"CREATE", "UPDATE", "DELETE", "EXEC", "SCALE", "PUT", "PUSH", "MODIFY"}
	for _, op := range writeOps {
		if strings.Contains(upperOp, op) {
			return LevelHigh
		}
	}

	// 检查批量操作
	if c.isBatchOperation(params) {
		return LevelHigh
	}

	// 检查目标范围是否明确
	if !c.hasSpecificTarget(params) {
		return LevelMedium
	}

	return LevelLow
}

// CheckAlertMute 检查告警屏蔽风险
func (c *Checker) CheckAlertMute(tags map[string]string, matchCount int, durationHours int) Level {
	// 范围过宽
	if len(tags) == 0 {
		return LevelHigh
	}

	// 匹配数过多
	if matchCount > c.config.AlertMuteMaxMatch {
		return LevelHigh
	}

	// 时间跨度过长
	if durationHours > c.config.AlertMuteMaxHours {
		return LevelHigh
	}

	// 检查是否有过宽的正则
	for _, v := range tags {
		if v == ".*" || v == ".+" || v == "" {
			return LevelHigh
		}
	}

	return LevelLow
}

// CheckK8sOperation 检查 K8s 操作风险
func (c *Checker) CheckK8sOperation(operation string, namespace string, resourceName string) Level {
	upperOp := strings.ToUpper(operation)

	// 高风险操作
	highRiskOps := []string{"EXEC", "SCALE", "DELETE", "DRAIN", "CORDON"}
	for _, op := range highRiskOps {
		if strings.Contains(upperOp, op) {
			// 必须指定 namespace 和资源名
			if namespace == "" || resourceName == "" {
				return LevelHigh
			}
			return LevelHigh // exec/scale 即使指定了也需要确认
		}
	}

	// 中等风险：获取日志（可能敏感）
	if strings.Contains(upperOp, "LOG") {
		return LevelMedium
	}

	// 低风险：只读操作
	if strings.Contains(upperOp, "LIST") || strings.Contains(upperOp, "GET") || strings.Contains(upperOp, "DESCRIBE") {
		// 如果是 namespace=all 或跨多 namespace，升级为中等
		if namespace == "" || namespace == "all" {
			return LevelMedium
		}
		return LevelLow
	}

	return LevelLow
}

// isMultiStatement 检查是否是多语句
func (c *Checker) isMultiStatement(sql string) bool {
	// 移除字符串字面量中的分号
	cleaned := regexp.MustCompile(`'[^']*'`).ReplaceAllString(sql, "")
	cleaned = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(cleaned, "")

	// 检查是否有多个非空语句
	statements := strings.Split(cleaned, ";")
	count := 0
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) != "" {
			count++
		}
	}
	return count > 1
}

// isReadOnlySQL 检查是否是只读 SQL
func (c *Checker) isReadOnlySQL(upperSQL string) bool {
	readOnlyPrefixes := []string{"SELECT", "SHOW", "EXPLAIN", "DESC", "DESCRIBE", "WITH"}
	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(upperSQL, prefix) {
			return true
		}
	}
	return false
}

// isBatchOperation 检查是否是批量操作
func (c *Checker) isBatchOperation(params map[string]interface{}) bool {
	// 检查是否有批量参数
	batchKeys := []string{"ids", "names", "targets", "resources"}
	for _, key := range batchKeys {
		if val, ok := params[key]; ok {
			switch v := val.(type) {
			case []interface{}:
				if len(v) > c.config.BatchOperationThreshold {
					return true
				}
			case []string:
				if len(v) > c.config.BatchOperationThreshold {
					return true
				}
			case []int64:
				if len(v) > c.config.BatchOperationThreshold {
					return true
				}
			}
		}
	}

	// 检查通配符
	if val, ok := params["name"]; ok {
		if str, ok := val.(string); ok {
			if str == "*" || str == "all" || strings.Contains(str, "*") {
				return true
			}
		}
	}

	return false
}

// hasSpecificTarget 检查是否有明确的目标
func (c *Checker) hasSpecificTarget(params map[string]interface{}) bool {
	// 必须有以下至少一个参数
	targetKeys := []string{"id", "name", "namespace", "resource", "target"}
	for _, key := range targetKeys {
		if val, ok := params[key]; ok {
			if str, ok := val.(string); ok && str != "" && str != "*" && str != "all" {
				return true
			}
			if num, ok := val.(int64); ok && num > 0 {
				return true
			}
		}
	}
	return false
}

// Result 风险检查结果
type Result struct {
	Level       Level  `json:"level"`
	Reason      string `json:"reason"`
	Suggestion  string `json:"suggestion"`
	NeedConfirm bool   `json:"need_confirm"`
}

// CheckSQLWithResult 检查 SQL 并返回详细结果
func (c *Checker) CheckSQLWithResult(sql string) *Result {
	level := c.CheckSQL(sql)
	result := &Result{
		Level:       level,
		NeedConfirm: level == LevelHigh,
	}

	switch level {
	case LevelHigh:
		result.Reason = "检测到高风险 SQL 操作（写操作/危险操作/多语句）"
		result.Suggestion = "请确认操作内容和影响范围后再执行"
	case LevelMedium:
		result.Reason = "SELECT 语句未指定 LIMIT，可能返回大量数据"
		result.Suggestion = "建议添加 LIMIT 限制返回行数"
	default:
		result.Reason = "只读操作，风险较低"
	}

	return result
}
