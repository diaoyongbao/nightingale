package router

import (
	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

// getArcheryClient 动态从数据库获取 Archery 客户端
// 优先从 middleware_datasource 表中获取 archery 类型的数据源
// 如果没有配置，则回退到启动时初始化的 ArcheryClient（从 config.toml）
func (rt *Router) getArcheryClient() (*dbm.ArcheryClient, error) {
	// 使用已有的 GetArcheryClientFromDB 函数获取配置
	config, err := models.GetArcheryClientFromDB(rt.Ctx, "")
	if err != nil {
		logger.Warningf("Failed to get archery config from database: %v", err)
		// 回退到静态配置
		if rt.ArcheryClient != nil {
			return rt.ArcheryClient, nil
		}
		return nil, err
	}

	if config == nil {
		// 没有配置数据源，回退到静态配置
		if rt.ArcheryClient != nil {
			return rt.ArcheryClient, nil
		}
		return nil, nil
	}

	// Debug: 打印配置信息
	logger.Debugf("Archery config from DB: address=%s, auth_type=%s, token_len=%d", 
		config.Address, config.AuthType, len(config.AuthToken))

	return dbm.NewArcheryClient(config)
}

// archeryInstancesGet 获取 Archery 实例列表
func (rt *Router) archeryInstancesGet(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	instances, err := client.GetInstances()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(instances, nil)
}

// archeryHealthCheck Archery 健康检查
func (rt *Router) archeryHealthCheck(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	err = client.HealthCheck()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"status":  "ok",
		"service": "archery",
	}, nil)
}

// archerySessions 获取会话列表(processlist)
func (rt *Router) archerySessions(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcherySessionListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	sessions, err := client.GetSessions(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(sessions, nil)
}

// archeryKillSessions 批量Kill会话
func (rt *Router) archeryKillSessions(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcheryKillSessionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 记录操作日志
	username := c.MustGet("username").(string)
	logger.Infof("User %s killing sessions on instance %d: %v", username, req.InstanceID, req.ThreadIDs)

	err = client.KillSessions(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":      "Sessions killed successfully",
		"killed_count": len(req.ThreadIDs),
	}, nil)
}

// archeryUncommittedTrx 获取未提交事务
func (rt *Router) archeryUncommittedTrx(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcheryUncommittedTrxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	trxList, err := client.GetUncommittedTransactions(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(trxList, nil)
}

// archerySlowQueries 获取慢查询列表
func (rt *Router) archerySlowQueries(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcherySlowQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	queries, err := client.GetSlowQueries(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(queries, nil)
}

// archerySlowQueryDetail 获取慢查询详情
func (rt *Router) archerySlowQueryDetail(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	instanceID := ginx.UrlParamInt64(c, "instance_id")
	checksum := ginx.UrlParamStr(c, "checksum")

	detail, err := client.GetSlowQueryDetail(int(instanceID), checksum)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(detail, nil)
}

// archerySQLQuery 执行SQL查询
func (rt *Router) archerySQLQuery(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcherySQLQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 记录操作日志
	username := c.MustGet("username").(string)
	logger.Infof("User %s executing query on instance %d, db: %s", username, req.InstanceID, req.DBName)

	result, err := client.ExecuteQuery(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(result.Data, nil)
}

// archerySQLCheck SQL语法检测
func (rt *Router) archerySQLCheck(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcherySQLCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	result, err := client.CheckSQL(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(result.Data, nil)
}

// archerySQLWorkflow 提交SQL工单
func (rt *Router) archerySQLWorkflow(c *gin.Context) {
	client, err := rt.getArcheryClient()
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}
	if client == nil {
		ginx.NewRender(c).Message("Archery integration is not enabled")
		return
	}

	var req dbm.ArcherySQLWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 记录操作日志
	username := c.MustGet("username").(string)
	logger.Infof("User %s submitting SQL workflow on instance %d: %s", username, req.InstanceID, req.Title)

	result, err := client.SubmitSQLWorkflow(req)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(result.Data, nil)
}
