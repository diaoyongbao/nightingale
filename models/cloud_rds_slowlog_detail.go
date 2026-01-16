// n9e-2kai: 云数据库 RDS 慢日志明细模型
// 该表存储从华为云同步的原始慢日志明细，可定期清理
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudRDSSlowLogDetail 云数据库慢日志明细（原始数据，可清理）
type CloudRDSSlowLogDetail struct {
	Id int64 `json:"id" gorm:"primaryKey;autoIncrement"`

	// 关联信息
	AccountId    int64  `json:"account_id" gorm:"index"`
	RdsId        int64  `json:"rds_id" gorm:"index"`      // 关联到 cloud_rds 表的 ID
	InstanceId   string `json:"instance_id" gorm:"index"` // RDS 实例 ID
	InstanceName string `json:"instance_name"`            // RDS 实例名称
	Provider     string `json:"provider" gorm:"type:varchar(32)"`
	Region       string `json:"region" gorm:"type:varchar(64)"`

	// 慢日志明细信息
	SqlText      string  `json:"sql_text" gorm:"type:text"`         // 执行语句
	SqlType      string  `json:"sql_type" gorm:"type:varchar(32)"`  // 语句类型: SELECT, INSERT, UPDATE, DELETE
	Database     string  `json:"database" gorm:"type:varchar(128)"` // 所属数据库
	ExecuteTime  float64 `json:"execute_time"`                      // 执行时间(s)
	LockTime     float64 `json:"lock_time"`                         // 等待锁时间(s)
	RowsSent     int64   `json:"rows_sent"`                         // 结果行数
	RowsExamined int64   `json:"rows_examined"`                     // 扫描行数
	Users        string  `json:"users" gorm:"type:varchar(128)"`    // 执行用户
	ClientIP     string  `json:"client_ip" gorm:"type:varchar(64)"` // 客户端IP

	// 时间信息
	ExecutedAt int64 `json:"executed_at" gorm:"index"` // 执行时间（Unix 时间戳）
	SyncTime   int64 `json:"sync_time" gorm:"index"`   // 同步时间
}

func (CloudRDSSlowLogDetail) TableName() string {
	return "cloud_rds_slowlog_detail"
}

// Add 添加慢日志明细记录
func (s *CloudRDSSlowLogDetail) Add(c *ctx.Context) error {
	s.SyncTime = time.Now().Unix()
	return Insert(c, s)
}

// CloudRDSSlowLogDetailBatchAdd 批量添加慢日志明细
func CloudRDSSlowLogDetailBatchAdd(c *ctx.Context, logs []CloudRDSSlowLogDetail) error {
	if len(logs) == 0 {
		return nil
	}

	now := time.Now().Unix()
	for i := range logs {
		logs[i].SyncTime = now
	}

	return DB(c).CreateInBatches(logs, 100).Error
}

// CloudRDSSlowLogDetailCleanup 清理指定天数之前的慢日志明细
func CloudRDSSlowLogDetailCleanup(c *ctx.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 7 // 默认保留 7 天
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).Unix()
	result := DB(c).Where("executed_at < ?", cutoffTime).Delete(&CloudRDSSlowLogDetail{})
	return result.RowsAffected, result.Error
}

// CloudRDSSlowLogDetailGetByTimeRange 获取指定时间范围的慢日志明细
func CloudRDSSlowLogDetailGetByTimeRange(c *ctx.Context, instanceId string, startTime, endTime int64) ([]CloudRDSSlowLogDetail, error) {
	var logs []CloudRDSSlowLogDetail
	session := DB(c).Where("executed_at >= ? AND executed_at <= ?", startTime, endTime)
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	err := session.Find(&logs).Error
	return logs, err
}

// CloudRDSSlowLogDetailCount 获取明细数量
func CloudRDSSlowLogDetailCount(c *ctx.Context, instanceId string, startTime, endTime int64) (int64, error) {
	var count int64
	session := DB(c).Model(&CloudRDSSlowLogDetail{})
	if startTime > 0 {
		session = session.Where("executed_at >= ?", startTime)
	}
	if endTime > 0 {
		session = session.Where("executed_at <= ?", endTime)
	}
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	err := session.Count(&count).Error
	return count, err
}

// CloudRDSSlowLogDetailExists 检查指定时间范围是否已有数据
func CloudRDSSlowLogDetailExists(c *ctx.Context, instanceId string, startTime, endTime int64) (bool, error) {
	var count int64
	err := DB(c).Model(&CloudRDSSlowLogDetail{}).
		Where("instance_id = ? AND executed_at >= ? AND executed_at <= ?", instanceId, startTime, endTime).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

// CloudRDSSlowLogDetailDeleteByTimeRange 删除指定时间范围的数据（用于重新同步）
func CloudRDSSlowLogDetailDeleteByTimeRange(c *ctx.Context, instanceId string, startTime, endTime int64) error {
	return DB(c).Where("instance_id = ? AND executed_at >= ? AND executed_at <= ?", instanceId, startTime, endTime).
		Delete(&CloudRDSSlowLogDetail{}).Error
}
