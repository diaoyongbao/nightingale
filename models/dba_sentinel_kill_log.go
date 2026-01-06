package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// DBASentinelKillLog DBA 哨兵 Kill 日志
type DBASentinelKillLog struct {
	Id           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	RuleId       int64  `json:"rule_id" gorm:"not null;index"`          // 关联规则 ID
	RuleName     string `json:"rule_name" gorm:"type:varchar(255)"`     // 规则名称 (冗余)
	InstanceId   int64  `json:"instance_id" gorm:"not null;index"`      // 实例 ID
	InstanceName string `json:"instance_name" gorm:"type:varchar(255)"` // 实例名称 (冗余)

	// 会话信息
	ThreadId int64  `json:"thread_id" gorm:"not null"`       // 线程 ID
	User     string `json:"user" gorm:"type:varchar(100)"`   // 用户
	Host     string `json:"host" gorm:"type:varchar(200)"`   // 主机
	Db       string `json:"db" gorm:"type:varchar(100)"`     // 数据库
	Command  string `json:"command" gorm:"type:varchar(50)"` // 命令类型
	Time     int    `json:"time"`                            // 执行时长 (秒)
	State    string `json:"state" gorm:"type:varchar(100)"`  // 状态
	SqlText  string `json:"sql_text" gorm:"type:text"`       // SQL 语句

	// 事务信息 (如果是未提交事务)
	TrxId      string `json:"trx_id" gorm:"type:varchar(100)"`     // 事务 ID
	TrxStarted string `json:"trx_started" gorm:"type:varchar(50)"` // 事务开始时间

	// Kill 信息
	KillReason string `json:"kill_reason" gorm:"type:varchar(500)"` // Kill 原因
	KillResult string `json:"kill_result" gorm:"type:varchar(50)"`  // 结果: success, failed
	ErrorMsg   string `json:"error_msg" gorm:"type:text"`           // 错误信息

	// 通知信息
	NotifyStatus  string `json:"notify_status" gorm:"type:varchar(50)"` // 通知状态: sent, failed, skipped
	NotifyMessage string `json:"notify_message" gorm:"type:text"`       // 通知消息

	// 时间戳
	CreatedAt int64 `json:"created_at" gorm:"not null;index"`
}

func (DBASentinelKillLog) TableName() string {
	return "dba_sentinel_kill_log"
}

// Kill 结果常量
const (
	KillResultSuccess = "success"
	KillResultFailed  = "failed"
)

// 通知状态常量
const (
	NotifyStatusSent    = "sent"
	NotifyStatusFailed  = "failed"
	NotifyStatusSkipped = "skipped"
)

// Add 添加日志
func (l *DBASentinelKillLog) Add(ctx *ctx.Context) error {
	if l.CreatedAt == 0 {
		l.CreatedAt = time.Now().Unix()
	}
	return Insert(ctx, l)
}

// DBASentinelKillLogGets 获取日志列表
func DBASentinelKillLogGets(ctx *ctx.Context, ruleId, instanceId int64, startTime, endTime int64, limit, offset int) ([]DBASentinelKillLog, error) {
	var logs []DBASentinelKillLog
	session := DB(ctx).Order("id desc")

	if ruleId > 0 {
		session = session.Where("rule_id = ?", ruleId)
	}
	if instanceId > 0 {
		session = session.Where("instance_id = ?", instanceId)
	}
	if startTime > 0 {
		session = session.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		session = session.Where("created_at <= ?", endTime)
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&logs).Error
	return logs, err
}

// DBASentinelKillLogCount 获取日志总数
func DBASentinelKillLogCount(ctx *ctx.Context, ruleId, instanceId int64, startTime, endTime int64) (int64, error) {
	session := DB(ctx).Model(&DBASentinelKillLog{})

	if ruleId > 0 {
		session = session.Where("rule_id = ?", ruleId)
	}
	if instanceId > 0 {
		session = session.Where("instance_id = ?", instanceId)
	}
	if startTime > 0 {
		session = session.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		session = session.Where("created_at <= ?", endTime)
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// DBASentinelKillLogGetsByRuleId 根据规则 ID 获取日志
func DBASentinelKillLogGetsByRuleId(ctx *ctx.Context, ruleId int64, limit int) ([]DBASentinelKillLog, error) {
	var logs []DBASentinelKillLog
	session := DB(ctx).Where("rule_id = ?", ruleId).Order("id desc")
	if limit > 0 {
		session = session.Limit(limit)
	}
	err := session.Find(&logs).Error
	return logs, err
}

// DBASentinelKillLogDel 删除日志
func DBASentinelKillLogDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(new(DBASentinelKillLog)).Error
}

// DBASentinelKillLogCleanup 清理过期日志
func DBASentinelKillLogCleanup(ctx *ctx.Context, beforeTime int64) (int64, error) {
	result := DB(ctx).Where("created_at < ?", beforeTime).Delete(new(DBASentinelKillLog))
	return result.RowsAffected, result.Error
}

// GetStatsByRuleId 获取规则统计信息
func DBASentinelKillLogStatsByRuleId(ctx *ctx.Context, ruleId int64, days int) (map[string]int64, error) {
	stats := make(map[string]int64)

	// 计算时间范围
	now := time.Now()
	startTime := now.AddDate(0, 0, -days).Unix()

	// 总数
	var total int64
	if err := DB(ctx).Model(&DBASentinelKillLog{}).
		Where("rule_id = ? AND created_at >= ?", ruleId, startTime).
		Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 成功数
	var success int64
	if err := DB(ctx).Model(&DBASentinelKillLog{}).
		Where("rule_id = ? AND created_at >= ? AND kill_result = ?", ruleId, startTime, KillResultSuccess).
		Count(&success).Error; err != nil {
		return nil, err
	}
	stats["success"] = success

	// 失败数
	var failed int64
	if err := DB(ctx).Model(&DBASentinelKillLog{}).
		Where("rule_id = ? AND created_at >= ? AND kill_result = ?", ruleId, startTime, KillResultFailed).
		Count(&failed).Error; err != nil {
		return nil, err
	}
	stats["failed"] = failed

	return stats, nil
}
