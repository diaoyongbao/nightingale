// n9e-2kai: 慢SQL优化跟踪路由（基于关联表设计）
package router

import (
	"strconv"
	"time"

	"github.com/ccfos/nightingale/v6/models"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// configSlowSQLTrackingRoutes 配置慢SQL跟踪相关路由
func (rt *Router) configSlowSQLTrackingRoutes(pages *gin.RouterGroup) {
	// 注意：特定路径必须放在 :id 参数路由之前

	// 优化看板统计
	pages.GET("/cloud-management/slowsql-tracking/stats", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingStats)
	pages.GET("/cloud-management/slowsql-tracking/trend", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingTrend)

	// n9e-2kai: 负责人维度统计
	pages.GET("/cloud-management/slowsql-tracking/owner-stats", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingOwnerStats)
	pages.GET("/cloud-management/slowsql-tracking/owner-leaderboard", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingOwnerLeaderboard)
	pages.GET("/cloud-management/slowsql-tracking/owner-trend", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingOwnerTrend)
	pages.GET("/cloud-management/slowsql-tracking/owner-trends-all", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingOwnerTrendsAll)

	// 周报告
	pages.GET("/cloud-management/slowsql-tracking/weekly-report", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingWeeklyReport)

	// 批量操作
	pages.POST("/cloud-management/slowsql-tracking/batch-status", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingBatchStatus)

	// 列表查询（核心：JOIN 查询）
	pages.GET("/cloud-management/slowsql-tracking", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingGets)

	// 按 sql_hash 操作（不再使用 :id）
	pages.PUT("/cloud-management/slowsql-tracking/status", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingUpdateStatus)
	pages.PUT("/cloud-management/slowsql-tracking/assign", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingAssign)
	pages.PUT("/cloud-management/slowsql-tracking/update", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingUpdate)

	// 状态变更日志
	pages.GET("/cloud-management/slowsql-tracking/logs", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.slowSQLTrackingLogs)
}

// slowSQLTrackingGets 获取慢SQL列表（带跟踪状态）
func (rt *Router) slowSQLTrackingGets(c *gin.Context) {
	instanceId := c.Query("instance_id")
	status := c.Query("status")
	priority := c.Query("priority")
	owner := c.Query("owner")
	query := c.Query("query")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	list, total, err := models.CloudRDSSlowSQLWithStatusGets(rt.Ctx, instanceId, status, priority, owner, query, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  list,
		"total": total,
	}, nil)
}

// slowSQLTrackingUpdateStatus 更新状态
func (rt *Router) slowSQLTrackingUpdateStatus(c *gin.Context) {
	var req struct {
		SqlHash string `json:"sql_hash" binding:"required"`
		Status  string `json:"status" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	username := c.GetString("username")
	err := models.CloudRDSSlowSQLStatusUpdateStatus(rt.Ctx, req.SqlHash, req.Status, username, req.Comment)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Message("")
}

// slowSQLTrackingAssign 指派负责人
func (rt *Router) slowSQLTrackingAssign(c *gin.Context) {
	var req struct {
		SqlHash    string `json:"sql_hash" binding:"required"`
		Owner      string `json:"owner" binding:"required"`
		OwnerEmail string `json:"owner_email"`
		Team       string `json:"team"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	username := c.GetString("username")
	updates := map[string]interface{}{
		"owner":       req.Owner,
		"owner_email": req.OwnerEmail,
		"team":        req.Team,
	}
	err := models.CloudRDSSlowSQLStatusUpdate(rt.Ctx, req.SqlHash, updates, username)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Message("")
}

// slowSQLTrackingUpdate 更新跟踪信息
func (rt *Router) slowSQLTrackingUpdate(c *gin.Context) {
	var req struct {
		SqlHash        string `json:"sql_hash" binding:"required"`
		Priority       string `json:"priority"`
		Owner          string `json:"owner"`
		OwnerEmail     string `json:"owner_email"`
		Team           string `json:"team"`
		OptimizeNote   string `json:"optimize_note"`
		OptimizeResult string `json:"optimize_result"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	username := c.GetString("username")
	updates := map[string]interface{}{}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Owner != "" {
		updates["owner"] = req.Owner
	}
	if req.OwnerEmail != "" {
		updates["owner_email"] = req.OwnerEmail
	}
	if req.Team != "" {
		updates["team"] = req.Team
	}
	if req.OptimizeNote != "" {
		updates["optimize_note"] = req.OptimizeNote
	}
	if req.OptimizeResult != "" {
		updates["optimize_result"] = req.OptimizeResult
	}

	err := models.CloudRDSSlowSQLStatusUpdate(rt.Ctx, req.SqlHash, updates, username)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Message("")
}

// slowSQLTrackingBatchStatus 批量更新状态
func (rt *Router) slowSQLTrackingBatchStatus(c *gin.Context) {
	var req struct {
		SqlHashes []string `json:"sql_hashes" binding:"required"`
		Status    string   `json:"status" binding:"required"`
		Comment   string   `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	username := c.GetString("username")
	var errCount int
	for _, sqlHash := range req.SqlHashes {
		err := models.CloudRDSSlowSQLStatusUpdateStatus(rt.Ctx, sqlHash, req.Status, username, req.Comment)
		if err != nil {
			errCount++
		}
	}

	ginx.NewRender(c).Data(gin.H{
		"success": len(req.SqlHashes) - errCount,
		"failed":  errCount,
	}, nil)
}

// slowSQLTrackingStats 获取统计数据
func (rt *Router) slowSQLTrackingStats(c *gin.Context) {
	instanceId := c.Query("instance_id")
	owner := c.Query("owner") // n9e-2kai: 支持按负责人筛选

	stats, err := models.CloudRDSSlowSQLStatusGetStats(rt.Ctx, instanceId, owner)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(stats, nil)
}

// slowSQLTrackingTrend 获取趋势数据
// n9e-2kai: 使用 CloudRDSSlowSQLStatusGetTrend 函数，基于正确的表查询数据
func (rt *Router) slowSQLTrackingTrend(c *gin.Context) {
	instanceId := c.Query("instance_id")
	owner := c.Query("owner")
	weeks := ginx.QueryInt(c, "weeks", 4)
	if weeks > 12 {
		weeks = 12
	}

	// n9e-2kai: 使用新的趋势查询函数，支持按实例和负责人筛选
	trends, err := models.CloudRDSSlowSQLStatusGetTrend(rt.Ctx, instanceId, owner, weeks)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 转换为 map 格式
	var result []map[string]interface{}
	for _, t := range trends {
		result = append(result, map[string]interface{}{
			"week_key":      t.WeekKey,
			"new_count":     t.NewCount,
			"done_count":    t.DoneCount,
			"pending_count": t.PendingCount,
			"net_change":    t.NetChange,
		})
	}

	ginx.NewRender(c).Data(gin.H{
		"trends": result,
	}, nil)
}

// slowSQLTrackingWeeklyReport 生成周优化报告
// n9e-2kai: 增强版周报告，包含负责人统计和趋势数据
func (rt *Router) slowSQLTrackingWeeklyReport(c *gin.Context) {
	instanceId := c.Query("instance_id")
	instanceName := c.Query("instance_name")

	stats, err := models.CloudRDSSlowSQLStatusGetStats(rt.Ctx, instanceId, "")
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	now := time.Now()
	year, week := now.ISOWeek()
	weekKey := strconv.Itoa(year) + "-W" + strconv.Itoa(week)
	if week < 10 {
		weekKey = strconv.Itoa(year) + "-W0" + strconv.Itoa(week)
	}

	// 生成 Markdown 报告
	markdown := "# 慢SQL优化周报 - " + weekKey + "\n\n"
	if instanceName != "" {
		markdown += "**实例**: " + instanceName + " (" + instanceId + ")\n\n"
	}
	markdown += "## 📊 本周概览\n\n"
	markdown += "| 指标 | 数量 |\n"
	markdown += "|------|------|\n"
	markdown += "| 本周新增 | " + strconv.FormatInt(stats.ThisWeekNew, 10) + " |\n"
	markdown += "| 本周完成 | " + strconv.FormatInt(stats.ThisWeekDone, 10) + " |\n"
	netChange := stats.ThisWeekNew - stats.ThisWeekDone
	netChangeStr := strconv.FormatInt(netChange, 10)
	if netChange > 0 {
		netChangeStr = "+" + netChangeStr + " ⚠️"
	} else if netChange < 0 {
		netChangeStr = netChangeStr + " ✅"
	}
	markdown += "| 净变化 | " + netChangeStr + " |\n"
	markdown += "| 待处理总数 | " + strconv.FormatInt(stats.PendingCount+stats.UrgentCount, 10) + " |\n"
	markdown += "| 高优先级待处理 | " + strconv.FormatInt(stats.HighPriorityPending, 10) + " |\n\n"

	markdown += "## 🎯 状态分布\n\n"
	markdown += "| 状态 | 数量 |\n"
	markdown += "|------|------|\n"
	markdown += "| 待评估 | " + strconv.FormatInt(stats.PendingCount, 10) + " |\n"
	markdown += "| 紧急 | " + strconv.FormatInt(stats.UrgentCount, 10) + " |\n"
	markdown += "| 已优化 | " + strconv.FormatInt(stats.OptimizedCount, 10) + " |\n"
	markdown += "| 已忽略 | " + strconv.FormatInt(stats.IgnoredCount, 10) + " |\n\n"

	// n9e-2kai: 负责人统计（使用新函数）
	ownerStats, ownerErr := models.CloudRDSSlowSQLStatusGetOwnerStats(rt.Ctx, 0)
	if ownerErr == nil && len(ownerStats) > 0 {
		markdown += "## 👥 负责人统计\n\n"
		markdown += "| 负责人 | 团队 | 实例数 | 本周新增 | 本周完成 | 待处理 | 完成率 |\n"
		markdown += "|--------|------|--------|----------|----------|--------|--------|\n"
		for _, os := range ownerStats {
			completionRateStr := strconv.FormatFloat(os.CompletionRate, 'f', 1, 64) + "%"
			pendingTotal := strconv.FormatInt(os.PendingCount+os.UrgentCount, 10)
			markdown += "| " + os.Owner + " | " + os.Team + " | " +
				strconv.Itoa(os.InstanceCount) + " | " +
				strconv.FormatInt(os.ThisWeekNew, 10) + " | " +
				strconv.FormatInt(os.ThisWeekDone, 10) + " | " +
				pendingTotal + " | " + completionRateStr + " |\n"
		}
		markdown += "\n"
	}

	// n9e-2kai: 近4周趋势
	trends, trendErr := models.CloudRDSSlowSQLStatusGetTrend(rt.Ctx, instanceId, "", 4)
	if trendErr == nil && len(trends) > 0 {
		markdown += "## 📈 近4周趋势\n\n"
		markdown += "| 周 | 新增 | 完成 | 净变化 |\n"
		markdown += "|------|------|------|--------|\n"
		for _, t := range trends {
			changeStr := strconv.FormatInt(t.NetChange, 10)
			if t.NetChange > 0 {
				changeStr = "⚠️ +" + changeStr
			} else if t.NetChange < 0 {
				changeStr = "✅ " + changeStr
			}
			markdown += "| " + t.WeekKey + " | " + strconv.FormatInt(t.NewCount, 10) + " | " +
				strconv.FormatInt(t.DoneCount, 10) + " | " + changeStr + " |\n"
		}
		markdown += "\n"
	}

	markdown += "---\n"
	markdown += "*报告生成时间: " + now.Format("2006-01-02 15:04:05") + "*\n"

	ginx.NewRender(c).Data(gin.H{
		"markdown":    markdown,
		"week_key":    weekKey,
		"instance_id": instanceId,
	}, nil)
}

// slowSQLTrackingLogs 获取状态变更日志
func (rt *Router) slowSQLTrackingLogs(c *gin.Context) {
	sqlHash := c.Query("sql_hash")
	if sqlHash == "" {
		ginx.NewRender(c).Message("sql_hash is required")
		return
	}

	var logs []models.CloudRDSSlowSQLStatusLog
	err := models.DB(rt.Ctx).Where("sql_hash = ?", sqlHash).Order("created_at DESC").Find(&logs).Error
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list": logs,
	}, nil)
}


// ==================== n9e-2kai: 负责人维度统计 ====================

// slowSQLTrackingOwnerStats 获取负责人统计数据
// n9e-2kai: 使用新的 CloudRDSSlowSQLStatusGetOwnerStats 函数
func (rt *Router) slowSQLTrackingOwnerStats(c *gin.Context) {
	weekOffset := ginx.QueryInt(c, "week_offset", 0)

	stats, err := models.CloudRDSSlowSQLStatusGetOwnerStats(rt.Ctx, weekOffset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  stats,
		"total": len(stats),
	}, nil)
}

// slowSQLTrackingOwnerLeaderboard 获取负责人排行榜
// n9e-2kai: 基于 CloudRDSSlowSQLStatusGetOwnerStats 生成排行榜
func (rt *Router) slowSQLTrackingOwnerLeaderboard(c *gin.Context) {
	weekOffset := ginx.QueryInt(c, "week_offset", 0)
	limit := ginx.QueryInt(c, "limit", 10)
	sortBy := c.Query("sort_by") // done_count 或 completion_rate

	stats, err := models.CloudRDSSlowSQLStatusGetOwnerStats(rt.Ctx, weekOffset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 根据排序字段排序
	type LeaderboardItem struct {
		Rank           int     `json:"rank"`
		Owner          string  `json:"owner"`
		Team           string  `json:"team"`
		DoneCount      int64   `json:"done_count"`
		PendingCount   int64   `json:"pending_count"`
		CompletionRate float64 `json:"completion_rate"`
	}

	var items []LeaderboardItem
	for _, s := range stats {
		items = append(items, LeaderboardItem{
			Owner:          s.Owner,
			Team:           s.Team,
			DoneCount:      s.ThisWeekDone,
			PendingCount:   s.PendingCount + s.UrgentCount,
			CompletionRate: s.CompletionRate,
		})
	}

	// 排序
	if sortBy == "completion_rate" {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].CompletionRate > items[i].CompletionRate {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	} else {
		// 默认按 done_count 排序
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].DoneCount > items[i].DoneCount {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	}

	// 添加排名并限制数量
	if limit > len(items) {
		limit = len(items)
	}
	result := items[:limit]
	for i := range result {
		result[i].Rank = i + 1
	}

	ginx.NewRender(c).Data(gin.H{
		"list": result,
	}, nil)
}

// slowSQLTrackingOwnerTrend 获取负责人趋势数据
// n9e-2kai: 使用新的 CloudRDSSlowSQLStatusGetOwnerTrend 函数
func (rt *Router) slowSQLTrackingOwnerTrend(c *gin.Context) {
	owner := c.Query("owner")
	if owner == "" {
		ginx.NewRender(c).Message("owner is required")
		return
	}

	weeks := ginx.QueryInt(c, "weeks", 4)
	if weeks > 12 {
		weeks = 12
	}

	trends, err := models.CloudRDSSlowSQLStatusGetOwnerTrend(rt.Ctx, owner, weeks)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"owner":  owner,
		"trends": trends,
	}, nil)
}

// slowSQLTrackingOwnerTrendsAll 获取所有负责人的趋势汇总数据
// n9e-2kai: 用于趋势图同时展示多个负责人的数据
func (rt *Router) slowSQLTrackingOwnerTrendsAll(c *gin.Context) {
	weeks := ginx.QueryInt(c, "weeks", 4)
	if weeks > 12 {
		weeks = 12
	}

	// 获取所有负责人统计
	stats, err := models.CloudRDSSlowSQLStatusGetOwnerStats(rt.Ctx, 0)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 获取总体趋势
	overallTrends, err := models.CloudRDSSlowSQLStatusGetTrend(rt.Ctx, "", "", weeks)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 转换总体趋势格式
	var overallData []map[string]interface{}
	for _, t := range overallTrends {
		overallData = append(overallData, map[string]interface{}{
			"week_key":      t.WeekKey,
			"new_count":     t.NewCount,
			"done_count":    t.DoneCount,
			"pending_count": t.PendingCount,
			"net_change":    t.NetChange,
		})
	}

	// 获取每个负责人的趋势数据
	var ownerTrends []map[string]interface{}
	for _, s := range stats {
		if s.Owner == "" {
			continue
		}
		trends, trendErr := models.CloudRDSSlowSQLStatusGetOwnerTrend(rt.Ctx, s.Owner, weeks)
		if trendErr != nil {
			continue
		}

		// 汇总该负责人的趋势数据
		var trendData []map[string]interface{}
		for _, t := range trends {
			trendData = append(trendData, map[string]interface{}{
				"week_key":      t.WeekKey,
				"new_count":     t.NewCount,
				"done_count":    t.DoneCount,
				"pending_count": t.PendingCount,
				"net_change":    t.NetChange,
			})
		}

		ownerTrends = append(ownerTrends, map[string]interface{}{
			"owner":  s.Owner,
			"team":   s.Team,
			"trends": trendData,
		})
	}

	ginx.NewRender(c).Data(gin.H{
		"overall": overallData,
		"owners":  ownerTrends,
	}, nil)
}
