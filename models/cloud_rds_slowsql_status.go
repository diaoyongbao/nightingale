// n9e-2kai: 慢SQL优化跟踪状态关联表
// 通过 sql_hash 关联 cloud_rds_slowlog_report 表
package models

import (
	"fmt"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// 注意：状态常量和优先级常量定义在 cloud_rds_slowsql_tracking.go 中
// SlowSQLStatusPending, SlowSQLStatusAnalyzing, etc.
// SlowSQLPriorityHigh, SlowSQLPriorityMedium, SlowSQLPriorityLow

// CloudRDSSlowSQLStatus 慢SQL优化跟踪状态表（关联表）
type CloudRDSSlowSQLStatus struct {
	Id              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SqlHash         string `json:"sql_hash" gorm:"type:varchar(32);uniqueIndex;not null"` // 关联 cloud_rds_slowlog_report.sql_hash
	Status          string `json:"status" gorm:"type:varchar(32);index;default:'pending'"`
	Priority        string `json:"priority" gorm:"type:varchar(16);index;default:'medium'"`
	Owner           string `json:"owner" gorm:"type:varchar(64)"`
	OwnerEmail      string `json:"owner_email" gorm:"type:varchar(128)"`
	Team            string `json:"team" gorm:"type:varchar(128)"`
	OptimizeNote    string `json:"optimize_note" gorm:"type:text"`
	OptimizeResult  string `json:"optimize_result" gorm:"type:text"`
	StatusChangedAt int64  `json:"status_changed_at"`
	StatusChangedBy string `json:"status_changed_by" gorm:"type:varchar(64)"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
	// n9e-2kai: 优化判定逻辑增强字段
	LastSeenTime     int64   `json:"last_seen_time" gorm:"index"` // 最后出现时间（每日更新）
	FirstSeenTime    int64   `json:"first_seen_time"`             // 首次出现时间
	AvgFrequency     float64 `json:"avg_frequency"`               // 日均执行频率（滑动窗口30天）
	TotalOccurrences int64   `json:"total_occurrences"`           // 总出现天数
	Confidence       float64 `json:"confidence"`                  // 优化判定置信度 (0-1)
	ObservingSince   int64   `json:"observing_since"`             // 进入观察期的时间
}

func (CloudRDSSlowSQLStatus) TableName() string {
	return "cloud_rds_slowsql_status"
}

// CloudRDSSlowSQLStatusLog 状态变更日志
type CloudRDSSlowSQLStatusLog struct {
	Id        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SqlHash   string `json:"sql_hash" gorm:"type:varchar(32);index;not null"`
	OldStatus string `json:"old_status" gorm:"type:varchar(32)"`
	NewStatus string `json:"new_status" gorm:"type:varchar(32)"`
	Operator  string `json:"operator" gorm:"type:varchar(64)"`
	Comment   string `json:"comment" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CloudRDSSlowSQLStatusLog) TableName() string {
	return "cloud_rds_slowsql_status_log"
}

// ==================== CRUD 操作 ====================

func CloudRDSSlowSQLStatusGetBySqlHash(c *ctx.Context, sqlHash string) (*CloudRDSSlowSQLStatus, error) {
	var status CloudRDSSlowSQLStatus
	err := DB(c).Where("sql_hash = ?", sqlHash).First(&status).Error
	if err != nil {
		if err.Error() == "record not found" {
			return nil, nil
		}
		return nil, err
	}
	return &status, nil
}

func CloudRDSSlowSQLStatusUpsert(c *ctx.Context, sqlHash, statusVal, priority, owner, operator string) error {
	existing, err := CloudRDSSlowSQLStatusGetBySqlHash(c, sqlHash)
	if err != nil {
		return err
	}

	now := time.Now().Unix()

	if existing == nil {
		// 创建新记录
		status := &CloudRDSSlowSQLStatus{
			SqlHash:         sqlHash,
			Status:          statusVal,
			Priority:        priority,
			Owner:           owner,
			StatusChangedAt: now,
			StatusChangedBy: operator,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if status.Status == "" {
			status.Status = SlowSQLStatusPending
		}
		if status.Priority == "" {
			status.Priority = SlowSQLPriorityMedium
		}
		return DB(c).Create(status).Error
	}

	// 更新现有记录
	updates := map[string]interface{}{
		"updated_at": now,
	}
	if statusVal != "" && statusVal != existing.Status {
		updates["status"] = statusVal
		updates["status_changed_at"] = now
		updates["status_changed_by"] = operator
	}
	if priority != "" {
		updates["priority"] = priority
	}
	if owner != "" {
		updates["owner"] = owner
	}

	return DB(c).Model(&CloudRDSSlowSQLStatus{}).Where("id = ?", existing.Id).Updates(updates).Error
}

func CloudRDSSlowSQLStatusUpdateStatus(c *ctx.Context, sqlHash, newStatus, operator, comment string) error {
	existing, err := CloudRDSSlowSQLStatusGetBySqlHash(c, sqlHash)
	now := time.Now().Unix()

	if err != nil || existing == nil {
		// 不存在则创建
		status := &CloudRDSSlowSQLStatus{
			SqlHash:         sqlHash,
			Status:          newStatus,
			Priority:        SlowSQLPriorityMedium,
			StatusChangedAt: now,
			StatusChangedBy: operator,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		return DB(c).Create(status).Error
	}

	oldStatus := existing.Status

	// 更新状态
	err = DB(c).Model(&CloudRDSSlowSQLStatus{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
		"status":            newStatus,
		"status_changed_at": now,
		"status_changed_by": operator,
		"updated_at":        now,
	}).Error
	if err != nil {
		return err
	}

	// 记录日志
	log := &CloudRDSSlowSQLStatusLog{
		SqlHash:   sqlHash,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Operator:  operator,
		Comment:   comment,
		CreatedAt: now,
	}
	return DB(c).Create(log).Error
}

func CloudRDSSlowSQLStatusUpdate(c *ctx.Context, sqlHash string, updates map[string]interface{}, operator string) error {
	existing, err := CloudRDSSlowSQLStatusGetBySqlHash(c, sqlHash)
	now := time.Now().Unix()

	if err != nil || existing == nil {
		// 不存在则创建
		status := &CloudRDSSlowSQLStatus{
			SqlHash:   sqlHash,
			Status:    SlowSQLStatusPending,
			Priority:  SlowSQLPriorityMedium,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if v, ok := updates["status"].(string); ok {
			status.Status = v
		}
		if v, ok := updates["priority"].(string); ok {
			status.Priority = v
		}
		if v, ok := updates["owner"].(string); ok {
			status.Owner = v
		}
		if v, ok := updates["owner_email"].(string); ok {
			status.OwnerEmail = v
		}
		if v, ok := updates["team"].(string); ok {
			status.Team = v
		}
		if v, ok := updates["optimize_note"].(string); ok {
			status.OptimizeNote = v
		}
		if v, ok := updates["optimize_result"].(string); ok {
			status.OptimizeResult = v
		}
		return DB(c).Create(status).Error
	}

	updates["updated_at"] = now
	return DB(c).Model(&CloudRDSSlowSQLStatus{}).Where("id = ?", existing.Id).Updates(updates).Error
}

// ==================== JOIN 查询（核心）====================

// SlowSQLWithStatus 报表数据 + 跟踪状态的联合结构
type SlowSQLWithStatus struct {
	// 来自 cloud_rds_slowlog_report
	Id             int64   `json:"id"`
	InstanceId     string  `json:"instance_id"`
	InstanceName   string  `json:"instance_name"`
	SqlHash        string  `json:"sql_hash"`
	SqlFingerprint string  `json:"sql_fingerprint"`
	SqlType        string  `json:"sql_type"`
	Database       string  `json:"database"`
	SampleSql      string  `json:"sample_sql"`
	ExecuteCount   int64   `json:"execute_count"`
	AvgTime        float64 `json:"avg_time"`
	MaxTime        float64 `json:"max_time"`
	TotalTime      float64 `json:"total_time"`
	FirstSeenAt    int64   `json:"first_seen_at"`
	LastSeenAt     int64   `json:"last_seen_at"`
	PeriodType     string  `json:"period_type"`
	PeriodStart    int64   `json:"period_start"`
	// 来自 cloud_rds_slowsql_status（LEFT JOIN）
	Status          string  `json:"status"`
	Priority        string  `json:"priority"`
	Owner           string  `json:"owner"`
	OwnerEmail      string  `json:"owner_email"`
	Team            string  `json:"team"`
	OptimizeNote    string  `json:"optimize_note"`
	OptimizeResult  string  `json:"optimize_result"`
	StatusChangedAt int64   `json:"status_changed_at"`
	Confidence      float64 `json:"confidence"`      // n9e-2kai: 优化判定置信度
	AvgFrequency    float64 `json:"avg_frequency"`   // n9e-2kai: 日均频率
	ObservingSince  int64   `json:"observing_since"` // n9e-2kai: 进入观察期时间
}

// CloudRDSSlowSQLWithStatusGets 获取带跟踪状态的慢SQL列表
func CloudRDSSlowSQLWithStatusGets(c *ctx.Context, instanceId, status, priority, owner, query string, limit, offset int) ([]SlowSQLWithStatus, int64, error) {
	var list []SlowSQLWithStatus
	var total int64

	// 基础查询：从 report 表 LEFT JOIN status 表
	baseQuery := `
		SELECT 
			r.id, r.instance_id, r.instance_name, r.sql_hash, r.sql_fingerprint, 
			r.sql_type, r.database, r.sample_sql, r.execute_count, r.avg_time, 
			r.max_time, r.total_time, r.first_seen_at, r.last_seen_at,
			r.period_type, r.period_start,
			COALESCE(s.status, 'pending') as status,
			COALESCE(s.priority, 'medium') as priority,
			COALESCE(s.owner, '') as owner,
			COALESCE(s.owner_email, '') as owner_email,
			COALESCE(s.team, '') as team,
			COALESCE(s.optimize_note, '') as optimize_note,
			COALESCE(s.optimize_result, '') as optimize_result,
			COALESCE(s.status_changed_at, 0) as status_changed_at,
			COALESCE(s.confidence, 0) as confidence,
			COALESCE(s.avg_frequency, 0) as avg_frequency,
			COALESCE(s.observing_since, 0) as observing_since
		FROM cloud_rds_slowlog_report r
		LEFT JOIN cloud_rds_slowsql_status s ON r.sql_hash = s.sql_hash
		WHERE r.period_type = 'day'
	`

	countQuery := `
		SELECT COUNT(DISTINCT r.sql_hash)
		FROM cloud_rds_slowlog_report r
		LEFT JOIN cloud_rds_slowsql_status s ON r.sql_hash = s.sql_hash
		WHERE r.period_type = 'day'
	`

	args := []interface{}{}

	// 条件
	if instanceId != "" && instanceId != "all" {
		baseQuery += " AND r.instance_id = ?"
		countQuery += " AND r.instance_id = ?"
		args = append(args, instanceId)
	}
	if status != "" && status != "all" {
		if status == "pending" {
			baseQuery += " AND (s.status IS NULL OR s.status = 'pending')"
			countQuery += " AND (s.status IS NULL OR s.status = 'pending')"
		} else {
			baseQuery += " AND s.status = ?"
			countQuery += " AND s.status = ?"
			args = append(args, status)
		}
	}
	if priority != "" && priority != "all" {
		if priority == "medium" {
			baseQuery += " AND (s.priority IS NULL OR s.priority = 'medium')"
			countQuery += " AND (s.priority IS NULL OR s.priority = 'medium')"
		} else {
			baseQuery += " AND s.priority = ?"
			countQuery += " AND s.priority = ?"
			args = append(args, priority)
		}
	}
	if owner != "" && owner != "all" {
		// n9e-2kai: 通过 cloud_rds_owner 表关联筛选负责人
		baseQuery += " AND r.instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)"
		countQuery += " AND r.instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)"
		args = append(args, owner)
	}
	if query != "" {
		baseQuery += " AND (r.sql_fingerprint LIKE ? OR r.sql_hash LIKE ? OR r.database LIKE ?)"
		countQuery += " AND (r.sql_fingerprint LIKE ? OR r.sql_hash LIKE ? OR r.database LIKE ?)"
		args = append(args, "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	// 按 sql_hash 分组取最新
	baseQuery += " GROUP BY r.sql_hash ORDER BY r.last_seen_at DESC LIMIT ? OFFSET ?"

	// 计算 count 用的参数（不含 limit/offset）
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)

	// 添加 limit/offset
	args = append(args, limit, offset)

	// 执行 count
	DB(c).Raw(countQuery, countArgs...).Scan(&total)

	// 执行查询
	err := DB(c).Raw(baseQuery, args...).Scan(&list).Error

	return list, total, err
}

// ==================== 统计 ====================

type SlowSQLStatusStats struct {
	PendingCount        int64 `json:"pending_count"`
	UrgentCount         int64 `json:"urgent_count"`
	ObservingCount      int64 `json:"observing_count"` // n9e-2kai: 观察期中的数量
	OptimizedCount      int64 `json:"optimized_count"`
	IgnoredCount        int64 `json:"ignored_count"`
	TotalCount          int64 `json:"total_count"`
	ThisWeekNew         int64 `json:"this_week_new"`
	ThisWeekDone        int64 `json:"this_week_done"`
	HighPriorityPending int64 `json:"high_priority_pending"`
}

// CloudRDSSlowSQLStatusGetStats 获取慢SQL跟踪统计数据
// n9e-2kai: 增加 owner 参数支持按负责人筛选
func CloudRDSSlowSQLStatusGetStats(c *ctx.Context, instanceId, owner string) (*SlowSQLStatusStats, error) {
	stats := &SlowSQLStatusStats{}

	// 总数（report 表中的唯一 sql_hash 数量）
	totalQuery := "SELECT COUNT(DISTINCT sql_hash) FROM cloud_rds_slowlog_report WHERE period_type = 'day'"
	if instanceId != "" && instanceId != "all" {
		totalQuery += fmt.Sprintf(" AND instance_id = '%s'", instanceId)
	}
	// n9e-2kai: 通过 cloud_rds_owner 表关联筛选负责人
	if owner != "" && owner != "all" {
		totalQuery += fmt.Sprintf(" AND instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = '%s')", owner)
	}
	DB(c).Raw(totalQuery).Scan(&stats.TotalCount)

	// 各状态计数
	var statusCounts []struct {
		Status string
		Count  int64
	}
	statusSession := DB(c).Model(&CloudRDSSlowSQLStatus{}).Select("status, COUNT(*) as count")
	// n9e-2kai: 如果有 owner 筛选，需要关联 report 表获取 instance_id
	if owner != "" && owner != "all" {
		statusSession = statusSession.Where("sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?))", owner)
	}
	statusSession.Group("status").Find(&statusCounts)

	for _, sc := range statusCounts {
		switch sc.Status {
		case SlowSQLStatusPending:
			stats.PendingCount = sc.Count
		case SlowSQLStatusUrgent:
			stats.UrgentCount = sc.Count
		case SlowSQLStatusObserving:
			stats.ObservingCount = sc.Count
		case SlowSQLStatusOptimized:
			stats.OptimizedCount = sc.Count
		case SlowSQLStatusIgnored:
			stats.IgnoredCount = sc.Count
		// 兼容旧状态
		case SlowSQLStatusAnalyzing, SlowSQLStatusOptimizing:
			stats.PendingCount += sc.Count
		case SlowSQLStatusVerified:
			stats.OptimizedCount += sc.Count
		}
	}

	// pending 包含 status 表中没有记录的
	hasStatusCount := stats.PendingCount + stats.UrgentCount + stats.ObservingCount +
		stats.OptimizedCount + stats.IgnoredCount
	stats.PendingCount = stats.TotalCount - hasStatusCount + stats.PendingCount

	// 本周新增（report 表中 first_seen_at 在本周的）
	weekStart := getWeekStart(time.Now())
	thisWeekQuery := "SELECT COUNT(DISTINCT sql_hash) FROM cloud_rds_slowlog_report WHERE period_type = 'day' AND first_seen_at >= ?"
	if instanceId != "" && instanceId != "all" {
		thisWeekQuery += fmt.Sprintf(" AND instance_id = '%s'", instanceId)
	}
	if owner != "" && owner != "all" {
		thisWeekQuery += fmt.Sprintf(" AND instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = '%s')", owner)
	}
	DB(c).Raw(thisWeekQuery, weekStart.Unix()).Scan(&stats.ThisWeekNew)

	// 本周完成
	doneSession := DB(c).Model(&CloudRDSSlowSQLStatus{}).
		Where("status_changed_at >= ? AND status = ?", weekStart.Unix(), SlowSQLStatusOptimized)
	if owner != "" && owner != "all" {
		doneSession = doneSession.Where("sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?))", owner)
	}
	doneSession.Count(&stats.ThisWeekDone)

	// 高优先级待处理 - n9e-2kai: 直接使用紧急状态的数量
	stats.HighPriorityPending = stats.UrgentCount

	return stats, nil
}

func getWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(t.Year(), t.Month(), t.Day()-weekday+1, 0, 0, 0, 0, t.Location())
}

// ==================== 趋势数据 ====================

// SlowSQLStatusTrend 优化趋势数据结构
type SlowSQLStatusTrend struct {
	WeekKey      string `json:"week_key"`
	NewCount     int64  `json:"new_count"`
	DoneCount    int64  `json:"done_count"`
	PendingCount int64  `json:"pending_count"`
	NetChange    int64  `json:"net_change"`
}

// CloudRDSSlowSQLStatusGetTrend 获取优化趋势数据
// 基于 cloud_rds_slowlog_report 和 cloud_rds_slowsql_status 表
// 支持按实例ID和负责人筛选
func CloudRDSSlowSQLStatusGetTrend(c *ctx.Context, instanceId, owner string, weeks int) ([]SlowSQLStatusTrend, error) {
	if weeks <= 0 {
		weeks = 4
	}
	if weeks > 12 {
		weeks = 12
	}

	var trends []SlowSQLStatusTrend
	now := time.Now()

	// 获取 owner 筛选条件的实例列表
	var ownerInstanceIds []string
	if owner != "" && owner != "all" {
		err := DB(c).Model(&CloudRDSOwner{}).
			Where("owner = ?", owner).
			Pluck("instance_id", &ownerInstanceIds).Error
		if err != nil {
			return nil, err
		}
		if len(ownerInstanceIds) == 0 {
			// 没有匹配的实例，返回空趋势
			for i := weeks - 1; i >= 0; i-- {
				weekTime := now.AddDate(0, 0, -7*i)
				weekKey := getWeekKeyFromTime(weekTime)
				trends = append(trends, SlowSQLStatusTrend{WeekKey: weekKey})
			}
			return trends, nil
		}
	}

	// 从最早的周开始遍历到当前周
	for i := weeks - 1; i >= 0; i-- {
		weekTime := now.AddDate(0, 0, -7*i)
		weekStart := getWeekStart(weekTime)
		weekEnd := weekStart.AddDate(0, 0, 7)
		weekKey := getWeekKeyFromTime(weekTime)

		trend := SlowSQLStatusTrend{WeekKey: weekKey}

		// 本周新增：统计 cloud_rds_slowlog_report 表中 first_seen_at 在本周的唯一 sql_hash 数量
		newCountQuery := DB(c).Model(&CloudRDSSlowLogReportDB{}).
			Where("period_type = 'day'").
			Where("first_seen_at >= ? AND first_seen_at < ?", weekStart.Unix(), weekEnd.Unix())
		if instanceId != "" && instanceId != "all" {
			newCountQuery = newCountQuery.Where("instance_id = ?", instanceId)
		}
		if len(ownerInstanceIds) > 0 {
			newCountQuery = newCountQuery.Where("instance_id IN ?", ownerInstanceIds)
		}
		newCountQuery.Distinct("sql_hash").Count(&trend.NewCount)

		// 本周完成：统计 cloud_rds_slowsql_status 表中 status_changed_at 在本周且 status 为 optimized 的数量
		doneCountQuery := DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("status_changed_at >= ? AND status_changed_at < ?", weekStart.Unix(), weekEnd.Unix()).
			Where("status = ?", SlowSQLStatusOptimized)
		// 如果有实例或负责人筛选，需要关联 report 表
		if (instanceId != "" && instanceId != "all") || len(ownerInstanceIds) > 0 {
			subQuery := "sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE period_type = 'day'"
			if instanceId != "" && instanceId != "all" {
				subQuery += fmt.Sprintf(" AND instance_id = '%s'", instanceId)
			}
			if len(ownerInstanceIds) > 0 {
				instanceIdsStr := "'" + ownerInstanceIds[0] + "'"
				for j := 1; j < len(ownerInstanceIds); j++ {
					instanceIdsStr += ",'" + ownerInstanceIds[j] + "'"
				}
				subQuery += " AND instance_id IN (" + instanceIdsStr + ")"
			}
			subQuery += ")"
			doneCountQuery = doneCountQuery.Where(subQuery)
		}
		doneCountQuery.Count(&trend.DoneCount)

		// 截至该周末的待处理总数
		// 待处理 = 总数 - 已优化数 - 已忽略数
		var totalCount int64
		totalQuery := DB(c).Model(&CloudRDSSlowLogReportDB{}).
			Where("period_type = 'day'").
			Where("first_seen_at < ?", weekEnd.Unix())
		if instanceId != "" && instanceId != "all" {
			totalQuery = totalQuery.Where("instance_id = ?", instanceId)
		}
		if len(ownerInstanceIds) > 0 {
			totalQuery = totalQuery.Where("instance_id IN ?", ownerInstanceIds)
		}
		totalQuery.Distinct("sql_hash").Count(&totalCount)

		var optimizedCount, ignoredCount int64
		optimizedQuery := DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("status_changed_at < ?", weekEnd.Unix()).
			Where("status = ?", SlowSQLStatusOptimized)
		if (instanceId != "" && instanceId != "all") || len(ownerInstanceIds) > 0 {
			subQuery := "sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE period_type = 'day'"
			if instanceId != "" && instanceId != "all" {
				subQuery += fmt.Sprintf(" AND instance_id = '%s'", instanceId)
			}
			if len(ownerInstanceIds) > 0 {
				instanceIdsStr := "'" + ownerInstanceIds[0] + "'"
				for j := 1; j < len(ownerInstanceIds); j++ {
					instanceIdsStr += ",'" + ownerInstanceIds[j] + "'"
				}
				subQuery += " AND instance_id IN (" + instanceIdsStr + ")"
			}
			subQuery += ")"
			optimizedQuery = optimizedQuery.Where(subQuery)
		}
		optimizedQuery.Count(&optimizedCount)

		ignoredQuery := DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("status_changed_at < ?", weekEnd.Unix()).
			Where("status = ?", SlowSQLStatusIgnored)
		if (instanceId != "" && instanceId != "all") || len(ownerInstanceIds) > 0 {
			subQuery := "sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE period_type = 'day'"
			if instanceId != "" && instanceId != "all" {
				subQuery += fmt.Sprintf(" AND instance_id = '%s'", instanceId)
			}
			if len(ownerInstanceIds) > 0 {
				instanceIdsStr := "'" + ownerInstanceIds[0] + "'"
				for j := 1; j < len(ownerInstanceIds); j++ {
					instanceIdsStr += ",'" + ownerInstanceIds[j] + "'"
				}
				subQuery += " AND instance_id IN (" + instanceIdsStr + ")"
			}
			subQuery += ")"
			ignoredQuery = ignoredQuery.Where(subQuery)
		}
		ignoredQuery.Count(&ignoredCount)

		trend.PendingCount = totalCount - optimizedCount - ignoredCount
		if trend.PendingCount < 0 {
			trend.PendingCount = 0
		}

		trend.NetChange = trend.NewCount - trend.DoneCount
		trends = append(trends, trend)
	}

	return trends, nil
}

// ==================== 优化判定逻辑（基于置信度 + 动态观察期）====================

// OptimizationJudgmentResult 每日优化判定结果
type OptimizationJudgmentResult struct {
	// Step 1: 活跃状态更新
	ActiveUpdatedCount    int64 `json:"active_updated_count"`    // 今日活跃的指纹数量
	RevertedFromObserving int64 `json:"reverted_from_observing"` // 从观察期回退的数量
	// Step 2: 老化判定
	MovedToObservingCount int64 `json:"moved_to_observing_count"` // 进入观察期的数量
	// Step 3: 确认优化
	ConfirmedOptimizedCount int64    `json:"confirmed_optimized_count"` // 确认已优化的数量
	ConfirmedHashes         []string `json:"confirmed_hashes"`          // 确认已优化的 sql_hash
	// 统计信息
	TotalProcessed int64 `json:"total_processed"` // 总处理数量
}

// getRequiredObservingDays 根据日均频率计算所需的观察期天数
// 高频SQL短观察期，低频SQL长观察期
func getRequiredObservingDays(avgFrequency float64) int {
	switch {
	case avgFrequency >= 100: // 高频（100+次/天）
		return 3
	case avgFrequency >= 10: // 中高频（10-100次/天）
		return 7
	case avgFrequency >= 1: // 中频（1-10次/天）
		return 14
	case avgFrequency >= 0.1: // 低频（约3天一次）
		return 21
	default: // 极低频（<0.1次/天，约月报）
		return 45
	}
}

// getRequiredObservingDaysForEntry 进入观察期所需的天数（为完整观察期的一半）
func getRequiredObservingDaysForEntry(avgFrequency float64) int {
	fullDays := getRequiredObservingDays(avgFrequency)
	entryDays := fullDays / 2
	if entryDays < 1 {
		entryDays = 1
	}
	return entryDays
}

// calculateConfidence 计算优化判定置信度
// 综合考虑多个因素：稳定性、频率、历史跨度、数据充足性
func calculateConfidence(status *CloudRDSSlowSQLStatus, now time.Time) float64 {
	nowUnix := now.Unix()
	requiredDays := getRequiredObservingDays(status.AvgFrequency)

	// 因素1: 观察期稳定性 (权重 40%)
	// 在观察期内，SQL持续未出现的天数占比
	daysSinceLastSeen := float64(nowUnix-status.LastSeenTime) / 86400.0
	stabilityScore := daysSinceLastSeen / float64(requiredDays)
	if stabilityScore > 1.0 {
		stabilityScore = 1.0
	}

	// 因素2: 历史频率规律性 (权重 30%)
	// 频率越高，判定越可靠（高频SQL消失更有意义）
	frequencyScore := status.AvgFrequency / 10.0 // 10次/天为满分
	if frequencyScore > 1.0 {
		frequencyScore = 1.0
	}

	// 因素3: 历史跨度 (权重 20%)
	// 存在时间越长的SQL，判断越可靠
	var lifespanScore float64
	if status.FirstSeenTime > 0 && status.LastSeenTime > status.FirstSeenTime {
		lifespanDays := float64(status.LastSeenTime-status.FirstSeenTime) / 86400.0
		lifespanScore = lifespanDays / 30.0 // 30天为满分
		if lifespanScore > 1.0 {
			lifespanScore = 1.0
		}
	}

	// 因素4: 数据点充足性 (权重 10%)
	// 出现的天数越多，数据越可靠
	occurrenceScore := float64(status.TotalOccurrences) / 14.0 // 14天为满分
	if occurrenceScore > 1.0 {
		occurrenceScore = 1.0
	}

	confidence := stabilityScore*0.4 + frequencyScore*0.3 + lifespanScore*0.2 + occurrenceScore*0.1
	return confidence
}

// DailyOptimizationJudgment 每日优化判定任务
// 实现基于 last_seen_time + 动态观察期 + 置信度的判定逻辑
func DailyOptimizationJudgment(c *ctx.Context) (*OptimizationJudgmentResult, error) {
	result := &OptimizationJudgmentResult{}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayUnix := today.Unix()
	nowUnix := now.Unix()

	fmt.Printf("[DailyOptimizationJudgment] Starting at %s\n", now.Format("2006-01-02 15:04:05"))

	// ==================== Step 1: 更新活跃状态 ====================
	// 获取今日出现的所有 sql_hash（从 report 表中最近24小时的数据）
	yesterday := today.AddDate(0, 0, -1)
	var todayActiveHashes []string
	err := DB(c).Model(&CloudRDSSlowLogReportDB{}).
		Where("period_type = 'day'").
		Where("period_start >= ?", yesterday.Unix()).
		Distinct().
		Pluck("sql_hash", &todayActiveHashes).Error
	if err != nil {
		return nil, fmt.Errorf("get today active sql_hash failed: %w", err)
	}

	fmt.Printf("[DailyOptimizationJudgment] Found %d active SQL hashes today\n", len(todayActiveHashes))

	// 更新这些指纹的 last_seen_time
	if len(todayActiveHashes) > 0 {
		// 批量更新已存在的状态记录
		updateResult := DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("sql_hash IN ?", todayActiveHashes).
			Updates(map[string]interface{}{
				"last_seen_time": todayUnix,
				"updated_at":     nowUnix,
			})
		result.ActiveUpdatedCount = updateResult.RowsAffected

		// 处理观察期中的指纹回退
		// 如果处于观察期的 SQL 今天又出现了，回退到待评估状态
		var observingToRevert []CloudRDSSlowSQLStatus
		DB(c).Where("sql_hash IN ?", todayActiveHashes).
			Where("status = ?", SlowSQLStatusObserving).
			Find(&observingToRevert)

		if len(observingToRevert) > 0 {
			var hashesToRevert []string
			for _, s := range observingToRevert {
				hashesToRevert = append(hashesToRevert, s.SqlHash)
			}

			DB(c).Model(&CloudRDSSlowSQLStatus{}).
				Where("sql_hash IN ?", hashesToRevert).
				Updates(map[string]interface{}{
					"status":            SlowSQLStatusPending,
					"observing_since":   0,
					"status_changed_at": nowUnix,
					"status_changed_by": "system_auto",
					"updated_at":        nowUnix,
				})
			result.RevertedFromObserving = int64(len(observingToRevert))

			// 记录日志
			var logs []CloudRDSSlowSQLStatusLog
			for _, hash := range hashesToRevert {
				logs = append(logs, CloudRDSSlowSQLStatusLog{
					SqlHash:   hash,
					OldStatus: SlowSQLStatusObserving,
					NewStatus: SlowSQLStatusPending,
					Operator:  "system_auto",
					Comment:   "观察期内再次出现，回退到待评估",
					CreatedAt: nowUnix,
				})
			}
			if len(logs) > 0 {
				DB(c).CreateInBatches(logs, 100)
			}
			fmt.Printf("[DailyOptimizationJudgment] Reverted %d SQL from observing to pending\n", len(observingToRevert))
		}

		// 为新出现的 sql_hash 创建状态记录（如果不存在）
		var existingHashes []string
		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("sql_hash IN ?", todayActiveHashes).
			Pluck("sql_hash", &existingHashes)

		existingSet := make(map[string]bool)
		for _, hash := range existingHashes {
			existingSet[hash] = true
		}

		var newRecords []CloudRDSSlowSQLStatus
		for _, hash := range todayActiveHashes {
			if !existingSet[hash] {
				newRecords = append(newRecords, CloudRDSSlowSQLStatus{
					SqlHash:       hash,
					Status:        SlowSQLStatusPending,
					Priority:      SlowSQLPriorityMedium,
					LastSeenTime:  todayUnix,
					FirstSeenTime: todayUnix,
					CreatedAt:     nowUnix,
					UpdatedAt:     nowUnix,
				})
			}
		}
		if len(newRecords) > 0 {
			DB(c).CreateInBatches(newRecords, 100)
		}
	}

	// ==================== Step 2: 更新频率统计 ====================
	// 从 report 表获取每个 sql_hash 的频率统计
	err = updateAllFrequencyStats(c)
	if err != nil {
		fmt.Printf("[DailyOptimizationJudgment] Warning: failed to update frequency stats: %v\n", err)
	}

	// ==================== Step 3: 老化判定 - Pending/Urgent → Observing ====================
	var pendingStatuses []CloudRDSSlowSQLStatus
	DB(c).Where("status IN ?", []string{SlowSQLStatusPending, SlowSQLStatusUrgent}).
		Where("last_seen_time > 0").
		Find(&pendingStatuses)

	var movedToObserving int64
	for _, status := range pendingStatuses {
		daysSinceLastSeen := (nowUnix - status.LastSeenTime) / 86400
		requiredDays := getRequiredObservingDaysForEntry(status.AvgFrequency)

		if daysSinceLastSeen >= int64(requiredDays) {
			oldStatus := status.Status
			DB(c).Model(&CloudRDSSlowSQLStatus{}).
				Where("id = ?", status.Id).
				Updates(map[string]interface{}{
					"status":            SlowSQLStatusObserving,
					"observing_since":   nowUnix,
					"status_changed_at": nowUnix,
					"status_changed_by": "system_auto",
					"updated_at":        nowUnix,
				})
			movedToObserving++

			// 记录日志
			DB(c).Create(&CloudRDSSlowSQLStatusLog{
				SqlHash:   status.SqlHash,
				OldStatus: oldStatus,
				NewStatus: SlowSQLStatusObserving,
				Operator:  "system_auto",
				Comment:   fmt.Sprintf("未出现%d天，进入观察期（频率%.2f次/天，阈值%d天）", daysSinceLastSeen, status.AvgFrequency, requiredDays),
				CreatedAt: nowUnix,
			})
		}
	}
	result.MovedToObservingCount = movedToObserving
	fmt.Printf("[DailyOptimizationJudgment] Moved %d SQL to observing status\n", movedToObserving)

	// ==================== Step 4: 观察期 → 已优化（基于置信度）====================
	var observingStatuses []CloudRDSSlowSQLStatus
	DB(c).Where("status = ?", SlowSQLStatusObserving).Find(&observingStatuses)

	var confirmedOptimized int64
	var confirmedHashes []string
	const confidenceThreshold = 0.7 // 置信度阈值

	for _, status := range observingStatuses {
		daysSinceLastSeen := (nowUnix - status.LastSeenTime) / 86400
		requiredDays := getRequiredObservingDays(status.AvgFrequency)
		confidence := calculateConfidence(&status, now)

		// 双重条件：满足观察期 AND 置信度达标
		if daysSinceLastSeen >= int64(requiredDays) && confidence >= confidenceThreshold {
			DB(c).Model(&CloudRDSSlowSQLStatus{}).
				Where("id = ?", status.Id).
				Updates(map[string]interface{}{
					"status":            SlowSQLStatusOptimized,
					"confidence":        confidence,
					"status_changed_at": nowUnix,
					"status_changed_by": "system_auto",
					"optimize_note":     fmt.Sprintf("系统自动标记：%d天未出现，置信度%.1f%%", daysSinceLastSeen, confidence*100),
					"updated_at":        nowUnix,
				})
			confirmedOptimized++
			confirmedHashes = append(confirmedHashes, status.SqlHash)

			// 记录日志
			DB(c).Create(&CloudRDSSlowSQLStatusLog{
				SqlHash:   status.SqlHash,
				OldStatus: SlowSQLStatusObserving,
				NewStatus: SlowSQLStatusOptimized,
				Operator:  "system_auto",
				Comment:   fmt.Sprintf("确认已优化：%d天未出现，置信度%.1f%%（阈值%d天，置信度≥70%%）", daysSinceLastSeen, confidence*100, requiredDays),
				CreatedAt: nowUnix,
			})
		}
	}
	result.ConfirmedOptimizedCount = confirmedOptimized
	result.ConfirmedHashes = confirmedHashes
	result.TotalProcessed = int64(len(todayActiveHashes)) + int64(len(pendingStatuses)) + int64(len(observingStatuses))

	fmt.Printf("[DailyOptimizationJudgment] Confirmed %d SQL as optimized\n", confirmedOptimized)
	fmt.Printf("[DailyOptimizationJudgment] Completed: active=%d, reverted=%d, observing=%d, optimized=%d\n",
		result.ActiveUpdatedCount, result.RevertedFromObserving, result.MovedToObservingCount, result.ConfirmedOptimizedCount)

	return result, nil
}

// updateAllFrequencyStats 更新所有指纹的频率统计
// 基于最近30天的滑动窗口计算日均频率
func updateAllFrequencyStats(c *ctx.Context) error {
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30).Unix()

	// 从 report 表获取每个 sql_hash 的统计信息
	type FreqStat struct {
		SqlHash         string
		FirstSeenAt     int64
		LastSeenAt      int64
		TotalDays       int64   // 出现的天数
		AvgExecuteCount float64 // 日均执行次数
	}

	var stats []FreqStat
	// 统计最近30天内每个 sql_hash 出现的天数和日均执行次数
	err := DB(c).Model(&CloudRDSSlowLogReportDB{}).
		Select(`
			sql_hash,
			MIN(first_seen_at) as first_seen_at,
			MAX(last_seen_at) as last_seen_at,
			COUNT(DISTINCT DATE(FROM_UNIXTIME(period_start))) as total_days,
			AVG(execute_count) as avg_execute_count
		`).
		Where("period_type = 'day'").
		Where("period_start >= ?", thirtyDaysAgo).
		Group("sql_hash").
		Find(&stats).Error
	if err != nil {
		return fmt.Errorf("query frequency stats failed: %w", err)
	}

	// 批量更新状态表
	for _, stat := range stats {
		// 计算日均频率：总出现天数 / 30天 * 日均执行次数
		// 简化为：出现天数 / 30 作为"活跃度"指标
		avgFrequency := float64(stat.TotalDays) / 30.0 * stat.AvgExecuteCount
		if avgFrequency < 0.01 {
			avgFrequency = 0.01 // 最小值，避免除零
		}

		// 获取当前状态记录，用于计算置信度
		var status CloudRDSSlowSQLStatus
		err := DB(c).Where("sql_hash = ?", stat.SqlHash).First(&status).Error
		if err != nil {
			// 状态记录不存在，跳过（后续会由 DailyOptimizationJudgment 创建）
			continue
		}

		// 更新状态对象的字段，用于计算置信度
		status.FirstSeenTime = stat.FirstSeenAt
		status.TotalOccurrences = stat.TotalDays
		status.AvgFrequency = avgFrequency
		if status.LastSeenTime == 0 {
			status.LastSeenTime = stat.LastSeenAt
		}

		// 计算置信度（预测值：如果 SQL 从现在开始不再出现，多少天后能确认优化）
		confidence := calculateConfidence(&status, now)

		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("sql_hash = ?", stat.SqlHash).
			Updates(map[string]interface{}{
				"first_seen_time":   stat.FirstSeenAt,
				"total_occurrences": stat.TotalDays,
				"avg_frequency":     avgFrequency,
				"confidence":        confidence,
				"updated_at":        now.Unix(),
			})

		// 如果 last_seen_time 为空，也更新它
		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where("sql_hash = ? AND last_seen_time = 0", stat.SqlHash).
			Update("last_seen_time", stat.LastSeenAt)
	}

	return nil
}

// AutoMarkOptimizedSlowSQLs 自动标记已优化的慢SQL（兼容旧接口）
// 新逻辑：调用 DailyOptimizationJudgment
// Deprecated: 请使用 DailyOptimizationJudgment
func AutoMarkOptimizedSlowSQLs(c *ctx.Context) (*OptimizationJudgmentResult, error) {
	return DailyOptimizationJudgment(c)
}

// getWeekKeyFromTime 获取周标识字符串
func getWeekKeyFromTime(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ==================== 负责人维度统计 ====================

// OwnerStatsItem 负责人统计项
type OwnerStatsItem struct {
	Owner          string  `json:"owner"`
	Team           string  `json:"team"`
	InstanceCount  int     `json:"instance_count"`
	PendingCount   int64   `json:"pending_count"`
	UrgentCount    int64   `json:"urgent_count"`
	ObservingCount int64   `json:"observing_count"` // n9e-2kai: 观察期中的数量
	OptimizedCount int64   `json:"optimized_count"`
	IgnoredCount   int64   `json:"ignored_count"`
	TotalCount     int64   `json:"total_count"`
	ThisWeekNew    int64   `json:"this_week_new"`
	ThisWeekDone   int64   `json:"this_week_done"`
	CompletionRate float64 `json:"completion_rate"`
}

// CloudRDSSlowSQLStatusGetOwnerStats 获取负责人统计数据
// 基于 cloud_rds_slowlog_report + cloud_rds_slowsql_status + cloud_rds_owner 表
func CloudRDSSlowSQLStatusGetOwnerStats(c *ctx.Context, weekOffset int) ([]OwnerStatsItem, error) {
	// 获取目标周的开始时间
	now := time.Now()
	targetDate := now.AddDate(0, 0, weekOffset*7)
	weekStart := getWeekStart(targetDate)

	// 获取所有负责人及其关联的实例
	var owners []CloudRDSOwner
	err := DB(c).Where("owner != ''").Find(&owners).Error
	if err != nil {
		return nil, err
	}

	// 调试日志
	fmt.Printf("[DEBUG] CloudRDSSlowSQLStatusGetOwnerStats: found %d owners in cloud_rds_owner table\n", len(owners))
	for _, o := range owners {
		fmt.Printf("[DEBUG]   Owner: %s, InstanceId: %s\n", o.Owner, o.InstanceId)
	}

	// 按负责人分组
	ownerInstanceMap := make(map[string][]string)
	ownerTeamMap := make(map[string]string)
	for _, o := range owners {
		ownerInstanceMap[o.Owner] = append(ownerInstanceMap[o.Owner], o.InstanceId)
		if o.Team != "" {
			ownerTeamMap[o.Owner] = o.Team
		}
	}

	var results []OwnerStatsItem

	for owner, instanceIds := range ownerInstanceMap {
		if len(instanceIds) == 0 {
			continue
		}

		item := OwnerStatsItem{
			Owner:         owner,
			Team:          ownerTeamMap[owner],
			InstanceCount: len(instanceIds),
		}

		// 总数：该负责人所有实例的唯一 sql_hash 数量
		DB(c).Model(&CloudRDSSlowLogReportDB{}).
			Where("period_type = 'day'").
			Where("instance_id IN ?", instanceIds).
			Distinct("sql_hash").Count(&item.TotalCount)

		// 本周新增
		DB(c).Model(&CloudRDSSlowLogReportDB{}).
			Where("period_type = 'day'").
			Where("instance_id IN ?", instanceIds).
			Where("first_seen_at >= ?", weekStart.Unix()).
			Distinct("sql_hash").Count(&item.ThisWeekNew)

		// 获取状态分布
		// 构建该负责人相关的 sql_hash 列表的子查询条件
		instanceIdsStr := "'" + instanceIds[0] + "'"
		for j := 1; j < len(instanceIds); j++ {
			instanceIdsStr += ",'" + instanceIds[j] + "'"
		}
		subQuery := "sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE period_type = 'day' AND instance_id IN (" + instanceIdsStr + "))"

		var statusCounts []struct {
			Status string
			Count  int64
		}
		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where(subQuery).
			Select("status, COUNT(*) as count").
			Group("status").Find(&statusCounts)

		for _, sc := range statusCounts {
			switch sc.Status {
			case SlowSQLStatusPending:
				item.PendingCount = sc.Count
			case SlowSQLStatusUrgent:
				item.UrgentCount = sc.Count
			case SlowSQLStatusObserving:
				item.ObservingCount = sc.Count
			case SlowSQLStatusOptimized:
				item.OptimizedCount = sc.Count
			case SlowSQLStatusIgnored:
				item.IgnoredCount = sc.Count
			case SlowSQLStatusAnalyzing, SlowSQLStatusOptimizing:
				item.PendingCount += sc.Count
			case SlowSQLStatusVerified:
				item.OptimizedCount += sc.Count
			}
		}

		// 待处理数 = 总数 - 有状态的数量 + pending状态的数量
		hasStatusCount := item.PendingCount + item.UrgentCount + item.ObservingCount + item.OptimizedCount + item.IgnoredCount
		item.PendingCount = item.TotalCount - hasStatusCount + item.PendingCount

		// 本周完成
		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where(subQuery).
			Where("status_changed_at >= ? AND status = ?", weekStart.Unix(), SlowSQLStatusOptimized).
			Count(&item.ThisWeekDone)

		// 计算完成率
		if item.TotalCount > 0 {
			item.CompletionRate = float64(item.OptimizedCount+item.IgnoredCount) / float64(item.TotalCount) * 100
		}

		results = append(results, item)
	}

	return results, nil
}

// OwnerTrendItem 负责人趋势项
type OwnerTrendItem struct {
	WeekKey      string `json:"week_key"`
	NewCount     int64  `json:"new_count"`
	DoneCount    int64  `json:"done_count"`
	PendingCount int64  `json:"pending_count"`
	NetChange    int64  `json:"net_change"`
}

// CloudRDSSlowSQLStatusGetOwnerTrend 获取负责人趋势数据
func CloudRDSSlowSQLStatusGetOwnerTrend(c *ctx.Context, owner string, weeks int) ([]OwnerTrendItem, error) {
	if weeks <= 0 {
		weeks = 4
	}
	if weeks > 12 {
		weeks = 12
	}

	// 获取该负责人关联的所有实例
	var instanceIds []string
	err := DB(c).Model(&CloudRDSOwner{}).
		Where("owner = ?", owner).
		Pluck("instance_id", &instanceIds).Error
	if err != nil {
		return nil, err
	}

	var trends []OwnerTrendItem
	now := time.Now()

	if len(instanceIds) == 0 {
		// 没有匹配的实例，返回空趋势
		for i := weeks - 1; i >= 0; i-- {
			weekTime := now.AddDate(0, 0, -7*i)
			weekKey := getWeekKeyFromTime(weekTime)
			trends = append(trends, OwnerTrendItem{WeekKey: weekKey})
		}
		return trends, nil
	}

	// 构建实例 ID 条件
	instanceIdsStr := "'" + instanceIds[0] + "'"
	for j := 1; j < len(instanceIds); j++ {
		instanceIdsStr += ",'" + instanceIds[j] + "'"
	}
	subQuery := "sql_hash IN (SELECT DISTINCT sql_hash FROM cloud_rds_slowlog_report WHERE period_type = 'day' AND instance_id IN (" + instanceIdsStr + "))"

	// 从最早的周开始遍历到当前周
	for i := weeks - 1; i >= 0; i-- {
		weekTime := now.AddDate(0, 0, -7*i)
		weekStart := getWeekStart(weekTime)
		weekEnd := weekStart.AddDate(0, 0, 7)
		weekKey := getWeekKeyFromTime(weekTime)

		trend := OwnerTrendItem{WeekKey: weekKey}

		// 本周新增
		DB(c).Model(&CloudRDSSlowLogReportDB{}).
			Where("period_type = 'day'").
			Where("instance_id IN ?", instanceIds).
			Where("first_seen_at >= ? AND first_seen_at < ?", weekStart.Unix(), weekEnd.Unix()).
			Distinct("sql_hash").Count(&trend.NewCount)

		// 本周完成
		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where(subQuery).
			Where("status_changed_at >= ? AND status_changed_at < ?", weekStart.Unix(), weekEnd.Unix()).
			Where("status = ?", SlowSQLStatusOptimized).
			Count(&trend.DoneCount)

		// 截至该周末的待处理数
		var totalCount int64
		DB(c).Model(&CloudRDSSlowLogReportDB{}).
			Where("period_type = 'day'").
			Where("instance_id IN ?", instanceIds).
			Where("first_seen_at < ?", weekEnd.Unix()).
			Distinct("sql_hash").Count(&totalCount)

		var optimizedCount, ignoredCount int64
		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where(subQuery).
			Where("status_changed_at < ?", weekEnd.Unix()).
			Where("status = ?", SlowSQLStatusOptimized).
			Count(&optimizedCount)

		DB(c).Model(&CloudRDSSlowSQLStatus{}).
			Where(subQuery).
			Where("status_changed_at < ?", weekEnd.Unix()).
			Where("status = ?", SlowSQLStatusIgnored).
			Count(&ignoredCount)

		trend.PendingCount = totalCount - optimizedCount - ignoredCount
		if trend.PendingCount < 0 {
			trend.PendingCount = 0
		}

		trend.NetChange = trend.NewCount - trend.DoneCount
		trends = append(trends, trend)
	}

	return trends, nil
}
