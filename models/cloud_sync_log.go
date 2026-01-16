// n9e-2kai: 云资源同步日志模型
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudSyncLog 云资源同步日志
type CloudSyncLog struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountId   int64  `json:"account_id" gorm:"not null;index"`
	AccountName string `json:"account_name" gorm:"type:varchar(128);not null"`
	Provider    string `json:"provider" gorm:"type:varchar(32);not null;index"`
	Region      string `json:"region" gorm:"type:varchar(64)"` // 空表示全部区域

	// 同步类型和范围
	SyncType      string `json:"sync_type" gorm:"type:varchar(32);not null;index"` // manual, auto
	ResourceTypes string `json:"resource_types" gorm:"type:varchar(256);not null"` // 逗号分隔: ecs,rds

	// 同步结果
	Status    int   `json:"status" gorm:"default:0;index"` // 0:进行中, 1:成功, 2:失败, 3:部分成功
	StartTime int64 `json:"start_time" gorm:"not null;index"`
	EndTime   int64 `json:"end_time" gorm:"default:0"`
	Duration  int   `json:"duration" gorm:"default:0"` // 秒

	// ECS 统计
	EcsTotal   int `json:"ecs_total" gorm:"default:0"`
	EcsAdded   int `json:"ecs_added" gorm:"default:0"`
	EcsUpdated int `json:"ecs_updated" gorm:"default:0"`
	EcsDeleted int `json:"ecs_deleted" gorm:"default:0"`

	// RDS 统计
	RdsTotal   int `json:"rds_total" gorm:"default:0"`
	RdsAdded   int `json:"rds_added" gorm:"default:0"`
	RdsUpdated int `json:"rds_updated" gorm:"default:0"`
	RdsDeleted int `json:"rds_deleted" gorm:"default:0"`

	// 统计信息摘要
	Summary string `json:"summary" gorm:"type:varchar(512)"`

	// 错误信息
	ErrorMessage string `json:"error_message,omitempty" gorm:"type:text"`
	ErrorDetails string `json:"error_details,omitempty" gorm:"type:text"` // JSON

	// 操作人
	Operator string `json:"operator" gorm:"type:varchar(64)"`
}

func (CloudSyncLog) TableName() string {
	return "cloud_sync_log"
}

// 同步类型
const (
	CloudSyncTypeManual = "manual"
	CloudSyncTypeAuto   = "auto"
)

// 同步日志状态
const (
	CloudSyncLogStatusRunning        = 0
	CloudSyncLogStatusSuccess        = 1
	CloudSyncLogStatusFailed         = 2
	CloudSyncLogStatusPartialSuccess = 3
)

// Add 添加同步日志
func (l *CloudSyncLog) Add(c *ctx.Context) error {
	return Insert(c, l)
}

// Update 更新同步日志
func (l *CloudSyncLog) Update(c *ctx.Context) error {
	return DB(c).Model(l).Updates(l).Error
}

// CloudSyncLogGet 根据 ID 获取同步日志
func CloudSyncLogGet(c *ctx.Context, id int64) (*CloudSyncLog, error) {
	var log CloudSyncLog
	err := DB(c).Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// CloudSyncLogGets 获取同步日志列表
func CloudSyncLogGets(c *ctx.Context, accountId int64, provider string, status *int, syncType string, startTime, endTime int64, limit, offset int) ([]CloudSyncLog, error) {
	var logs []CloudSyncLog
	session := DB(c).Order("id desc")

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if status != nil {
		session = session.Where("status = ?", *status)
	}

	if syncType != "" && syncType != "all" {
		session = session.Where("sync_type = ?", syncType)
	}

	if startTime > 0 {
		session = session.Where("start_time >= ?", startTime)
	}

	if endTime > 0 {
		session = session.Where("start_time <= ?", endTime)
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&logs).Error
	return logs, err
}

// CloudSyncLogCount 获取同步日志总数
func CloudSyncLogCount(c *ctx.Context, accountId int64, provider string, status *int, syncType string, startTime, endTime int64) (int64, error) {
	session := DB(c).Model(&CloudSyncLog{})

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if status != nil {
		session = session.Where("status = ?", *status)
	}

	if syncType != "" && syncType != "all" {
		session = session.Where("sync_type = ?", syncType)
	}

	if startTime > 0 {
		session = session.Where("start_time >= ?", startTime)
	}

	if endTime > 0 {
		session = session.Where("start_time <= ?", endTime)
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// CloudSyncLogDelByAccountId 删除指定账号的所有同步日志
func CloudSyncLogDelByAccountId(c *ctx.Context, accountId int64) error {
	return DB(c).Where("account_id = ?", accountId).Delete(new(CloudSyncLog)).Error
}

// CloudSyncLogComplete 完成同步日志
func (l *CloudSyncLog) Complete(c *ctx.Context, status int, errMsg string) error {
	l.Status = status
	l.EndTime = time.Now().Unix()
	l.Duration = int(l.EndTime - l.StartTime)
	l.ErrorMessage = errMsg
	return DB(c).Model(l).Select("status", "end_time", "duration", "error_message",
		"ecs_total", "ecs_added", "ecs_updated", "ecs_deleted",
		"rds_total", "rds_added", "rds_updated", "rds_deleted").Updates(l).Error
}

// CloudSyncLogGetLatest 获取账号最新的同步日志
func CloudSyncLogGetLatest(c *ctx.Context, accountId int64) (*CloudSyncLog, error) {
	var log CloudSyncLog
	err := DB(c).Where("account_id = ?", accountId).Order("id desc").First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// CloudSyncLogGetRunning 获取正在运行的同步任务
func CloudSyncLogGetRunning(c *ctx.Context, accountId int64) (*CloudSyncLog, error) {
	var log CloudSyncLog
	err := DB(c).Where("account_id = ? AND status = ?", accountId, CloudSyncLogStatusRunning).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// CloudSyncLogResetRunningStatus 重置所有"同步中"状态的日志为"失败"
// 服务启动时调用，处理异常中断的同步任务
func CloudSyncLogResetRunningStatus(c *ctx.Context) (int64, error) {
	now := time.Now().Unix()
	result := DB(c).Model(&CloudSyncLog{}).
		Where("status = ?", CloudSyncLogStatusRunning).
		Updates(map[string]interface{}{
			"status":        CloudSyncLogStatusFailed,
			"end_time":      now,
			"error_message": "服务重启，同步任务异常中断",
		})
	return result.RowsAffected, result.Error
}
