// n9e-2kai: 云服务管理路由
package router

import (
	"context"
	"time"

	"github.com/ccfos/nightingale/v6/center/cloudmgmt"
	"github.com/ccfos/nightingale/v6/models"

	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// configCloudManagementRoutes 配置云服务管理路由
func (rt *Router) configCloudManagementRoutes(pages *gin.RouterGroup) {
	// 通用接口
	pages.GET("/cloud-management/providers", rt.cloudProviders)
	pages.GET("/cloud-management/regions", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts"), rt.cloudRegions)

	// 云账号管理
	pages.GET("/cloud-management/accounts", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts"), rt.cloudAccountGets)
	pages.GET("/cloud-management/account/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts"), rt.cloudAccountGet)
	pages.POST("/cloud-management/account", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts/add"), rt.cloudAccountAdd)
	pages.PUT("/cloud-management/account/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts/put"), rt.cloudAccountPut)
	pages.DELETE("/cloud-management/accounts", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts/del"), rt.cloudAccountDel)
	pages.POST("/cloud-management/account/:id/test", rt.auth(), rt.user(), rt.perm("/cloud-management/accounts"), rt.cloudAccountTest)

	// 云主机资源
	pages.GET("/cloud-management/ecs", rt.auth(), rt.user(), rt.perm("/cloud-management/ecs"), rt.cloudEcsGets)
	pages.GET("/cloud-management/ecs/stats", rt.auth(), rt.user(), rt.perm("/cloud-management/ecs"), rt.cloudEcsStats)
	pages.GET("/cloud-management/ecs/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/ecs"), rt.cloudEcsGet)

	// 云数据库资源
	pages.GET("/cloud-management/rds", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsGets)
	pages.GET("/cloud-management/rds/stats", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsStats)
	pages.GET("/cloud-management/rds/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsGet)
	// 注意: GET /cloud-management/rds/:id/slowlogs 接口已废弃，请使用 GET /cloud-management/slowlog-report
	pages.POST("/cloud-management/rds/:id/slowlogs/sync", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudRdsSyncSlowLogs) // n9e-2kai: 手动同步慢日志

	// 资源同步
	pages.POST("/cloud-management/sync", rt.auth(), rt.user(), rt.perm("/cloud-management/sync"), rt.cloudSync)
	pages.POST("/cloud-management/sync/ecs", rt.auth(), rt.user(), rt.perm("/cloud-management/sync"), rt.cloudSyncECS)
	pages.POST("/cloud-management/sync/rds", rt.auth(), rt.user(), rt.perm("/cloud-management/sync"), rt.cloudSyncRDS)
	pages.GET("/cloud-management/sync-logs", rt.auth(), rt.user(), rt.perm("/cloud-management/sync-logs"), rt.cloudSyncLogGets)
	pages.GET("/cloud-management/sync-log/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/sync-logs"), rt.cloudSyncLogGet)

	// 同步配置管理
	pages.GET("/cloud-management/sync-configs", rt.auth(), rt.user(), rt.perm("/cloud-management/settings"), rt.cloudSyncConfigGets)
	pages.GET("/cloud-management/sync-config/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/settings"), rt.cloudSyncConfigGet)
	pages.POST("/cloud-management/sync-config", rt.auth(), rt.user(), rt.perm("/cloud-management/settings/add"), rt.cloudSyncConfigAdd)
	pages.PUT("/cloud-management/sync-config/:id", rt.auth(), rt.user(), rt.perm("/cloud-management/settings/put"), rt.cloudSyncConfigPut)
	pages.DELETE("/cloud-management/sync-configs", rt.auth(), rt.user(), rt.perm("/cloud-management/settings/del"), rt.cloudSyncConfigDel)
	pages.POST("/cloud-management/sync-config/:id/trigger", rt.auth(), rt.user(), rt.perm("/cloud-management/sync"), rt.cloudSyncConfigTrigger)

	// n9e-2kai: 慢日志统计报告
	pages.GET("/cloud-management/slowlog-report", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudSlowLogReport)
	pages.GET("/cloud-management/slowlog-report/summary", rt.auth(), rt.user(), rt.perm("/cloud-management/rds"), rt.cloudSlowLogReportSummary)
}

// ==================== 通用接口 ====================

// cloudProviders 获取支持的云厂商列表
func (rt *Router) cloudProviders(c *gin.Context) {
	ginx.NewRender(c).Data(cloudmgmt.GetSupportedProviders(), nil)
}

// cloudRegions 获取云厂商区域列表
func (rt *Router) cloudRegions(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		ginx.NewRender(c).Message("provider is required")
		return
	}

	// 目前只支持华为云
	if provider == "huawei" {
		regions := cloudmgmt.GetHuaweiRegions()
		ginx.NewRender(c).Data(regions, nil)
		return
	}

	ginx.NewRender(c).Message("unsupported provider: " + provider)
}

// ==================== 云账号管理接口 ====================

// cloudAccountGets 获取云账号列表
func (rt *Router) cloudAccountGets(c *gin.Context) {
	provider := c.Query("provider")
	query := c.Query("query")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	var enabled *bool
	if c.Query("enabled") != "" {
		e := c.Query("enabled") == "true"
		enabled = &e
	}

	accounts, err := models.CloudAccountGets(rt.Ctx, provider, query, enabled, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	total, err := models.CloudAccountCount(rt.Ctx, provider, query, enabled)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  accounts,
		"total": total,
	}, nil)
}

// cloudAccountGet 获取单个云账号
func (rt *Router) cloudAccountGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	account, err := models.CloudAccountGet(rt.Ctx, id)
	ginx.NewRender(c).Data(account, err)
}

// cloudAccountAdd 创建云账号
func (rt *Router) cloudAccountAdd(c *gin.Context) {
	var account models.CloudAccount
	ginx.BindJSON(c, &account)

	user := c.MustGet("user").(*models.User)
	account.CreateBy = user.Username
	account.UpdateBy = user.Username

	// 检查名称是否已存在
	existing, _ := models.CloudAccountGetByName(rt.Ctx, account.Name)
	if existing != nil {
		ginx.NewRender(c).Message("账号名称已存在")
		return
	}

	err := account.Add(rt.Ctx)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"id":   account.Id,
		"name": account.Name,
	}, nil)
}

// cloudAccountPut 更新云账号
func (rt *Router) cloudAccountPut(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	var req models.CloudAccount
	ginx.BindJSON(c, &req)

	account, err := models.CloudAccountGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	user := c.MustGet("user").(*models.User)

	// 更新字段
	account.Name = req.Name
	account.Description = req.Description
	account.RegionList = req.RegionList
	account.DefaultRegion = req.DefaultRegion
	account.SyncEnabled = req.SyncEnabled
	account.SyncInterval = req.SyncInterval
	account.Enabled = req.Enabled
	account.UpdateBy = user.Username

	// 如果提供了新的凭证，则更新
	if req.PlainAccessKey != "" && req.PlainSecretKey != "" {
		if err := account.SetCredentials(req.PlainAccessKey, req.PlainSecretKey); err != nil {
			ginx.NewRender(c).Message(err.Error())
			return
		}
	}

	err = account.Update(rt.Ctx, "name", "description", "regions", "default_region",
		"sync_enabled", "sync_interval", "enabled", "update_by", "update_at",
		"access_key", "secret_key")
	ginx.NewRender(c).Message(err)
}

// cloudAccountDel 批量删除云账号
func (rt *Router) cloudAccountDel(c *gin.Context) {
	var req struct {
		Ids []int64 `json:"ids"`
	}
	ginx.BindJSON(c, &req)

	if len(req.Ids) == 0 {
		ginx.NewRender(c).Message("ids is required")
		return
	}

	// 删除关联的资源
	for _, id := range req.Ids {
		models.CloudECSDelByAccountId(rt.Ctx, id)
		models.CloudRDSDelByAccountId(rt.Ctx, id)
		models.CloudSyncLogDelByAccountId(rt.Ctx, id)
	}

	err := models.CloudAccountDel(rt.Ctx, req.Ids)
	ginx.NewRender(c).Message(err)
}

// cloudAccountTest 测试账号连通性
func (rt *Router) cloudAccountTest(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	account, err := models.CloudAccountGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	manager := cloudmgmt.GetManager()
	if manager == nil {
		ginx.NewRender(c).Message("cloud manager not initialized")
		return
	}

	err = manager.TestConnection(account)
	if err != nil {
		account.UpdateHealthStatus(rt.Ctx, models.CloudHealthFailed, err.Error())
		ginx.NewRender(c).Data(gin.H{
			"success": false,
			"message": err.Error(),
		}, nil)
		return
	}

	account.UpdateHealthStatus(rt.Ctx, models.CloudHealthHealthy, "")
	ginx.NewRender(c).Data(gin.H{
		"success": true,
		"message": "连接成功",
	}, nil)
}

// ==================== 云主机资源接口 ====================

// cloudEcsGets 获取云主机列表
func (rt *Router) cloudEcsGets(c *gin.Context) {
	accountId := ginx.QueryInt64(c, "account_id", 0)
	provider := c.Query("provider")
	region := c.Query("region")
	status := c.Query("status")
	query := c.Query("query")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	ecsList, err := models.CloudECSGets(rt.Ctx, accountId, provider, region, status, query, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	total, err := models.CloudECSCount(rt.Ctx, accountId, provider, region, status, query)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  ecsList,
		"total": total,
	}, nil)
}

// cloudEcsGet 获取云主机详情
func (rt *Router) cloudEcsGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	ecs, err := models.CloudECSGet(rt.Ctx, id)
	ginx.NewRender(c).Data(ecs, err)
}

// cloudEcsStats 获取云主机统计
func (rt *Router) cloudEcsStats(c *gin.Context) {
	accountId := ginx.QueryInt64(c, "account_id", 0)
	provider := c.Query("provider")

	stats, err := models.CloudECSStats(rt.Ctx, accountId, provider)
	ginx.NewRender(c).Data(stats, err)
}

// ==================== 云数据库资源接口 ====================

// cloudRdsGets 获取云数据库列表
func (rt *Router) cloudRdsGets(c *gin.Context) {
	accountId := ginx.QueryInt64(c, "account_id", 0)
	provider := c.Query("provider")
	region := c.Query("region")
	engine := c.Query("engine")
	status := c.Query("status")
	query := c.Query("query")
	owner := c.Query("owner")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	rdsList, err := models.CloudRDSGets(rt.Ctx, accountId, provider, region, engine, status, query, owner, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	total, err := models.CloudRDSCount(rt.Ctx, accountId, provider, region, engine, status, query, owner)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  rdsList,
		"total": total,
	}, nil)
}

// cloudRdsGet 获取云数据库详情
func (rt *Router) cloudRdsGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	rds, err := models.CloudRDSGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Data(nil, err)
		return
	}

	// 获取只读实例
	replicas, _ := models.CloudRDSGetReplicas(rt.Ctx, rds.InstanceId)

	ginx.NewRender(c).Data(gin.H{
		"instance": rds,
		"replicas": replicas,
	}, nil)
}

// cloudRdsStats 获取云数据库统计
func (rt *Router) cloudRdsStats(c *gin.Context) {
	accountId := ginx.QueryInt64(c, "account_id", 0)
	provider := c.Query("provider")

	stats, err := models.CloudRDSStats(rt.Ctx, accountId, provider)
	ginx.NewRender(c).Data(stats, err)
}

// ==================== 资源同步接口 ====================

// cloudSync 触发手动同步
func (rt *Router) cloudSync(c *gin.Context) {
	var req struct {
		AccountId     int64    `json:"account_id"`
		Regions       []string `json:"regions"`
		ResourceTypes []string `json:"resource_types"`
	}
	ginx.BindJSON(c, &req)

	if req.AccountId == 0 {
		ginx.NewRender(c).Message("account_id is required")
		return
	}

	account, err := models.CloudAccountGet(rt.Ctx, req.AccountId)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	if !account.Enabled {
		ginx.NewRender(c).Message("账号已禁用")
		return
	}

	// 默认同步所有资源类型
	if len(req.ResourceTypes) == 0 {
		req.ResourceTypes = []string{"ecs", "rds"}
	}

	manager := cloudmgmt.GetManager()
	if manager == nil {
		ginx.NewRender(c).Message("cloud manager not initialized")
		return
	}

	if manager.IsSyncing(account.Id) {
		ginx.NewRender(c).Message("同步任务正在进行中")
		return
	}

	user := c.MustGet("user").(*models.User)

	// 异步执行同步
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		manager.SyncAccount(ctx, account, req.ResourceTypes, user.Username)
	}()

	ginx.NewRender(c).Data(gin.H{
		"message": "同步任务已启动",
	}, nil)
}

// cloudSyncLogGets 获取同步日志列表
func (rt *Router) cloudSyncLogGets(c *gin.Context) {
	accountId := ginx.QueryInt64(c, "account_id", 0)
	provider := c.Query("provider")
	syncType := c.Query("sync_type")
	startTime := ginx.QueryInt64(c, "start_time", 0)
	endTime := ginx.QueryInt64(c, "end_time", 0)
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	var status *int
	if c.Query("status") != "" {
		s := ginx.QueryInt(c, "status", 0)
		status = &s
	}

	logs, err := models.CloudSyncLogGets(rt.Ctx, accountId, provider, status, syncType, startTime, endTime, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	total, err := models.CloudSyncLogCount(rt.Ctx, accountId, provider, status, syncType, startTime, endTime)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  logs,
		"total": total,
	}, nil)
}

// cloudSyncLogGet 获取同步日志详情
func (rt *Router) cloudSyncLogGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	log, err := models.CloudSyncLogGet(rt.Ctx, id)
	ginx.NewRender(c).Data(log, err)
}

// ==================== ECS/RDS 专项同步接口 ====================

// cloudSyncECS 仅同步 ECS 资源
func (rt *Router) cloudSyncECS(c *gin.Context) {
	rt.doResourceSync(c, []string{"ecs"})
}

// cloudSyncRDS 仅同步 RDS 资源
func (rt *Router) cloudSyncRDS(c *gin.Context) {
	rt.doResourceSync(c, []string{"rds"})
}

// doResourceSync 执行资源同步的通用方法
func (rt *Router) doResourceSync(c *gin.Context, resourceTypes []string) {
	var req struct {
		AccountId int64    `json:"account_id"`
		Regions   []string `json:"regions"`
	}
	ginx.BindJSON(c, &req)

	if req.AccountId == 0 {
		ginx.NewRender(c).Message("account_id is required")
		return
	}

	account, err := models.CloudAccountGet(rt.Ctx, req.AccountId)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	if !account.Enabled {
		ginx.NewRender(c).Message("账号已禁用")
		return
	}

	manager := cloudmgmt.GetManager()
	if manager == nil {
		ginx.NewRender(c).Message("cloud manager not initialized")
		return
	}

	if manager.IsSyncing(account.Id) {
		ginx.NewRender(c).Message("同步任务正在进行中")
		return
	}

	user := c.MustGet("user").(*models.User)

	// 异步执行同步
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		manager.SyncAccount(ctx, account, resourceTypes, user.Username)
	}()

	ginx.NewRender(c).Data(gin.H{
		"message":        "同步任务已启动",
		"resource_types": resourceTypes,
	}, nil)
}

// ==================== 同步配置管理接口 ====================

// cloudSyncConfigGets 获取同步配置列表
func (rt *Router) cloudSyncConfigGets(c *gin.Context) {
	accountId := ginx.QueryInt64(c, "account_id", 0)
	resourceType := c.Query("resource_type")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	configs, err := models.CloudSyncConfigGets(rt.Ctx, accountId, resourceType, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	total, err := models.CloudSyncConfigCount(rt.Ctx, accountId, resourceType)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  configs,
		"total": total,
	}, nil)
}

// cloudSyncConfigGet 获取单个同步配置
func (rt *Router) cloudSyncConfigGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	config, err := models.CloudSyncConfigGet(rt.Ctx, id)
	ginx.NewRender(c).Data(config, err)
}

// cloudSyncConfigAdd 创建同步配置
func (rt *Router) cloudSyncConfigAdd(c *gin.Context) {
	var config models.CloudSyncConfig
	ginx.BindJSON(c, &config)

	user := c.MustGet("user").(*models.User)
	config.CreateBy = user.Username
	config.UpdateBy = user.Username

	// 检查账号是否存在
	account, err := models.CloudAccountGet(rt.Ctx, config.AccountId)
	if err != nil || account == nil {
		ginx.NewRender(c).Message("云账号不存在")
		return
	}

	// 检查同一账号下是否已存在相同资源类型的配置
	existing, _ := models.CloudSyncConfigGetByAccountAndResource(rt.Ctx, config.AccountId, config.ResourceType)
	if existing != nil {
		ginx.NewRender(c).Message("该账号已存在此资源类型的同步配置")
		return
	}

	err = config.Add(rt.Ctx)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"id": config.Id,
	}, nil)
}

// cloudSyncConfigPut 更新同步配置
func (rt *Router) cloudSyncConfigPut(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	var req models.CloudSyncConfig
	ginx.BindJSON(c, &req)

	config, err := models.CloudSyncConfigGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	user := c.MustGet("user").(*models.User)

	// 更新可修改字段
	config.Enabled = req.Enabled
	config.SyncInterval = req.SyncInterval
	config.RegionsList = req.RegionsList
	config.FiltersMap = req.FiltersMap
	config.UpdateBy = user.Username

	err = config.Update(rt.Ctx, "enabled", "sync_interval", "regions", "filters", "update_by", "update_at")
	ginx.NewRender(c).Message(err)
}

// cloudSyncConfigDel 批量删除同步配置
func (rt *Router) cloudSyncConfigDel(c *gin.Context) {
	var req struct {
		Ids []int64 `json:"ids"`
	}
	ginx.BindJSON(c, &req)

	if len(req.Ids) == 0 {
		ginx.NewRender(c).Message("ids is required")
		return
	}

	err := models.CloudSyncConfigDel(rt.Ctx, req.Ids)
	ginx.NewRender(c).Message(err)
}

// cloudSyncConfigTrigger 手动触发同步配置
func (rt *Router) cloudSyncConfigTrigger(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	config, err := models.CloudSyncConfigGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	if config == nil {
		ginx.NewRender(c).Message("同步配置不存在")
		return
	}

	// 获取关联的云账号
	account, err := models.CloudAccountGet(rt.Ctx, config.AccountId)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	if !account.Enabled {
		ginx.NewRender(c).Message("云账号已禁用")
		return
	}

	manager := cloudmgmt.GetManager()
	if manager == nil {
		ginx.NewRender(c).Message("cloud manager not initialized")
		return
	}

	if manager.IsSyncing(account.Id) {
		ginx.NewRender(c).Message("同步任务正在进行中")
		return
	}

	user := c.MustGet("user").(*models.User)
	
	// 解析时间参数
	var req struct {
		StartTime int64 `json:"start_time"`
		EndTime   int64 `json:"end_time"`
	}
	c.ShouldBindJSON(&req)

	// 更新配置状态为同步中
	config.LastSyncStatus = models.SyncStatusRunning
	config.Update(rt.Ctx, "last_sync_status")

	// 异步执行同步，只同步该配置对应的资源类型
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		manager.SyncAccountByConfig(ctx, account, config, user.Username, req.StartTime, req.EndTime)
	}()

	ginx.NewRender(c).Data(gin.H{
		"message":       "同步任务已启动",
		"config_id":     config.Id,
		"resource_type": config.ResourceType,
	}, nil)
}

// ==================== RDS 慢日志接口 ====================
// 注意: 旧的 cloudRdsSlowLogs 接口已废弃并删除
// 新架构使用 cloudSlowLogReport 接口从 cloud_rds_slowlog_report 表查询

// n9e-2kai: cloudRdsSyncSlowLogs 手动触发 RDS 慢日志同步
func (rt *Router) cloudRdsSyncSlowLogs(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	// 获取 RDS 实例信息
	rdsInstance, err := models.CloudRDSGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 获取请求参数
	var req struct {
		StartTime string `json:"start_time"` // 格式: 2026-01-12T00:00:00+08:00
		EndTime   string `json:"end_time"`
	}
	ginx.BindJSON(c, &req)

	// 解析时间参数，默认同步昨天的数据
	var startTime, endTime time.Time
	now := time.Now()

	if req.StartTime != "" && req.EndTime != "" {
		startTime, err = time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			ginx.NewRender(c).Message("invalid start_time format, expected RFC3339")
			return
		}
		endTime, err = time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			ginx.NewRender(c).Message("invalid end_time format, expected RFC3339")
			return
		}
	} else {
		// 默认同步昨天的数据
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	// 获取云管理器
	manager := cloudmgmt.GetManager()
	if manager == nil {
		ginx.NewRender(c).Message("cloud manager not initialized")
		return
	}

	// 异步执行同步
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		// n9e-2kai: 使用 SyncAndAggregateSlowLogs 确保同步后立即聚合生成报表，解决前端查不到数据的问题
		manager.SyncAndAggregateSlowLogs(ctx, rdsInstance, startTime, endTime, "day")
	}()

	ginx.NewRender(c).Data(gin.H{
		"message":       "慢日志同步任务已启动",
		"instance_id":   rdsInstance.InstanceId,
		"instance_name": rdsInstance.InstanceName,
		"start_time":    startTime.Format(time.RFC3339),
		"end_time":      endTime.Format(time.RFC3339),
	}, nil)
}

// ==================== 慢日志统计报告接口 ====================

// n9e-2kai: cloudSlowLogReport 获取慢日志统计报告（从 Report 表查询）
func (rt *Router) cloudSlowLogReport(c *gin.Context) {
	// 获取查询参数
	periodType := c.Query("period") // yesterday, week, month
	sqlType := c.Query("sql_type")
	instanceId := c.Query("instance_id")
	database := c.Query("database")
	sortField := c.Query("sort_field")
	sortOrder := c.Query("sort_order")
	limit := ginx.QueryInt(c, "limit", 50)
	offset := ginx.QueryInt(c, "offset", 0)

	// 默认排序
	if sortField == "" {
		sortField = "execute_count"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// 计算时间范围和周期类型
	now := time.Now()
	var startTime, endTime time.Time
	var dbPeriodType string = "day"

	switch periodType {
	case "yesterday":
		// 昨天 0:00 到 今天 0:00（与 AggregateSlowLogReport 写入的 PeriodEnd 对齐）
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()) // 今天 00:00:00
		dbPeriodType = "day"
	case "today":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endTime = now
		dbPeriodType = "day"
	case "week":
		// 本周一到现在
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startTime = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		endTime = now
		dbPeriodType = "day" // 查询多天的 day 报表
	case "month":
		// 本月1号到现在
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endTime = now
		dbPeriodType = "day" // 查询多天的 day 报表
	default:
		// 默认昨天
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()) // 今天 00:00:00
		dbPeriodType = "day"
	}

	// 也支持自定义时间范围
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

	// 从 Report 表获取数据
	reports, total, err := models.CloudRDSSlowLogReportDBGetByPeriod(
		rt.Ctx,
		instanceId,
		database,
		sqlType,
		dbPeriodType,
		startTime.Unix(),
		endTime.Unix(),
		sortField,
		sortOrder,
		limit,
		offset,
	)
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 转换为前端期望的格式
	type reportResponse struct {
		SqlHash         string  `json:"sql_hash"`
		SqlFingerprint  string  `json:"sql_fingerprint"`
		SqlType         string  `json:"sql_type"`
		Database        string  `json:"database"`
		InstanceId      string  `json:"instance_id"`
		InstanceName    string  `json:"instance_name"`
		TotalExecutions int64   `json:"total_executions"`
		AvgTime         float64 `json:"avg_time"`
		MaxAvgTime      float64 `json:"max_avg_time"`
		MinAvgTime      float64 `json:"min_avg_time"`
		TotalRowsSent   int64   `json:"total_rows_sent"`
		InstanceCount   int64   `json:"instance_count"`
		SampleSql       string  `json:"sample_sql"`
		FirstSeen       string  `json:"first_seen"`
		IsHighFrequency bool    `json:"is_high_frequency"`
		IsSlowGrowing   bool    `json:"is_slow_growing"`
		IsCriticalSlow  bool    `json:"is_critical_slow"`
		GrowthRate      float64 `json:"growth_rate"`
	}

	// 计算平均执行次数用于高频标记
	var totalExec int64
	for _, r := range reports {
		totalExec += r.ExecuteCount
	}
	avgExec := float64(0)
	if len(reports) > 0 {
		avgExec = float64(totalExec) / float64(len(reports))
	}

	responseList := make([]reportResponse, 0, len(reports))
	for _, r := range reports {
		resp := reportResponse{
			SqlHash:         r.SqlHash,
			SqlFingerprint:  r.SqlFingerprint,
			SqlType:         r.SqlType,
			Database:        r.Database,
			InstanceId:      r.InstanceId,
			InstanceName:    r.InstanceName,
			TotalExecutions: r.ExecuteCount,
			AvgTime:         r.AvgTime,
			MaxAvgTime:      r.MaxTime,
			MinAvgTime:      r.MinTime,
			TotalRowsSent:   r.TotalRowsSent,
			InstanceCount:   1, // 当前架构一个实例一条记录
			SampleSql:       r.SampleSql,
		}
		if r.FirstSeenAt > 0 {
			resp.FirstSeen = time.Unix(r.FirstSeenAt, 0).Format(time.RFC3339)
		}
		// 高频标记
		if avgExec > 0 && float64(r.ExecuteCount) > avgExec*3 {
			resp.IsHighFrequency = true
		}
		// 严重慢查询标记
		if r.AvgTime > 1.0 {
			resp.IsCriticalSlow = true
		}
		responseList = append(responseList, resp)
	}

	ginx.NewRender(c).Data(gin.H{
		"list":       responseList,
		"total":      total,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
		"period":     periodType,
	}, nil)
}

// n9e-2kai: cloudSlowLogReportSummary 获取慢日志报告摘要（从 Report 表查询）
func (rt *Router) cloudSlowLogReportSummary(c *gin.Context) {
	// 获取查询参数
	periodType := c.Query("period") // yesterday, week, month
	instanceId := c.Query("instance_id")
	database := c.Query("database")

	// 计算时间范围
	now := time.Now()
	var startTime, endTime time.Time
	var dbPeriodType string = "day"

	switch periodType {
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()) // 今天 00:00:00
		dbPeriodType = "day"
	case "today":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endTime = now
		dbPeriodType = "day"
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startTime = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		endTime = now
		dbPeriodType = "day"
	case "month":
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endTime = now
		dbPeriodType = "day"
	default:
		yesterday := now.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()) // 今天 00:00:00
		dbPeriodType = "day"
	}

	// 也支持自定义时间范围
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

	// 从 Report 表获取摘要数据
	summary, err := models.CloudRDSSlowLogReportDBGetSummary(rt.Ctx, instanceId, database, dbPeriodType, startTime.Unix(), endTime.Unix())
	if err != nil {
		ginx.NewRender(c).Message(err.Error())
		return
	}

	// 获取数据库列表
	databases, _ := models.CloudRDSSlowLogReportDBGetDatabases(rt.Ctx, instanceId, dbPeriodType, startTime.Unix(), endTime.Unix())

	// 获取实例列表
	instances, _ := models.CloudRDSSlowLogReportDBGetInstances(rt.Ctx, dbPeriodType, startTime.Unix(), endTime.Unix())

	ginx.NewRender(c).Data(gin.H{
		"summary":    summary,
		"databases":  databases,
		"instances":  instances,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
		"period":     periodType,
	}, nil)
}
