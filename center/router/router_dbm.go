package router

import (
	"context"
	"errors"
	"time"

	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/gin-gonic/gin"
	"github.com/toolkits/pkg/ginx"
	"github.com/toolkits/pkg/logger"
)

// ==================== 实例管理 ====================

// dbInstanceGets 获取实例列表
func (rt *Router) dbInstanceGets(c *gin.Context) {
	dbType := ginx.QueryStr(c, "db_type", "")
	query := ginx.QueryStr(c, "query", "")
	limit := ginx.QueryInt(c, "limit", 20)
	offset := ginx.QueryInt(c, "p", 1)
	if offset < 1 {
		offset = 1
	}
	offset = (offset - 1) * limit

	total, err := models.DBInstanceCount(rt.Ctx, dbType, query)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instances, err := models.DBInstanceGets(rt.Ctx, dbType, query, limit, offset)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"list":  instances,
		"total": total,
	}, nil)
}

// dbInstanceGet 获取单个实例
func (rt *Router) dbInstanceGet(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	instance, err := models.DBInstanceGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(instance, nil)
}

// dbInstanceAdd 添加实例
func (rt *Router) dbInstanceAdd(c *gin.Context) {
	var instance models.DBInstance
	if err := c.ShouldBindJSON(&instance); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	instance.CreateBy = username
	instance.UpdateBy = username

	// 加密密码 (使用前端传来的 PlainPassword)
	if instance.PlainPassword != "" {
		if err := instance.SetPassword(instance.PlainPassword); err != nil {
			ginx.NewRender(c).Message(err)
			return
		}
	} else {
		ginx.NewRender(c).Message(errors.New("password is required"))
		return
	}

	if err := instance.Add(rt.Ctx); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	logger.Infof("User %s added DB instance: %s", username, instance.InstanceName)
	ginx.NewRender(c).Data(instance.Id, nil)
}

// dbInstancePut 更新实例
func (rt *Router) dbInstancePut(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	var req models.DBInstance
	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	req.Id = instance.Id
	req.CreateAt = instance.CreateAt
	req.CreateBy = instance.CreateBy
	req.UpdateBy = username

	// 如果密码有更新，重新加密 (使用前端传来的 PlainPassword)
	if req.PlainPassword != "" {
		if err := req.SetPassword(req.PlainPassword); err != nil {
			ginx.NewRender(c).Message(err)
			return
		}
	} else {
		req.Password = instance.Password
	}

	if err := req.Update(rt.Ctx, "instance_name", "db_type", "host", "port", "username", "password",
		"charset", "max_connections", "max_idle_conns", "is_master", "enabled", "description",
		"update_at", "update_by"); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 移除旧的连接池
	connManager := dbm.GetGlobalConnectionManager()
	connManager.RemoveConnection(instance.Id)

	logger.Infof("User %s updated DB instance: %s", username, instance.InstanceName)
	ginx.NewRender(c).Message(nil)
}

// dbInstanceDel 删除实例
func (rt *Router) dbInstanceDel(c *gin.Context) {
	var ids []int64
	if err := c.ShouldBindJSON(&ids); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 移除连接池
	connManager := dbm.GetGlobalConnectionManager()
	for _, id := range ids {
		connManager.RemoveConnection(id)
	}

	if err := models.DBInstanceDel(rt.Ctx, ids); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	logger.Infof("User %s deleted DB instances: %v", username, ids)
	ginx.NewRender(c).Message(nil)
}

// dbInstanceHealthCheck 健康检查
func (rt *Router) dbInstanceHealthCheck(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	instance, err := models.DBInstanceGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 创建客户端并检查健康
	dsn, err := instance.GetDSN("")
	if err != nil {
		instance.UpdateHealthStatus(rt.Ctx, models.DBHealthStatusFailed, err.Error())
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := dbm.NewMySQLClient(instance.Id, dsn, instance.MaxConnections, instance.MaxIdleConns)
	if err != nil {
		instance.UpdateHealthStatus(rt.Ctx, models.DBHealthStatusFailed, err.Error())
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		instance.UpdateHealthStatus(rt.Ctx, models.DBHealthStatusFailed, err.Error())
		ginx.NewRender(c).Message(err)
		return
	}

	instance.UpdateHealthStatus(rt.Ctx, models.DBHealthStatusHealthy, "")
	ginx.NewRender(c).Data(gin.H{
		"status":  "healthy",
		"message": "Connection successful",
	}, nil)
}

// dbmGetDatabases 获取实例的数据库列表
func (rt *Router) dbmGetDatabases(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")

	instance, err := models.DBInstanceGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	// 创建客户端
	dsn, err := instance.GetDSN("")
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := dbm.NewMySQLClient(instance.Id, dsn, instance.MaxConnections, instance.MaxIdleConns)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databases, err := client.GetDatabases(ctx)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(databases, nil)
}

// ==================== 会话管理 ====================

// dbmSessions 获取会话列表
func (rt *Router) dbmSessions(c *gin.Context) {
	var req struct {
		InstanceID int64  `json:"instance_id" binding:"required"`
		Command    string `json:"command"`
		User       string `json:"user"`
		DB         string `json:"db"`
		MinTime    int    `json:"min_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := dbm.SessionFilter{
		Command: req.Command,
		User:    req.User,
		DB:      req.DB,
		MinTime: req.MinTime,
	}

	sessions, err := client.GetSessions(ctx, filter)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(sessions, nil)
}

// dbmKillSessions 批量Kill会话
func (rt *Router) dbmKillSessions(c *gin.Context) {
	var req struct {
		InstanceID int64   `json:"instance_id" binding:"required"`
		ThreadIDs  []int64 `json:"thread_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	logger.Infof("User %s killing sessions on instance %d: %v", username, req.InstanceID, req.ThreadIDs)

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	killedCount := 0
	var lastErr error
	for _, threadID := range req.ThreadIDs {
		if err := client.KillSession(ctx, threadID); err != nil {
			logger.Errorf("Failed to kill session %d: %v", threadID, err)
			lastErr = err
		} else {
			killedCount++
		}
	}

	if lastErr != nil && killedCount == 0 {
		ginx.NewRender(c).Message(lastErr)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"message":      "Sessions killed",
		"killed_count": killedCount,
		"total_count":  len(req.ThreadIDs),
	}, nil)
}

// ==================== SQL查询工作台 ====================

// dbmExecuteSQL 执行SQL
func (rt *Router) dbmExecuteSQL(c *gin.Context) {
	var req struct {
		InstanceID int64  `json:"instance_id" binding:"required"`
		DBName     string `json:"db_name"`
		SQLContent string `json:"sql_content" binding:"required"`
		LimitNum   int    `json:"limit_num"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	username := c.MustGet("username").(string)
	logger.Infof("User %s executing SQL on instance %d, db: %s", username, req.InstanceID, req.DBName)

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	execReq := dbm.SQLExecuteRequest{
		DB:       req.DBName,
		SQL:      req.SQLContent,
		LimitNum: req.LimitNum,
	}

	result, err := client.ExecuteSQL(ctx, execReq)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(result, nil)
}

// dbmCheckSQL SQL语法检查
func (rt *Router) dbmCheckSQL(c *gin.Context) {
	var req struct {
		InstanceID int64  `json:"instance_id" binding:"required"`
		DBName     string `json:"db_name"`
		SQLContent string `json:"sql_content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.CheckSQL(ctx, req.SQLContent); err != nil {
		ginx.NewRender(c).Data(gin.H{
			"valid":   false,
			"message": err.Error(),
		}, nil)
		return
	}

	ginx.NewRender(c).Data(gin.H{
		"valid":   true,
		"message": "SQL syntax is valid",
	}, nil)
}

// ==================== 未提交事务 ====================

// dbmUncommittedTransactions 获取未提交事务
func (rt *Router) dbmUncommittedTransactions(c *gin.Context) {
	var req struct {
		InstanceID int64 `json:"instance_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	transactions, err := client.GetUncommittedTransactions(ctx)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(transactions, nil)
}

// ==================== 锁信息 ====================

// dbmLockWaits 获取锁等待信息
func (rt *Router) dbmLockWaits(c *gin.Context) {
	var req struct {
		InstanceID int64 `json:"instance_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lockWaits, err := client.GetLockWaits(ctx)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(lockWaits, nil)
}

// dbmInnoDBLocks 获取 InnoDB 锁信息
func (rt *Router) dbmInnoDBLocks(c *gin.Context) {
	instanceID := ginx.QueryInt64(c, "instance_id", 0)
	if instanceID <= 0 {
		ginx.NewRender(c).Message("instance_id is required")
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, instanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	locks, err := client.GetInnoDBLocks(ctx)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(locks, nil)
}

// ==================== 慢查询 ====================

// ==================== 表和列信息 ====================

// dbmGetTables 获取数据库的表列表
func (rt *Router) dbmGetTables(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	dbName := c.Param("db")

	instance, err := models.DBInstanceGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tables, err := client.GetTables(ctx, dbName)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(tables, nil)
}

// dbmGetTableColumns 获取表的列信息
func (rt *Router) dbmGetTableColumns(c *gin.Context) {
	id := ginx.UrlParamInt64(c, "id")
	dbName := c.Param("db")
	tableName := c.Param("table")

	instance, err := models.DBInstanceGet(rt.Ctx, id)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	columns, err := client.GetTableColumns(ctx, dbName, tableName)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(columns, nil)
}

// dbmSlowQueries 获取慢查询列表
func (rt *Router) dbmSlowQueries(c *gin.Context) {
	var req struct {
		InstanceID int64  `json:"instance_id" binding:"required"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
		DBName     string `json:"db_name"`
		Limit      int    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	instance, err := models.DBInstanceGet(rt.Ctx, req.InstanceID)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	client, err := getDBClient(instance)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := dbm.SlowQueryFilter{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		DBName:    req.DBName,
		Limit:     req.Limit,
	}

	slowQueries, err := client.GetSlowQueries(ctx, filter)
	if err != nil {
		ginx.NewRender(c).Message(err)
		return
	}

	ginx.NewRender(c).Data(slowQueries, nil)
}
