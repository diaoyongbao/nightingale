package router

import (
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// ==================== 哨兵规则管理 ====================

// dbaSentinelRuleGets 获取哨兵规则列表
func (rt *Router) dbaSentinelRuleGets(c *gin.Context) {
	query := ginx.QueryStr(c, "query", "")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "p", 1)
	if offset < 1 {
		offset = 1
	}
	offset = (offset - 1) * limit

	total, err := models.DBASentinelRuleCount(rt.Ctx, query)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	rules, err := models.DBASentinelRuleGets(rt.Ctx, query, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  rules,
		"total": total,
	}, nil)
}

// dbaSentinelRuleGet 获取单个哨兵规则
func (rt *Router) dbaSentinelRuleGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	rule, err := models.DBASentinelRuleGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(rule, nil)
}

// dbaSentinelRuleAdd 添加哨兵规则
func (rt *Router) dbaSentinelRuleAdd(c *gin.Context) {
	var rule models.DBASentinelRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	rule.CreateBy = username
	rule.UpdateBy = username

	if err := rule.Add(rt.Ctx); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(rule.Id, nil)
}

// dbaSentinelRulePut 更新哨兵规则
func (rt *Router) dbaSentinelRulePut(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	var req models.DBASentinelRule
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	rule, err := models.DBASentinelRuleGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	req.Id = rule.Id
	req.CreateAt = rule.CreateAt
	req.CreateBy = rule.CreateBy
	req.UpdateBy = username

	if err := req.Update(rt.Ctx, "name", "description", "instance_id", "enabled", "rule_type",
		"max_time", "match_user", "match_db", "match_sql", "match_command", "match_state",
		"exclude_user", "exclude_db", "exclude_sql", "action",
		"notify_channel_ids", "notify_user_group_ids", "check_interval", "update_at", "update_by"); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Message(nil)
}

// dbaSentinelRuleDel 删除哨兵规则
func (rt *Router) dbaSentinelRuleDel(c *gin.Context) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	if err := models.DBASentinelRuleDel(rt.Ctx, ids); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Message(nil)
}

// dbaSentinelRuleStatusPut 更新规则状态
func (rt *Router) dbaSentinelRuleStatusPut(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	rule, err := models.DBASentinelRuleGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	rule.Enabled = req.Enabled
	rule.UpdateBy = username

	if err := rule.Update(rt.Ctx, "enabled", "update_at", "update_by"); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Message(nil)
}

// ==================== Kill 日志管理 ====================

// dbaSentinelKillLogGets 获取 Kill 日志列表
func (rt *Router) dbaSentinelKillLogGets(c *gin.Context) {
	ruleId := ginx.QueryInt64(c, "rule_id", 0)
	instanceId := ginx.QueryInt64(c, "instance_id", 0)
	startTime := ginx.QueryInt64(c, "start_time", 0)
	endTime := ginx.QueryInt64(c, "end_time", 0)
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "p", 1)
	if offset < 1 {
		offset = 1
	}
	offset = (offset - 1) * limit

	total, err := models.DBASentinelKillLogCount(rt.Ctx, ruleId, instanceId, startTime, endTime)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	logs, err := models.DBASentinelKillLogGets(rt.Ctx, ruleId, instanceId, startTime, endTime, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  logs,
		"total": total,
	}, nil)
}

// dbaSentinelKillLogStats 获取规则统计信息
func (rt *Router) dbaSentinelKillLogStats(c *gin.Context) {
	ruleId := ginx.UrlParamInt64(c, "id")
	days := ginx.QueryInt(c, "days", 7)

	stats, err := models.DBASentinelKillLogStatsByRuleId(rt.Ctx, ruleId, days)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(stats, nil)
}

// ==================== 锁信息查询 ====================

// 锁信息查询已移至 router_dbm.go

// ==================== 哨兵状态 ====================

// dbaSentinelStatus 获取哨兵运行状态
func (rt *Router) dbaSentinelStatus(c *gin.Context) {
	// 获取所有启用的规则
	rules, err := models.DBASentinelRuleGetsEnabled(rt.Ctx)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 统计最近24小时的 kill 数量
	stats := make(map[string]interface{})
	stats["enabled_rules"] = len(rules)
	stats["running"] = true // 哨兵默认在运行

	// 计算最近24小时的统计
	now := time.Now().Unix()
	startTime := now - 86400 // 24小时前

	totalKills, err := models.DBASentinelKillLogCount(rt.Ctx, 0, 0, startTime, now)
	if err == nil {
		stats["kills_24h"] = totalKills
	}

	ginx.NewRender(c).Data(stats, nil)
}
