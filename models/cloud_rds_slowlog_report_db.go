// n9e-2kai: 云数据库 RDS 慢日志聚合报表模型
// 该表存储聚合后的慢日志数据，用于统计分析和前端展示
// 数据来源于 cloud_rds_slowlog_detail 表的聚合计算
package models

import (
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudRDSSlowLogReportDB 云数据库慢日志聚合报表（持久化表）
type CloudRDSSlowLogReportDB struct {
	Id           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountId    int64  `json:"account_id" gorm:"index"`
	RdsId        int64  `json:"rds_id" gorm:"index"`
	InstanceId   string `json:"instance_id" gorm:"type:varchar(64);index"`
	InstanceName string `json:"instance_name" gorm:"type:varchar(128)"`
	Provider     string `json:"provider" gorm:"type:varchar(32)"`
	Region       string `json:"region" gorm:"type:varchar(64)"`

	// SQL 信息
	SqlHash        string `json:"sql_hash" gorm:"type:varchar(32);index"`
	SqlFingerprint string `json:"sql_fingerprint" gorm:"type:text"`
	SqlType        string `json:"sql_type" gorm:"type:varchar(32);index"`
	Database       string `json:"database" gorm:"type:varchar(64);index"`
	SampleSql      string `json:"sample_sql" gorm:"type:text"`

	// 执行统计
	ExecuteCount  int64   `json:"execute_count"`
	TotalTime     float64 `json:"total_time"`
	AvgTime       float64 `json:"avg_time" gorm:"index"`
	MaxTime       float64 `json:"max_time"`
	MinTime       float64 `json:"min_time"`
	TotalLockTime float64 `json:"total_lock_time"`
	AvgLockTime   float64 `json:"avg_lock_time"`
	TotalRowsSent int64   `json:"total_rows_sent"`
	AvgRowsSent   int64   `json:"avg_rows_sent"`
	TotalRowsExam int64   `json:"total_rows_exam"`
	AvgRowsExam   int64   `json:"avg_rows_exam"`
	ExecuteRatio  float64 `json:"execute_ratio"` // 执行占比

	// 时间信息
	FirstSeenAt int64 `json:"first_seen_at" gorm:"index"`
	LastSeenAt  int64 `json:"last_seen_at" gorm:"index"`

	// 周期信息
	PeriodType  string `json:"period_type" gorm:"type:varchar(16);index"` // day, week, month
	PeriodStart int64  `json:"period_start" gorm:"index"`
	PeriodEnd   int64  `json:"period_end" gorm:"index"`

	// 创建时间
	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
}

func (CloudRDSSlowLogReportDB) TableName() string {
	return "cloud_rds_slowlog_report"
}

// CloudRDSSlowLogReportSummary 慢日志报告摘要
type CloudRDSSlowLogReportSummary struct {
	TotalUniqueQueries int64            `json:"total_unique_queries"`
	TotalExecutions    int64            `json:"total_executions"`
	AvgExecutionTime   float64          `json:"avg_execution_time"`
	TopSlowQueries     int64            `json:"top_slow_queries"` // 慢查询数量（>1s）
	ByType             map[string]int64 `json:"by_type"`
}

func (r *CloudRDSSlowLogReportDB) Add(c *ctx.Context) error {
	r.CreatedAt = 0 // 让 gorm 自动填充
	return DB(c).Create(r).Error
}

func (r *CloudRDSSlowLogReportDB) Update(c *ctx.Context) error {
	return DB(c).Save(r).Error
}

// CloudRDSSlowLogReportDBBatchUpsert 批量插入或更新聚合报表
func CloudRDSSlowLogReportDBBatchUpsert(c *ctx.Context, reports []CloudRDSSlowLogReportDB) error {
	if len(reports) == 0 {
		return nil
	}

	// 使用 batch insert，如遇冲突则更新
	for i := 0; i < len(reports); i += 100 {
		end := i + 100
		if end > len(reports) {
			end = len(reports)
		}
		if err := DB(c).Create(reports[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

// CloudRDSSlowLogReportDBDeleteByPeriod 删除指定周期的报表数据
func CloudRDSSlowLogReportDBDeleteByPeriod(c *ctx.Context, instanceId string, periodType string, periodStart int64) error {
	return DB(c).Where("instance_id = ? AND period_type = ? AND period_start = ?", instanceId, periodType, periodStart).
		Delete(&CloudRDSSlowLogReportDB{}).Error
}

// CloudRDSSlowLogReportDBGetByPeriod 获取指定周期的报表数据
func CloudRDSSlowLogReportDBGetByPeriod(c *ctx.Context, instanceId, database, sqlType, periodType string, periodStart, periodEnd int64, sortField, sortOrder string, limit, offset int) ([]CloudRDSSlowLogReportDB, int64, error) {
	var reports []CloudRDSSlowLogReportDB
	var total int64

	session := DB(c).Model(&CloudRDSSlowLogReportDB{})

	if periodType != "" {
		session = session.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		session = session.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		session = session.Where("period_end <= ?", periodEnd)
	}
	if instanceId != "" {
		session = session.Where("instance_id = ?", instanceId)
	}
	if database != "" {
		session = session.Where("database = ?", database)
	}
	if sqlType != "" {
		session = session.Where("sql_type = ?", sqlType)
	}

	// Count
	session.Count(&total)

	// 排序
	if sortField == "" {
		sortField = "execute_count"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}
	session = session.Order(sortField + " " + sortOrder)

	// 分页
	if limit > 0 {
		session = session.Limit(limit)
	}
	if offset > 0 {
		session = session.Offset(offset)
	}

	err := session.Find(&reports).Error
	return reports, total, err
}

// CloudRDSSlowLogReportDBGetSummary 获取报表摘要
func CloudRDSSlowLogReportDBGetSummary(c *ctx.Context, instanceId, database, periodType string, periodStart, periodEnd int64) (*CloudRDSSlowLogReportSummary, error) {
	summary := &CloudRDSSlowLogReportSummary{
		ByType: make(map[string]int64),
	}

	// 构建基础查询条件
	baseSession := DB(c).Model(&CloudRDSSlowLogReportDB{})
	if instanceId != "" {
		baseSession = baseSession.Where("instance_id = ?", instanceId)
	}
	if database != "" {
		baseSession = baseSession.Where("database = ?", database)
	}
	if periodType != "" {
		baseSession = baseSession.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		baseSession = baseSession.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		baseSession = baseSession.Where("period_end <= ?", periodEnd)
	}

	// 统计唯一查询数和总执行次数
	var stats struct {
		UniqueQueries   int64
		TotalExecutions int64
		SumTime         float64
	}
	statSession := DB(c).Model(&CloudRDSSlowLogReportDB{})
	if instanceId != "" {
		statSession = statSession.Where("instance_id = ?", instanceId)
	}
	if database != "" {
		statSession = statSession.Where("database = ?", database)
	}
	if periodType != "" {
		statSession = statSession.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		statSession = statSession.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		statSession = statSession.Where("period_end <= ?", periodEnd)
	}
	statSession.Select("COUNT(DISTINCT sql_hash) as unique_queries, SUM(execute_count) as total_executions, SUM(total_time) as sum_time").Scan(&stats)

	summary.TotalUniqueQueries = stats.UniqueQueries
	summary.TotalExecutions = stats.TotalExecutions
	if stats.TotalExecutions > 0 {
		summary.AvgExecutionTime = stats.SumTime / float64(stats.TotalExecutions)
	}

	// 统计慢查询数量（avg_time > 1s）
	slowSession := DB(c).Model(&CloudRDSSlowLogReportDB{}).Where("avg_time > 1")
	if instanceId != "" {
		slowSession = slowSession.Where("instance_id = ?", instanceId)
	}
	if database != "" {
		slowSession = slowSession.Where("database = ?", database)
	}
	if periodType != "" {
		slowSession = slowSession.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		slowSession = slowSession.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		slowSession = slowSession.Where("period_end <= ?", periodEnd)
	}
	slowSession.Count(&summary.TopSlowQueries)

	// 按类型统计
	var typeCounts []struct {
		SqlType string
		Count   int64
	}
	typeSession := DB(c).Model(&CloudRDSSlowLogReportDB{})
	if instanceId != "" {
		typeSession = typeSession.Where("instance_id = ?", instanceId)
	}
	if database != "" {
		typeSession = typeSession.Where("database = ?", database)
	}
	if periodType != "" {
		typeSession = typeSession.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		typeSession = typeSession.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		typeSession = typeSession.Where("period_end <= ?", periodEnd)
	}
	typeSession.Select("sql_type, COUNT(DISTINCT sql_hash) as count").Group("sql_type").Find(&typeCounts)

	for _, tc := range typeCounts {
		summary.ByType[tc.SqlType] = tc.Count
	}

	return summary, nil
}

// CloudRDSSlowLogReportDBGetDatabases 获取报表中涉及的数据库列表
func CloudRDSSlowLogReportDBGetDatabases(c *ctx.Context, instanceId, periodType string, periodStart, periodEnd int64) ([]string, error) {
	var databases []string
	session := DB(c).Model(&CloudRDSSlowLogReportDB{})
	if instanceId != "" {
		session = session.Where("instance_id = ?", instanceId)
	}
	if periodType != "" {
		session = session.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		session = session.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		session = session.Where("period_end <= ?", periodEnd)
	}
	session.Distinct().Pluck("database", &databases)
	return databases, nil
}

// CloudRDSSlowLogReportDBGetInstances 获取有慢日志数据的实例列表
func CloudRDSSlowLogReportDBGetInstances(c *ctx.Context, periodType string, periodStart, periodEnd int64) ([]struct {
	InstanceId   string `json:"instance_id"`
	InstanceName string `json:"instance_name"`
}, error) {
	var instances []struct {
		InstanceId   string `json:"instance_id"`
		InstanceName string `json:"instance_name"`
	}
	session := DB(c).Model(&CloudRDSSlowLogReportDB{})
	if periodType != "" {
		session = session.Where("period_type = ?", periodType)
	}
	if periodStart > 0 {
		session = session.Where("period_start >= ?", periodStart)
	}
	if periodEnd > 0 {
		session = session.Where("period_end <= ?", periodEnd)
	}
	session.Select("DISTINCT instance_id, instance_name").Find(&instances)
	return instances, nil
}

// CloudRDSSlowLogReportDBCleanup 清理旧的报表数据
func CloudRDSSlowLogReportDBCleanup(c *ctx.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoffTime := int64(retentionDays * 86400)
	now := time.Now().Unix()
	cutoffTime = now - cutoffTime

	result := DB(c).Where("period_end < ?", cutoffTime).Delete(&CloudRDSSlowLogReportDB{})
	return result.RowsAffected, result.Error
}
