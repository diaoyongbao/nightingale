// n9e-2kai: RDS 负责人管理 + 慢日志统计增强 + CSV导出路由
package router

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

// configCloudRDSOwnerRoutes 配置 RDS 负责人相关路由
func (rt *Router) configCloudRDSOwnerRoutes(pages *gin.RouterGroup) {
	// RDS 负责人管理
	pages.GET("/cloud-management/rds-owners", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsOwnerGets)
	pages.GET("/cloud-management/rds-owner/:instance_id", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsOwnerGet)
	pages.POST("/cloud-management/rds-owner", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsOwnerUpsert)
	pages.DELETE("/cloud-management/rds-owners", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsOwnerDel)

	// 慢日志统计 SQL 指纹搜索（增强现有接口）
	pages.GET("/cloud-management/slowlog-report/search", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudSlowLogReportSearch)

	// CSV 导出
	pages.GET("/cloud-management/slowlog-report/export-csv", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudSlowLogReportExportCSV)
}

// ==================== RDS 负责人管理 ====================

// cloudRdsOwnerGets 获取 RDS 负责人列表
func (rt *Router) cloudRdsOwnerGets(c *gin.Context) {
	query := c.Query("query")
	limit := ginx.QueryInt(c, "limit", 50)
	offset := ginx.QueryInt(c, "offset", 0)

	owners, total, err := models.CloudRDSOwnerGetAll(rt.Ctx, query, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  owners,
		"total": total,
	}, nil)
}

// cloudRdsOwnerGet 获取单个 RDS 负责人
func (rt *Router) cloudRdsOwnerGet(c *gin.Context) {
	instanceId := c.Param("instance_id")
	owner, err := models.CloudRDSOwnerGetByInstanceId(rt.Ctx, instanceId)
	if err != nil {
		// 不存在则返回空
		ginx.NewRender(c).Data(nil, nil)
		return
	}
	ginx.NewRender(c).Data(owner, nil)
}

// cloudRdsOwnerUpsert 创建或更新 RDS 负责人
func (rt *Router) cloudRdsOwnerUpsert(c *gin.Context) {
	var owner models.CloudRDSOwner
	ginx.BindJSON(c, &owner)

	if owner.InstanceId == "" {
		ginx.NewRender(c).Message("instance_id is required")
		return
	}

	user := c.MustGet("user").(*models.User)
	owner.UpdateBy = user.Username

	// 检查是否已存在
	existing, _ := models.CloudRDSOwnerGetByInstanceId(rt.Ctx, owner.InstanceId)
	if existing == nil {
		owner.CreateBy = user.Username
	}

	err := models.CloudRDSOwnerUpsert(rt.Ctx, &owner)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	logger.Infof("User %s upserted RDS owner for instance: %s", user.Username, owner.InstanceId)
	ginx.NewRender(c).Data(gin.H{"instance_id": owner.InstanceId}, nil)
}

// cloudRdsOwnerDel 删除 RDS 负责人
func (rt *Router) cloudRdsOwnerDel(c *gin.Context) {
	var req struct {
		Ids []int64 `json:"ids"`
	}
	ginx.BindJSON(c, &req)

	if len(req.Ids) == 0 {
		ginx.NewRender(c).Message("ids is required")
		return
	}

	err := models.CloudRDSOwnerDel(rt.Ctx, req.Ids)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	user := c.MustGet("user").(*models.User)
	logger.Infof("User %s deleted RDS owners: %v", user.Username, req.Ids)
	ginx.NewRender(c).Message(nil)
}

// ==================== 慢日志统计增强 ====================

// cloudSlowLogReportSearch 支持 SQL 指纹模糊搜索的慢日志查询
func (rt *Router) cloudSlowLogReportSearch(c *gin.Context) {
	instanceId := c.Query("instance_id")
	database := c.Query("database")
	sqlType := c.Query("sql_type")
	sqlFingerprint := c.Query("sql_fingerprint") // 新增：SQL 指纹模糊搜索
	periodType := c.Query("period")
	sortField := c.Query("sort_field")
	sortOrder := c.Query("sort_order")
	limit := ginx.QueryInt(c, "limit", 50)
	offset := ginx.QueryInt(c, "offset", 0)

	// 计算时间范围
	now := time.Now()
	var startTime, endTime time.Time
	dbPeriodType := "day"

	switch periodType {
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startTime = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		endTime = now
	case "month":
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endTime = now
	default:
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	// 支持自定义时间
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = t
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = t
		}
	}

	// 默认排序
	if sortField == "" {
		sortField = "execute_count"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// 从数据库查询（增加 SQL 指纹模糊搜索）
	reports, total, err := cloudRDSSlowLogReportDBSearchByFingerprint(
		rt.Ctx, instanceId, database, sqlType, sqlFingerprint, dbPeriodType,
		startTime.Unix(), endTime.Unix(), sortField, sortOrder, limit, offset,
	)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":       reports,
		"total":      total,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	}, nil)
}

// cloudRDSSlowLogReportDBSearchByFingerprint 支持 SQL 指纹搜索的查询
func cloudRDSSlowLogReportDBSearchByFingerprint(c *ctx.Context, instanceId, database, sqlType, sqlFingerprint, periodType string, periodStart, periodEnd int64, sortField, sortOrder string, limit, offset int) ([]models.CloudRDSSlowLogReportDB, int64, error) {
	var reports []models.CloudRDSSlowLogReportDB
	var total int64

	session := models.DB(c).Model(&models.CloudRDSSlowLogReportDB{})

	if instanceId != "" && instanceId != "all" {
		session = session.Where("instance_id = ?", instanceId)
	}
	if database != "" && database != "all" {
		session = session.Where("database = ?", database)
	}
	if sqlType != "" && sqlType != "all" {
		session = session.Where("sql_type = ?", sqlType)
	}
	if sqlFingerprint != "" {
		// 支持模糊搜索
		session = session.Where("sql_fingerprint LIKE ?", "%"+sqlFingerprint+"%")
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

	session.Count(&total)

	// 排序
	orderClause := "execute_count DESC"
	validSortFields := map[string]string{
		"execute_count":   "execute_count",
		"avg_time":        "avg_time",
		"total_time":      "total_time",
		"total_rows_sent": "total_rows_sent",
		"first_seen_at":   "first_seen_at",
		"last_seen_at":    "last_seen_at",
	}
	if field, ok := validSortFields[sortField]; ok {
		order := "DESC"
		if sortOrder == "asc" {
			order = "ASC"
		}
		orderClause = field + " " + order
	}

	err := session.Order(orderClause).Limit(limit).Offset(offset).Find(&reports).Error
	return reports, total, err
}

// ==================== CSV 导出 ====================

// cloudSlowLogReportExportCSV 导出慢日志统计为 CSV
func (rt *Router) cloudSlowLogReportExportCSV(c *gin.Context) {
	instanceId := c.Query("instance_id")
	database := c.Query("database")
	sqlType := c.Query("sql_type")
	sqlFingerprint := c.Query("sql_fingerprint")
	periodType := c.Query("period")

	// 计算时间范围
	now := time.Now()
	var startTime, endTime time.Time
	dbPeriodType := "day"

	switch periodType {
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startTime = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		endTime = now
	case "month":
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endTime = now
	default:
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	// 支持自定义时间
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = t
		}
	}
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = t
		}
	}

	// 获取数据（最多导出 10000 条）
	reports, _, err := cloudRDSSlowLogReportDBSearchByFingerprint(
		rt.Ctx, instanceId, database, sqlType, sqlFingerprint, dbPeriodType,
		startTime.Unix(), endTime.Unix(), "execute_count", "desc", 10000, 0,
	)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 生成 CSV
	buf := new(bytes.Buffer)
	// 添加 BOM 以支持 Excel 打开
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(buf)

	// 写入表头
	headers := []string{
		"SQL指纹", "SQL类型", "数据库", "实例ID", "实例名称",
		"执行次数", "平均耗时(s)", "最大耗时(s)", "最小耗时(s)",
		"总返回行数", "首次出现", "最后出现", "示例SQL",
	}
	writer.Write(headers)

	// 写入数据
	for _, r := range reports {
		row := []string{
			r.SqlFingerprint,
			r.SqlType,
			r.Database,
			r.InstanceId,
			r.InstanceName,
			strconv.FormatInt(r.ExecuteCount, 10),
			strconv.FormatFloat(r.AvgTime, 'f', 3, 64),
			strconv.FormatFloat(r.MaxTime, 'f', 3, 64),
			strconv.FormatFloat(r.MinTime, 'f', 3, 64),
			strconv.FormatInt(r.TotalRowsSent, 10),
			time.Unix(r.FirstSeenAt, 0).Format("2006-01-02 15:04:05"),
			time.Unix(r.LastSeenAt, 0).Format("2006-01-02 15:04:05"),
			r.SampleSql,
		}
		writer.Write(row)
	}

	writer.Flush()

	// 设置响应头
	filename := fmt.Sprintf("slowlog_report_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(200, "text/csv; charset=utf-8", buf.Bytes())
}
