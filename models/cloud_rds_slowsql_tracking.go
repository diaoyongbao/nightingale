// n9e-2kai: 慢SQL优化跟踪模型
package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"gorm.io/gorm"
)

// 慢SQL优化状态常量（简化版）
const (
	SlowSQLStatusPending   = "pending"   // 待评估
	SlowSQLStatusUrgent    = "urgent"    // 紧急
	SlowSQLStatusObserving = "observing" // 观察期（疑似已优化，等待确认）
	SlowSQLStatusOptimized = "optimized" // 已优化
	SlowSQLStatusIgnored   = "ignored"   // 已忽略
	// 废弃的状态，保留以兼容旧数据
	SlowSQLStatusAnalyzing  = "analyzing"  // 分析中 - 已废弃
	SlowSQLStatusOptimizing = "optimizing" // 优化中 - 已废弃
	SlowSQLStatusVerified   = "verified"   // 已验证 - 已废弃
)

// 优先级常量
const (
	SlowSQLPriorityHigh   = "high"
	SlowSQLPriorityMedium = "medium"
	SlowSQLPriorityLow    = "low"
)

// CloudRDSSlowSQLTracking 慢SQL优化跟踪表
type CloudRDSSlowSQLTracking struct {
	Id                 int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	SqlHash            string  `json:"sql_hash" gorm:"type:varchar(64);uniqueIndex;not null"`
	SqlFingerprint     string  `json:"sql_fingerprint" gorm:"type:text"`
	SqlType            string  `json:"sql_type" gorm:"type:varchar(32)"`
	SampleSql          string  `json:"sample_sql" gorm:"type:text"`
	Database           string  `json:"database" gorm:"type:varchar(128)"`
	InstanceId         string  `json:"instance_id" gorm:"type:varchar(128);index"`
	InstanceName       string  `json:"instance_name" gorm:"type:varchar(256)"`
	Status             string  `json:"status" gorm:"type:varchar(32);index;default:'pending'"`
	Priority           string  `json:"priority" gorm:"type:varchar(16);index;default:'medium'"`
	Owner              string  `json:"owner" gorm:"type:varchar(64)"`
	OwnerEmail         string  `json:"owner_email" gorm:"type:varchar(128)"`
	Team               string  `json:"team" gorm:"type:varchar(128)"`
	FirstSeenAt        int64   `json:"first_seen_at" gorm:"index"`
	LastSeenAt         int64   `json:"last_seen_at" gorm:"index"`
	StatusChangedAt    int64   `json:"status_changed_at"`
	ExpectedCompleteAt int64   `json:"expected_complete_at"`
	TotalExecutions    int64   `json:"total_executions"`
	AvgTime            float64 `json:"avg_time"`
	MaxTime            float64 `json:"max_time"`
	LastWeekCount      int64   `json:"last_week_count"`
	ThisWeekCount      int64   `json:"this_week_count"`
	OptimizeNote       string  `json:"optimize_note" gorm:"type:text"`
	OptimizeResult     string  `json:"optimize_result" gorm:"type:text"`
	AutoOptimized      bool    `json:"auto_optimized" gorm:"default:false"`
	CreatedAt          int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          int64   `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedBy          string  `json:"created_by" gorm:"type:varchar(64)"`
	UpdatedBy          string  `json:"updated_by" gorm:"type:varchar(64)"`
}

func (CloudRDSSlowSQLTracking) TableName() string {
	return "cloud_rds_slowsql_tracking"
}

// CloudRDSSlowSQLTrackingLog 状态变更日志表
type CloudRDSSlowSQLTrackingLog struct {
	Id         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	TrackingId int64  `json:"tracking_id" gorm:"index;not null"`
	SqlHash    string `json:"sql_hash" gorm:"type:varchar(64);index"`
	OldStatus  string `json:"old_status" gorm:"type:varchar(32)"`
	NewStatus  string `json:"new_status" gorm:"type:varchar(32)"`
	Operator   string `json:"operator" gorm:"type:varchar(64)"`
	Comment    string `json:"comment" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CloudRDSSlowSQLTrackingLog) TableName() string {
	return "cloud_rds_slowsql_tracking_log"
}

// ==================== CloudRDSSlowSQLTracking CRUD ====================

func CloudRDSSlowSQLTrackingGetBySqlHash(c *ctx.Context, sqlHash string) (*CloudRDSSlowSQLTracking, error) {
	var tracking CloudRDSSlowSQLTracking
	err := DB(c).Where("sql_hash = ?", sqlHash).First(&tracking).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tracking, err
}

func CloudRDSSlowSQLTrackingGetById(c *ctx.Context, id int64) (*CloudRDSSlowSQLTracking, error) {
	var tracking CloudRDSSlowSQLTracking
	err := DB(c).Where("id = ?", id).First(&tracking).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tracking, err
}

func CloudRDSSlowSQLTrackingGets(c *ctx.Context, instanceId, status, priority, owner, query string, limit, offset int) ([]CloudRDSSlowSQLTracking, int64, error) {
	var list []CloudRDSSlowSQLTracking
	var total int64

	session := DB(c).Model(&CloudRDSSlowSQLTracking{})

	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	if status != "" && status != "all" {
		session = session.Where("status = ?", status)
	}
	if priority != "" && priority != "all" {
		session = session.Where("priority = ?", priority)
	}
	// n9e-2kai: 通过 cloud_rds_owner 表关联筛选负责人
	if owner != "" && owner != "all" {
		session = session.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}
	if query != "" {
		session = session.Where("sql_fingerprint LIKE ? OR sql_hash LIKE ? OR database LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	err := session.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = session.Order("priority = 'high' DESC, last_seen_at DESC").
		Limit(limit).Offset(offset).Find(&list).Error
	return list, total, err
}

func CloudRDSSlowSQLTrackingCreate(c *ctx.Context, tracking *CloudRDSSlowSQLTracking) error {
	tracking.CreatedAt = time.Now().Unix()
	tracking.UpdatedAt = time.Now().Unix()
	if tracking.Status == "" {
		tracking.Status = SlowSQLStatusPending
	}
	if tracking.Priority == "" {
		tracking.Priority = SlowSQLPriorityMedium
	}
	return DB(c).Create(tracking).Error
}

func CloudRDSSlowSQLTrackingUpdate(c *ctx.Context, tracking *CloudRDSSlowSQLTracking, fields ...string) error {
	tracking.UpdatedAt = time.Now().Unix()
	if len(fields) > 0 {
		return DB(c).Model(tracking).Select(fields).Updates(tracking).Error
	}
	return DB(c).Model(tracking).Updates(tracking).Error
}

func CloudRDSSlowSQLTrackingUpdateStatus(c *ctx.Context, id int64, newStatus, operator, comment string) error {
	tracking, err := CloudRDSSlowSQLTrackingGetById(c, id)
	if err != nil || tracking == nil {
		return fmt.Errorf("tracking not found: %d", id)
	}

	oldStatus := tracking.Status
	now := time.Now().Unix()

	err = DB(c).Model(&CloudRDSSlowSQLTracking{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":            newStatus,
		"status_changed_at": now,
		"updated_at":        now,
		"updated_by":        operator,
	}).Error
	if err != nil {
		return err
	}

	log := &CloudRDSSlowSQLTrackingLog{
		TrackingId: id,
		SqlHash:    tracking.SqlHash,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Operator:   operator,
		Comment:    comment,
		CreatedAt:  now,
	}
	return DB(c).Create(log).Error
}

func CloudRDSSlowSQLTrackingUpsert(c *ctx.Context, tracking *CloudRDSSlowSQLTracking) error {
	existing, err := CloudRDSSlowSQLTrackingGetBySqlHash(c, tracking.SqlHash)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	if existing == nil {
		tracking.CreatedAt = now
		tracking.UpdatedAt = now
		tracking.FirstSeenAt = now
		tracking.LastSeenAt = now
		if tracking.Status == "" {
			tracking.Status = SlowSQLStatusPending
		}
		if tracking.Priority == "" {
			tracking.Priority = SlowSQLPriorityMedium
		}
		return DB(c).Create(tracking).Error
	}

	updates := map[string]interface{}{
		"last_seen_at":     now,
		"updated_at":       now,
		"total_executions": tracking.TotalExecutions,
		"avg_time":         tracking.AvgTime,
		"max_time":         tracking.MaxTime,
		"this_week_count":  tracking.ThisWeekCount,
	}
	if tracking.SampleSql != "" {
		updates["sample_sql"] = tracking.SampleSql
	}
	if tracking.InstanceName != "" {
		updates["instance_name"] = tracking.InstanceName
	}

	return DB(c).Model(&CloudRDSSlowSQLTracking{}).Where("id = ?", existing.Id).Updates(updates).Error
}

// ==================== 统计查询 ====================

type SlowSQLTrackingStats struct {
	PendingCount        int64 `json:"pending_count"`
	UrgentCount         int64 `json:"urgent_count"`
	OptimizedCount      int64 `json:"optimized_count"`
	IgnoredCount        int64 `json:"ignored_count"`
	TotalCount          int64 `json:"total_count"`
	ThisWeekNew         int64 `json:"this_week_new"`
	ThisWeekDone        int64 `json:"this_week_done"`
	HighPriorityPending int64 `json:"high_priority_pending"`
}

// CloudRDSSlowSQLTrackingGetStats 获取慢SQL跟踪统计数据
// n9e-2kai: 增加 owner 参数支持按负责人筛选
func CloudRDSSlowSQLTrackingGetStats(c *ctx.Context, instanceId, owner string) (*SlowSQLTrackingStats, error) {
	stats := &SlowSQLTrackingStats{}

	session := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	// n9e-2kai: 通过 cloud_rds_owner 表关联筛选负责人
	if owner != "" && owner != "all" {
		session = session.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}

	var statusCounts []struct {
		Status string
		Count  int64
	}
	err := session.Select("status, count(*) as count").Group("status").Find(&statusCounts).Error
	if err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case SlowSQLStatusPending:
			stats.PendingCount = sc.Count
		case SlowSQLStatusUrgent:
			stats.UrgentCount = sc.Count
		case SlowSQLStatusOptimized:
			stats.OptimizedCount = sc.Count
		case SlowSQLStatusIgnored:
			stats.IgnoredCount = sc.Count
		// 兑容旧状态，旧数据归类到pending
		case SlowSQLStatusAnalyzing, SlowSQLStatusOptimizing:
			stats.PendingCount += sc.Count
		case SlowSQLStatusVerified:
			stats.OptimizedCount += sc.Count
		}
		stats.TotalCount += sc.Count
	}

	weekStart := getWeekStartTime(time.Now())

	session2 := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session2 = session2.Where("instance_id = ?", instanceId)
	}
	if owner != "" && owner != "all" {
		session2 = session2.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}
	session2.Where("first_seen_at >= ?", weekStart.Unix()).Count(&stats.ThisWeekNew)

	session3 := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session3 = session3.Where("instance_id = ?", instanceId)
	}
	if owner != "" && owner != "all" {
		session3 = session3.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}
	session3.Where("status_changed_at >= ? AND status IN ?", weekStart.Unix(),
		[]string{SlowSQLStatusOptimized}).Count(&stats.ThisWeekDone)

	session4 := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session4 = session4.Where("instance_id = ?", instanceId)
	}
	if owner != "" && owner != "all" {
		session4 = session4.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}
	session4.Where("priority = ? AND status = ?", SlowSQLPriorityHigh, SlowSQLStatusPending).
		Count(&stats.HighPriorityPending)

	return stats, nil
}

type SlowSQLWeeklyTrend struct {
	WeekKey      string `json:"week_key"`
	PendingCount int64  `json:"pending_count"`
	NewCount     int64  `json:"new_count"`
	DoneCount    int64  `json:"done_count"`
	NetChange    int64  `json:"net_change"`
}

func CloudRDSSlowSQLTrackingGetTrend(c *ctx.Context, instanceId string, weeks int) ([]SlowSQLWeeklyTrend, error) {
	var trends []SlowSQLWeeklyTrend

	now := time.Now()
	for i := weeks - 1; i >= 0; i-- {
		weekTime := now.AddDate(0, 0, -7*i)
		weekStart := getWeekStartTime(weekTime)
		weekEnd := weekStart.AddDate(0, 0, 7)
		weekKey := getWeekKeyStr(weekTime)

		trend := SlowSQLWeeklyTrend{WeekKey: weekKey}

		session := DB(c).Model(&CloudRDSSlowSQLTracking{})
		if instanceId != "" && instanceId != "all" {
			session = session.Where("instance_id = ?", instanceId)
		}
		session.Where("first_seen_at >= ? AND first_seen_at < ?", weekStart.Unix(), weekEnd.Unix()).
			Count(&trend.NewCount)

		session2 := DB(c).Model(&CloudRDSSlowSQLTracking{})
		if instanceId != "" && instanceId != "all" {
			session2 = session2.Where("instance_id = ?", instanceId)
		}
		session2.Where("status_changed_at >= ? AND status_changed_at < ? AND status IN ?",
			weekStart.Unix(), weekEnd.Unix(),
			[]string{SlowSQLStatusOptimized, SlowSQLStatusVerified}).
			Count(&trend.DoneCount)

		session3 := DB(c).Model(&CloudRDSSlowSQLTracking{})
		if instanceId != "" && instanceId != "all" {
			session3 = session3.Where("instance_id = ?", instanceId)
		}
		session3.Where("first_seen_at < ? AND (status_changed_at >= ? OR status IN ?)",
			weekEnd.Unix(), weekEnd.Unix(),
			[]string{SlowSQLStatusPending, SlowSQLStatusAnalyzing, SlowSQLStatusOptimizing}).
			Count(&trend.PendingCount)

		trend.NetChange = trend.NewCount - trend.DoneCount
		trends = append(trends, trend)
	}

	return trends, nil
}

// ==================== 自动判断已优化 ====================

func CloudRDSSlowSQLTrackingAutoOptimize(c *ctx.Context, instanceId string) (int64, error) {
	now := time.Now()
	thisWeekStart := getWeekStartTime(now)
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart

	var count int64
	session := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}

	err := session.Where("last_seen_at >= ? AND last_seen_at < ?", lastWeekStart.Unix(), lastWeekEnd.Unix()).
		Where("status NOT IN ?", []string{SlowSQLStatusOptimized, SlowSQLStatusVerified, SlowSQLStatusIgnored}).
		Count(&count).Error
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	nowUnix := now.Unix()
	updateSession := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		updateSession = updateSession.Where("instance_id = ?", instanceId)
	}

	err = updateSession.Where("last_seen_at >= ? AND last_seen_at < ?", lastWeekStart.Unix(), lastWeekEnd.Unix()).
		Where("status NOT IN ?", []string{SlowSQLStatusOptimized, SlowSQLStatusVerified, SlowSQLStatusIgnored}).
		Updates(map[string]interface{}{
			"status":            SlowSQLStatusOptimized,
			"status_changed_at": nowUnix,
			"updated_at":        nowUnix,
			"auto_optimized":    true,
		}).Error

	return count, err
}

// ==================== Markdown报告生成（本周优化情况总结）====================

func GenerateWeeklyOptimizationReport(c *ctx.Context, instanceId, instanceName string) (string, error) {
	now := time.Now()
	weekStart := getWeekStartTime(now)
	weekEnd := weekStart.AddDate(0, 0, 7)
	weekKey := getWeekKeyStr(now)

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# 慢SQL优化周报 - %s\n\n", weekKey))
	sb.WriteString(fmt.Sprintf("**统计周期**: %s ~ %s\n\n",
		weekStart.Format("2006-01-02"),
		weekEnd.AddDate(0, 0, -1).Format("2006-01-02")))
	if instanceName != "" {
		sb.WriteString(fmt.Sprintf("**实例**: %s (%s)\n\n", instanceName, instanceId))
	}

	stats, err := CloudRDSSlowSQLTrackingGetStats(c, instanceId, "")
	if err != nil {
		return "", err
	}

	sb.WriteString("## 📊 本周概览\n\n")
	sb.WriteString("| 指标 | 数量 |\n")
	sb.WriteString("|------|------|\n")
	sb.WriteString(fmt.Sprintf("| 本周新增慢SQL | %d |\n", stats.ThisWeekNew))
	sb.WriteString(fmt.Sprintf("| 本周已优化 | %d |\n", stats.ThisWeekDone))
	netChange := stats.ThisWeekNew - stats.ThisWeekDone
	changeIcon := "📈"
	if netChange < 0 {
		changeIcon = "📉"
	} else if netChange == 0 {
		changeIcon = "➡️"
	}
	sb.WriteString(fmt.Sprintf("| 净变化 | %s %+d |\n", changeIcon, netChange))
	sb.WriteString(fmt.Sprintf("| 当前待处理总数 | %d |\n", stats.PendingCount+stats.UrgentCount))
	sb.WriteString(fmt.Sprintf("| 高优先级待处理 | %d |\n\n", stats.HighPriorityPending))

	sb.WriteString("## 🎯 优化阶段分布\n\n")
	sb.WriteString("| 阶段 | 数量 |\n")
	sb.WriteString("|------|------|\n")
	sb.WriteString(fmt.Sprintf("| 待评估 | %d |\n", stats.PendingCount))
	sb.WriteString(fmt.Sprintf("| 紧急 | %d |\n", stats.UrgentCount))
	sb.WriteString(fmt.Sprintf("| 已优化 | %d |\n", stats.OptimizedCount))
	sb.WriteString(fmt.Sprintf("| 已忽略 | %d |\n\n", stats.IgnoredCount))

	// n9e-2kai: 负责人统计摘要
	ownerStats, err := CloudRDSSlowSQLTrackingGetOwnerStats(c, 0)
	if err == nil && len(ownerStats) > 0 {
		sb.WriteString("## 👥 负责人统计\n\n")
		sb.WriteString("| 负责人 | 团队 | 本周新增 | 本周完成 | 待处理 | 完成率 |\n")
		sb.WriteString("|--------|------|----------|----------|--------|--------|\n")
		for _, os := range ownerStats {
			completionRateStr := fmt.Sprintf("%.1f%%", os.CompletionRate)
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %s |\n",
				os.Owner, os.Team, os.ThisWeekNew, os.ThisWeekDone,
				os.PendingCount+os.UrgentCount, completionRateStr))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 🆕 本周新增慢SQL\n\n")
	newList, _, err := CloudRDSSlowSQLTrackingGetsThisWeekNew(c, instanceId, 20)
	if err == nil && len(newList) > 0 {
		sb.WriteString("| SQL指纹 | 类型 | 数据库 | 执行次数 | 平均时间 |\n")
		sb.WriteString("|---------|------|--------|----------|----------|\n")
		for _, item := range newList {
			fingerprint := item.SqlFingerprint
			if len(fingerprint) > 60 {
				fingerprint = fingerprint[:60] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %.3fs |\n",
				fingerprint, item.SqlType, item.Database, item.TotalExecutions, item.AvgTime))
		}
	} else {
		sb.WriteString("本周无新增慢SQL 🎉\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## ✅ 本周已优化\n\n")
	doneList, _, err := CloudRDSSlowSQLTrackingGetsThisWeekDone(c, instanceId, 20)
	if err == nil && len(doneList) > 0 {
		sb.WriteString("| SQL指纹 | 类型 | 负责人 | 优化结果 |\n")
		sb.WriteString("|---------|------|--------|----------|\n")
		for _, item := range doneList {
			fingerprint := item.SqlFingerprint
			if len(fingerprint) > 60 {
				fingerprint = fingerprint[:60] + "..."
			}
			result := item.OptimizeResult
			if result == "" {
				result = "-"
			} else if len(result) > 30 {
				result = result[:30] + "..."
			}
			owner := item.Owner
			if owner == "" {
				owner = "-"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				fingerprint, item.SqlType, owner, result))
		}
	} else {
		sb.WriteString("本周无优化完成的慢SQL\n")
	}
	sb.WriteString("\n")

	trends, err := CloudRDSSlowSQLTrackingGetTrend(c, instanceId, 4)
	if err == nil && len(trends) > 1 {
		sb.WriteString("## 📈 近4周趋势\n\n")
		sb.WriteString("| 周 | 新增 | 完成 | 净变化 |\n")
		sb.WriteString("|------|------|------|--------|\n")
		for _, t := range trends {
			changeStr := fmt.Sprintf("%+d", t.NetChange)
			if t.NetChange < 0 {
				changeStr = fmt.Sprintf("✅ %+d", t.NetChange)
			} else if t.NetChange > 0 {
				changeStr = fmt.Sprintf("⚠️ %+d", t.NetChange)
			}
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", t.WeekKey, t.NewCount, t.DoneCount, changeStr))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString(fmt.Sprintf("*报告生成时间: %s*\n", now.Format("2006-01-02 15:04:05")))

	return sb.String(), nil
}

func CloudRDSSlowSQLTrackingGetsThisWeekNew(c *ctx.Context, instanceId string, limit int) ([]CloudRDSSlowSQLTracking, int64, error) {
	var list []CloudRDSSlowSQLTracking
	var total int64

	weekStart := getWeekStartTime(time.Now())

	session := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	session = session.Where("first_seen_at >= ?", weekStart.Unix())

	err := session.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = session.Order("total_executions DESC").Limit(limit).Find(&list).Error
	return list, total, err
}

func CloudRDSSlowSQLTrackingGetsThisWeekDone(c *ctx.Context, instanceId string, limit int) ([]CloudRDSSlowSQLTracking, int64, error) {
	var list []CloudRDSSlowSQLTracking
	var total int64

	weekStart := getWeekStartTime(time.Now())

	session := DB(c).Model(&CloudRDSSlowSQLTracking{})
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	session = session.Where("status_changed_at >= ? AND status IN ?", weekStart.Unix(),
		[]string{SlowSQLStatusOptimized, SlowSQLStatusVerified})

	err := session.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = session.Order("status_changed_at DESC").Limit(limit).Find(&list).Error
	return list, total, err
}

// ==================== 辅助函数（使用不同名称避免重复声明）====================

func getWeekStartTime(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(t.Year(), t.Month(), t.Day()-weekday+1, 0, 0, 0, 0, t.Location())
}

func getWeekKeyStr(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ==================== 从慢日志报表同步数据 ====================

// SyncFromSlowLogReportResult 同步结果
type SyncFromSlowLogReportResult struct {
	Created int64 `json:"created"` // 新创建的记录数
	Updated int64 `json:"updated"` // 更新的记录数
	Skipped int64 `json:"skipped"` // 跳过的记录数（已存在且状态已推进）
}

// SyncFromSlowLogReport 从 cloud_rds_slowlog_report 表同步数据到跟踪表
// instanceId 为空则同步所有实例
func SyncFromSlowLogReport(c *ctx.Context, instanceId string) (*SyncFromSlowLogReportResult, error) {
	result := &SyncFromSlowLogReportResult{}

	// 查询慢日志报表中的数据（按 sql_hash 聚合，取最新周期的数据）
	var reports []CloudRDSSlowLogReportDB
	session := DB(c).Model(&CloudRDSSlowLogReportDB{})
	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	// 取所有聚合数据（不限制 period_type），按 period_start 降序取最新
	err := session.Order("period_start DESC").
		Find(&reports).Error
	if err != nil {
		return nil, err
	}

	// 按 sql_hash 去重，保留最新的
	reportMap := make(map[string]*CloudRDSSlowLogReportDB)
	for i := range reports {
		r := &reports[i]
		if _, exists := reportMap[r.SqlHash]; !exists {
			reportMap[r.SqlHash] = r
		}
	}

	now := time.Now().Unix()

	for _, report := range reportMap {
		// 检查是否已存在
		existing, err := CloudRDSSlowSQLTrackingGetBySqlHash(c, report.SqlHash)
		if err != nil {
			continue
		}

		if existing != nil {
			// 已存在，检查是否需要更新
			// 如果状态已经是 optimized/verified/ignored，则不更新核心状态
			if existing.Status == SlowSQLStatusOptimized ||
				existing.Status == SlowSQLStatusVerified ||
				existing.Status == SlowSQLStatusIgnored {
				result.Skipped++
				continue
			}

			// 更新统计数据
			updates := map[string]interface{}{
				"last_seen_at":     report.LastSeenAt,
				"total_executions": report.ExecuteCount,
				"avg_time":         report.AvgTime,
				"max_time":         report.MaxTime,
				"this_week_count":  report.ExecuteCount,
				"updated_at":       now,
			}
			if report.SampleSql != "" {
				updates["sample_sql"] = report.SampleSql
			}
			if report.InstanceName != "" {
				updates["instance_name"] = report.InstanceName
			}

			err = DB(c).Model(&CloudRDSSlowSQLTracking{}).Where("id = ?", existing.Id).Updates(updates).Error
			if err == nil {
				result.Updated++
			}
		} else {
			// 新建跟踪记录
			tracking := &CloudRDSSlowSQLTracking{
				SqlHash:         report.SqlHash,
				SqlFingerprint:  report.SqlFingerprint,
				SqlType:         report.SqlType,
				SampleSql:       report.SampleSql,
				Database:        report.Database,
				InstanceId:      report.InstanceId,
				InstanceName:    report.InstanceName,
				Status:          SlowSQLStatusPending,
				Priority:        determinePriority(report.AvgTime, report.ExecuteCount),
				FirstSeenAt:     report.FirstSeenAt,
				LastSeenAt:      report.LastSeenAt,
				TotalExecutions: report.ExecuteCount,
				AvgTime:         report.AvgTime,
				MaxTime:         report.MaxTime,
				ThisWeekCount:   report.ExecuteCount,
				CreatedAt:       now,
				UpdatedAt:       now,
				CreatedBy:       "system",
			}

			err = DB(c).Create(tracking).Error
			if err == nil {
				result.Created++
			}
		}
	}

	return result, nil
}

// determinePriority 根据执行时间和次数自动判定优先级
func determinePriority(avgTime float64, executeCount int64) string {
	// 平均执行时间 > 5s 或 执行次数 > 1000 => 高优先级
	if avgTime > 5.0 || executeCount > 1000 {
		return SlowSQLPriorityHigh
	}
	// 平均执行时间 > 2s 或 执行次数 > 100 => 中优先级
	if avgTime > 2.0 || executeCount > 100 {
		return SlowSQLPriorityMedium
	}
	return SlowSQLPriorityLow
}

// ==================== n9e-2kai: 负责人维度统计 ====================

// OwnerStats 负责人统计数据
type OwnerStats struct {
	Owner          string  `json:"owner"`
	Team           string  `json:"team"`
	Department     string  `json:"department"`
	InstanceCount  int     `json:"instance_count"`
	PendingCount   int64   `json:"pending_count"`
	UrgentCount    int64   `json:"urgent_count"`
	OptimizedCount int64   `json:"optimized_count"`
	IgnoredCount   int64   `json:"ignored_count"`
	TotalCount     int64   `json:"total_count"`
	ThisWeekNew    int64   `json:"this_week_new"`
	ThisWeekDone   int64   `json:"this_week_done"`
	CompletionRate float64 `json:"completion_rate"`
}

// OwnerLeaderboardItem 负责人排行榜项
type OwnerLeaderboardItem struct {
	Rank           int     `json:"rank"`
	Owner          string  `json:"owner"`
	Team           string  `json:"team"`
	DoneCount      int64   `json:"done_count"`
	PendingCount   int64   `json:"pending_count"`
	CompletionRate float64 `json:"completion_rate"`
}

// OwnerWeeklyTrend 负责人周趋势
type OwnerWeeklyTrend struct {
	WeekKey      string `json:"week_key"`
	NewCount     int64  `json:"new_count"`
	DoneCount    int64  `json:"done_count"`
	PendingCount int64  `json:"pending_count"`
	NetChange    int64  `json:"net_change"`
}

// CloudRDSSlowSQLTrackingGetOwnerStats 获取负责人统计数据
// 通过 JOIN cloud_rds_owner 表聚合统计各负责人的慢SQL数据
func CloudRDSSlowSQLTrackingGetOwnerStats(c *ctx.Context, weekOffset int) ([]OwnerStats, error) {
	// 计算目标周的开始时间
	now := time.Now()
	targetDate := now.AddDate(0, 0, weekOffset*7)
	weekStart := getWeekStartTime(targetDate)

	// 使用原生 SQL 进行聚合查询，提高性能
	var results []struct {
		Owner          string
		Team           string
		Department     string
		InstanceCount  int
		PendingCount   int64
		UrgentCount    int64
		OptimizedCount int64
		IgnoredCount   int64
		TotalCount     int64
		ThisWeekNew    int64
		ThisWeekDone   int64
	}

	sql := `
		SELECT 
			o.owner,
			o.team,
			o.department,
			COUNT(DISTINCT o.instance_id) as instance_count,
			SUM(CASE WHEN t.status = 'pending' OR t.status = 'analyzing' OR t.status = 'optimizing' THEN 1 ELSE 0 END) as pending_count,
			SUM(CASE WHEN t.status = 'urgent' THEN 1 ELSE 0 END) as urgent_count,
			SUM(CASE WHEN t.status = 'optimized' OR t.status = 'verified' THEN 1 ELSE 0 END) as optimized_count,
			SUM(CASE WHEN t.status = 'ignored' THEN 1 ELSE 0 END) as ignored_count,
			COUNT(t.id) as total_count,
			SUM(CASE WHEN t.first_seen_at >= ? THEN 1 ELSE 0 END) as this_week_new,
			SUM(CASE WHEN t.status_changed_at >= ? AND (t.status = 'optimized' OR t.status = 'verified') THEN 1 ELSE 0 END) as this_week_done
		FROM cloud_rds_owner o
		LEFT JOIN cloud_rds_slowsql_tracking t ON o.instance_id = t.instance_id
		WHERE o.owner != ''
		GROUP BY o.owner, o.team, o.department
		HAVING COUNT(t.id) > 0
		ORDER BY this_week_done DESC, total_count DESC
	`

	err := DB(c).Raw(sql, weekStart.Unix(), weekStart.Unix()).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// 转换为 OwnerStats 并计算完成率
	var stats []OwnerStats
	for _, r := range results {
		completionRate := float64(0)
		// 完成率 = 已完成 / (已完成 + 待处理 + 紧急)
		total := r.OptimizedCount + r.PendingCount + r.UrgentCount
		if total > 0 {
			completionRate = float64(r.OptimizedCount) / float64(total) * 100
		}

		stats = append(stats, OwnerStats{
			Owner:          r.Owner,
			Team:           r.Team,
			Department:     r.Department,
			InstanceCount:  r.InstanceCount,
			PendingCount:   r.PendingCount,
			UrgentCount:    r.UrgentCount,
			OptimizedCount: r.OptimizedCount,
			IgnoredCount:   r.IgnoredCount,
			TotalCount:     r.TotalCount,
			ThisWeekNew:    r.ThisWeekNew,
			ThisWeekDone:   r.ThisWeekDone,
			CompletionRate: completionRate,
		})
	}

	return stats, nil
}

// CloudRDSSlowSQLTrackingGetOwnerLeaderboard 获取负责人排行榜
// 按 done_count 或 completion_rate 排序返回负责人排行榜
// n9e-2kai: 修改为使用 cloud_rds_slowlog_report 和 cloud_rds_slowsql_status 表
func CloudRDSSlowSQLTrackingGetOwnerLeaderboard(c *ctx.Context, weekOffset int, limit int, sortBy string) ([]OwnerLeaderboardItem, error) {
	if limit <= 0 {
		limit = 10
	}

	// 计算目标周的开始时间
	now := time.Now()
	targetDate := now.AddDate(0, 0, weekOffset*7)
	weekStart := getWeekStartTime(targetDate)

	// 使用原生 SQL 进行聚合查询
	// n9e-2kai: 通过 cloud_rds_owner 关联 cloud_rds_slowlog_report，再关联 cloud_rds_slowsql_status
	var results []struct {
		Owner        string
		Team         string
		DoneCount    int64
		PendingCount int64
		TotalCount   int64
	}

	sql := `
		SELECT 
			o.owner,
			o.team,
			COUNT(DISTINCT CASE WHEN s.status_changed_at >= ? AND (s.status = 'optimized' OR s.status = 'verified') THEN r.sql_hash END) as done_count,
			COUNT(DISTINCT CASE WHEN s.status IS NULL OR s.status = 'pending' OR s.status = 'urgent' OR s.status = 'analyzing' OR s.status = 'optimizing' THEN r.sql_hash END) as pending_count,
			COUNT(DISTINCT r.sql_hash) as total_count
		FROM cloud_rds_owner o
		INNER JOIN cloud_rds_slowlog_report r ON o.instance_id = r.instance_id
		LEFT JOIN cloud_rds_slowsql_status s ON r.sql_hash = s.sql_hash
		WHERE o.owner != '' AND r.period_type = 'day'
		GROUP BY o.owner, o.team
		HAVING COUNT(DISTINCT r.sql_hash) > 0
	`

	// 根据排序字段添加 ORDER BY
	if sortBy == "completion_rate" {
		sql += " ORDER BY (CASE WHEN total_count > 0 THEN done_count * 100.0 / total_count ELSE 0 END) DESC"
	} else {
		sql += " ORDER BY done_count DESC, total_count DESC"
	}

	sql += fmt.Sprintf(" LIMIT %d", limit)

	err := DB(c).Raw(sql, weekStart.Unix()).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// 转换为 OwnerLeaderboardItem 并计算完成率和排名
	var items []OwnerLeaderboardItem
	for i, r := range results {
		completionRate := float64(0)
		if r.TotalCount > 0 {
			completionRate = float64(r.DoneCount) / float64(r.TotalCount) * 100
		}

		items = append(items, OwnerLeaderboardItem{
			Rank:           i + 1,
			Owner:          r.Owner,
			Team:           r.Team,
			DoneCount:      r.DoneCount,
			PendingCount:   r.PendingCount,
			CompletionRate: completionRate,
		})
	}

	return items, nil
}

// CloudRDSSlowSQLTrackingGetOwnerTrend 获取负责人趋势数据
// 返回指定负责人的多周趋势数据，按时间顺序排列（从旧到新）
func CloudRDSSlowSQLTrackingGetOwnerTrend(c *ctx.Context, owner string, weeks int) ([]OwnerWeeklyTrend, error) {
	if weeks <= 0 {
		weeks = 4
	}

	// 首先获取该负责人关联的所有实例 ID
	var instanceIds []string
	err := DB(c).Model(&CloudRDSOwner{}).
		Where("owner = ?", owner).
		Pluck("instance_id", &instanceIds).Error
	if err != nil {
		return nil, err
	}

	if len(instanceIds) == 0 {
		return []OwnerWeeklyTrend{}, nil
	}

	var trends []OwnerWeeklyTrend
	now := time.Now()

	// 从最早的周开始遍历到当前周
	for i := weeks - 1; i >= 0; i-- {
		weekTime := now.AddDate(0, 0, -7*i)
		weekStart := getWeekStartTime(weekTime)
		weekEnd := weekStart.AddDate(0, 0, 7)
		weekKey := getWeekKeyStr(weekTime)

		trend := OwnerWeeklyTrend{WeekKey: weekKey}

		// 本周新增
		session := DB(c).Model(&CloudRDSSlowSQLTracking{})
		session.Where("instance_id IN ?", instanceIds).
			Where("first_seen_at >= ? AND first_seen_at < ?", weekStart.Unix(), weekEnd.Unix()).
			Count(&trend.NewCount)

		// 本周完成
		session2 := DB(c).Model(&CloudRDSSlowSQLTracking{})
		session2.Where("instance_id IN ?", instanceIds).
			Where("status_changed_at >= ? AND status_changed_at < ? AND status IN ?",
				weekStart.Unix(), weekEnd.Unix(),
				[]string{SlowSQLStatusOptimized, SlowSQLStatusVerified}).
			Count(&trend.DoneCount)

		// 待处理（截止到该周末的待处理数）
		session3 := DB(c).Model(&CloudRDSSlowSQLTracking{})
		session3.Where("instance_id IN ?", instanceIds).
			Where("first_seen_at < ?", weekEnd.Unix()).
			Where("status IN ?", []string{SlowSQLStatusPending, SlowSQLStatusUrgent, SlowSQLStatusAnalyzing, SlowSQLStatusOptimizing}).
			Count(&trend.PendingCount)

		trend.NetChange = trend.NewCount - trend.DoneCount
		trends = append(trends, trend)
	}

	return trends, nil
}
