package cron

import (
	"time"

	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// MiddlewareHealthChecker 中间件健康检查器
type MiddlewareHealthChecker struct {
	ctx      *ctx.Context
	interval time.Duration
	quit     chan struct{}
}

// NewMiddlewareHealthChecker 创建健康检查器
func NewMiddlewareHealthChecker(ctx *ctx.Context, interval time.Duration) *MiddlewareHealthChecker {
	if interval == 0 {
		interval = 60 * time.Second // 默认 60 秒
	}

	return &MiddlewareHealthChecker{
		ctx:      ctx,
		interval: interval,
		quit:     make(chan struct{}),
	}
}

// Start 启动健康检查
func (m *MiddlewareHealthChecker) Start() {
	logger.Info("middleware health checker started")

	// 立即执行一次
	go m.checkAllMiddleware()

	ticker := time.NewTicker(m.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.checkAllMiddleware()
			case <-m.quit:
				ticker.Stop()
				logger.Info("middleware health checker stopped")
				return
			}
		}
	}()
}

// Stop 停止健康检查
func (m *MiddlewareHealthChecker) Stop() {
	close(m.quit)
}

// checkAllMiddleware 检查所有中间件
func (m *MiddlewareHealthChecker) checkAllMiddleware() {
	list, err := models.GetMiddlewareDatasources(m.ctx)
	if err != nil {
		logger.Errorf("failed to get middleware datasources: %v", err)
		return
	}

	for _, ds := range list {
		// 只检查启用的数据源
		if !ds.IsEnabled() {
			continue
		}

		// 检查是否到了检查时间
		now := time.Now().Unix()
		if ds.LastHealthCheck > 0 && now-ds.LastHealthCheck < int64(ds.HealthCheckInterval) {
			continue
		}

		// 执行健康检查
		go m.checkMiddleware(ds)
	}
}

// checkMiddleware 检查单个中间件
func (m *MiddlewareHealthChecker) checkMiddleware(ds *models.MiddlewareDatasource) {
	var status string
	var message string

	switch ds.Type {
	case models.MiddlewareTypeArchery:
		status, message = m.checkArchery(ds)
	case models.MiddlewareTypeJumpServer:
		status, message = m.checkJumpServer(ds)
	case models.MiddlewareTypeJenkins:
		status, message = m.checkJenkins(ds)
	default:
		status = models.HealthStatusUnknown
		message = "unsupported middleware type: " + ds.Type
	}

	// 更新健康状态
	err := ds.UpdateHealthStatus(m.ctx, status, message)
	if err != nil {
		logger.Errorf("failed to update health status for %s: %v", ds.Name, err)
	}

	// 记录健康检查结果
	if status == models.HealthStatusHealthy {
		logger.Debugf("health check passed: %s (%s)", ds.Name, ds.Type)
	} else {
		logger.Warningf("health check failed: %s (%s) - %s", ds.Name, ds.Type, message)
	}
}

// checkArchery 检查 Archery 健康状态
func (m *MiddlewareHealthChecker) checkArchery(ds *models.MiddlewareDatasource) (string, string) {
	config, err := models.GetMiddlewareDatasourceAsArcheryConfig(m.ctx, ds.Id)
	if err != nil {
		return models.HealthStatusUnhealthy, "failed to get config: " + err.Error()
	}

	client, err := dbm.NewArcheryClient(config)
	if err != nil {
		return models.HealthStatusUnhealthy, "failed to create client: " + err.Error()
	}

	err = client.HealthCheck()
	if err != nil {
		return models.HealthStatusUnhealthy, err.Error()
	}

	return models.HealthStatusHealthy, "服务正常"
}

// checkJumpServer 检查 JumpServer 健康状态
func (m *MiddlewareHealthChecker) checkJumpServer(ds *models.MiddlewareDatasource) (string, string) {
	// TODO: 实现 JumpServer 健康检查
	// 这里只是示例,实际需要根据 JumpServer API 实现
	return models.HealthStatusUnknown, "JumpServer health check not implemented yet"
}

// checkJenkins 检查 Jenkins 健康状态
func (m *MiddlewareHealthChecker) checkJenkins(ds *models.MiddlewareDatasource) (string, string) {
	// TODO: 实现 Jenkins 健康检查
	// 这里只是示例,实际需要根据 Jenkins API 实现
	return models.HealthStatusUnknown, "Jenkins health check not implemented yet"
}

// ScheduleMiddlewareHealthCheck 启动中间件健康检查定时任务
func ScheduleMiddlewareHealthCheck(ctx *ctx.Context, interval time.Duration) *MiddlewareHealthChecker {
	checker := NewMiddlewareHealthChecker(ctx, interval)
	checker.Start()
	return checker
}
