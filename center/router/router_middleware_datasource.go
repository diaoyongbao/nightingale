package router

import (
	"strconv"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
)

// middlewareDatasourcesGets 获取中间件数据源列表
func (rt *Router) middlewareDatasourcesGets(c *gin.Context) {
	middlewareType := ginx.QueryStr(c, "type", "")
	status := ginx.QueryStr(c, "status", "")
	keyword := ginx.QueryStr(c, "keyword", "")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "offset", 0)

	list, err := models.GetMiddlewareDatasourcesBy(rt.Ctx, middlewareType, status, keyword, limit, offset)
	ginx.NewRender(c).Data(list, err)
}

// middlewareDatasourceGet 获取单个中间件数据源
func (rt *Router) middlewareDatasourceGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	ds, err := models.MiddlewareDatasourceGet(rt.Ctx, id)
	ginx.NewRender(c).Data(ds, err)
}

// middlewareDatasourceAdd 添加中间件数据源
func (rt *Router) middlewareDatasourceAdd(c *gin.Context) {
	var req models.MiddlewareDatasource
	ginx.BindJSON(c, &req)

	username := c.MustGet("username").(string)
	req.CreatedBy = username
	req.UpdatedBy = username

	// 验证认证配置
	err := req.ValidateAuthConfig()
	ginx.Dangerous(err)

	err = req.Add(rt.Ctx)
	ginx.NewRender(c).Message(err)
}

// middlewareDatasourcePut 更新中间件数据源
func (rt *Router) middlewareDatasourcePut(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	
	var req models.MiddlewareDatasource
	ginx.BindJSON(c, &req)

	// 设置 ID
	req.Id = id

	username := c.MustGet("username").(string)
	req.UpdatedBy = username

	// 验证认证配置
	err := req.ValidateAuthConfig()
	ginx.Dangerous(err)

	err = req.Update(rt.Ctx, "*")
	ginx.NewRender(c).Message(err)
}

// middlewareDatasourceDel 删除中间件数据源
func (rt *Router) middlewareDatasourceDel(c *gin.Context) {
	var req idsForm
	ginx.BindJSON(c, &req)
	ginx.NewRender(c).Message(models.MiddlewareDatasourceDel(rt.Ctx, req.Ids))
}

// middlewareDatasourceTestConnection 测试连接
func (rt *Router) middlewareDatasourceTestConnection(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	ds, err := models.MiddlewareDatasourceGet(rt.Ctx, id)
	ginx.Dangerous(err)

	if !ds.IsEnabled() {
		ginx.Dangerous("middleware datasource is disabled")
	}

	// 根据类型测试连接
	var testErr error
	switch ds.Type {
	case models.MiddlewareTypeArchery:
		// 测试 Archery 连接
		config, err := models.GetMiddlewareDatasourceAsArcheryConfig(rt.Ctx, id)
		ginx.Dangerous(err)

		client, err := rt.createArcheryClient(config)
		ginx.Dangerous(err)

		testErr = client.HealthCheck()

	default:
		ginx.Dangerous("unsupported middleware type: " + ds.Type)
	}

	if testErr != nil {
		// 更新健康状态为不健康
		ds.UpdateHealthStatus(rt.Ctx, models.HealthStatusUnhealthy, testErr.Error())
		ginx.NewRender(c).Message(testErr)
		return
	}

	// 更新健康状态为健康
	ds.UpdateHealthStatus(rt.Ctx, models.HealthStatusHealthy, "连接成功")
	ginx.NewRender(c).Data(gin.H{
		"status":  "success",
		"message": "连接测试成功",
	}, nil)
}

// middlewareDatasourceTypes 获取支持的中间件类型及数量
func (rt *Router) middlewareDatasourceTypes(c *gin.Context) {
	typeMap, err := models.GetMiddlewareDatasourceTypes(rt.Ctx)
	ginx.NewRender(c).Data(gin.H{
		"types": []gin.H{
			{"value": models.MiddlewareTypeArchery, "label": "Archery SQL审核", "count": typeMap[models.MiddlewareTypeArchery]},
			{"value": models.MiddlewareTypeJumpServer, "label": "JumpServer 堡垒机", "count": typeMap[models.MiddlewareTypeJumpServer]},
			{"value": models.MiddlewareTypeJenkins, "label": "Jenkins CI/CD", "count": typeMap[models.MiddlewareTypeJenkins]},
			{"value": models.MiddlewareTypeGitLab, "label": "GitLab 代码仓库", "count": typeMap[models.MiddlewareTypeGitLab]},
			{"value": models.MiddlewareTypeNacos, "label": "Nacos 配置中心", "count": typeMap[models.MiddlewareTypeNacos]},
			{"value": models.MiddlewareTypeConsul, "label": "Consul 服务发现", "count": typeMap[models.MiddlewareTypeConsul]},
		},
	}, err)
}

// middlewareDatasourceMigrateArchery 从配置文件迁移 Archery
func (rt *Router) middlewareDatasourceMigrateArchery(c *gin.Context) {
	// 从配置读取 Archery 配置
	archeryConf := rt.getArcheryConfigFromEnv()
	if archeryConf == nil {
		ginx.NewRender(c).Message("archery configuration not found in config file")
		return
	}

	err := models.MigrateArcheryConfigToDB(rt.Ctx, *archeryConf)
	ginx.NewRender(c).Message(err)
}

// middlewareDatasourceGetByType 根据类型获取中间件数据源
func (rt *Router) middlewareDatasourceGetByType(c *gin.Context) {
	middlewareType := ginx.QueryStr(c, "type", "")
	if middlewareType == "" {
		ginx.Dangerous("type parameter is required")
	}

	list, err := models.GetEnabledMiddlewareDatasourcesByType(rt.Ctx, middlewareType)
	ginx.NewRender(c).Data(list, err)
}

// middlewareDatasourceCount 获取中间件数据源数量
func (rt *Router) middlewareDatasourceCount(c *gin.Context) {
	middlewareType := ginx.QueryStr(c, "type", "")
	status := ginx.QueryStr(c, "status", "")
	keyword := ginx.QueryStr(c, "keyword", "")

	count, err := models.GetMiddlewareDatasourcesCount(rt.Ctx, middlewareType, status, keyword)
	ginx.NewRender(c).Data(gin.H{"count": count}, err)
}

// middlewareDatasourceUpdateStatus 批量更新状态
func (rt *Router) middlewareDatasourceUpdateStatus(c *gin.Context) {
	var req struct {
		Ids    []int64 `json:"ids"`
		Status string  `json:"status"`
	}
	ginx.BindJSON(c, &req)

	if req.Status != models.MiddlewareStatusEnabled && req.Status != models.MiddlewareStatusDisabled {
		ginx.Dangerous("invalid status value")
	}

	username := c.MustGet("username").(string)
	for _, id := range req.Ids {
		ds, err := models.MiddlewareDatasourceGet(rt.Ctx, id)
		if err != nil {
			continue
		}
		ds.Status = req.Status
		ds.UpdatedBy = username
		ds.Update(rt.Ctx, "status", "updated_by", "updated_at")
	}

	ginx.NewRender(c).Message(nil)
}

// middlewareDatasourceExport 导出配置
func (rt *Router) middlewareDatasourceExport(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	ds, err := models.MiddlewareDatasourceGet(rt.Ctx, id)
	ginx.Dangerous(err)

	// 清除敏感信息后导出
	ds.ClearPlaintext()

	c.Header("Content-Disposition", "attachment; filename=middleware_"+strconv.FormatInt(id, 10)+".json")
	c.Header("Content-Type", "application/json")
	ginx.NewRender(c).Data(ds, nil)
}
