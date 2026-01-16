// n9e-2kai: 云服务管理模块 - 同步管理器
package cloudmgmt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/center/cloudmgmt/huawei"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// Manager 云服务管理器
type Manager struct {
	ctx       *ctx.Context
	providers map[string]ProviderFactory

	// 同步任务控制
	syncMutex sync.Mutex
	syncing   map[int64]bool // account_id -> is_syncing
}

// ProviderFactory 云服务提供者工厂函数
type ProviderFactory func(ak, sk string, regions []string) CloudProvider

// NewManager 创建管理器
func NewManager(c *ctx.Context) *Manager {
	m := &Manager{
		ctx:       c,
		providers: make(map[string]ProviderFactory),
		syncing:   make(map[int64]bool),
	}

	return m
}

// RegisterProvider 注册云服务提供者
func (m *Manager) RegisterProvider(name string, factory ProviderFactory) {
	m.providers[name] = factory
}

// GetProvider 获取云服务提供者实例
func (m *Manager) GetProvider(account *models.CloudAccount) (CloudProvider, error) {
	factory, ok := m.providers[account.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", account.Provider)
	}

	// 解密凭证
	ak, sk, err := account.GetCredentials()
	if err != nil {
		return nil, fmt.Errorf("get credentials failed: %w", err)
	}

	// 解析区域列表
	var regions []string
	if account.Regions != "" {
		json.Unmarshal([]byte(account.Regions), &regions)
	}

	return factory(ak, sk, regions), nil
}

// TestConnection 测试账号连接
func (m *Manager) TestConnection(account *models.CloudAccount) error {
	provider, err := m.GetProvider(account)
	if err != nil {
		return err
	}

	return provider.TestConnection(context.Background())
}

// IsSyncing 检查账号是否正在同步
func (m *Manager) IsSyncing(accountId int64) bool {
	m.syncMutex.Lock()
	defer m.syncMutex.Unlock()
	return m.syncing[accountId]
}

// SyncAccount 同步单个账号的资源
func (m *Manager) SyncAccount(bgCtx context.Context, account *models.CloudAccount, resourceTypes []string, operator string) (*models.CloudSyncLog, error) {
	// 检查是否正在同步
	m.syncMutex.Lock()
	if m.syncing[account.Id] {
		m.syncMutex.Unlock()
		return nil, fmt.Errorf("account %s is syncing", account.Name)
	}
	m.syncing[account.Id] = true
	m.syncMutex.Unlock()

	defer func() {
		m.syncMutex.Lock()
		delete(m.syncing, account.Id)
		m.syncMutex.Unlock()
	}()

	// 创建同步日志
	syncLog := &models.CloudSyncLog{
		AccountId:     account.Id,
		AccountName:   account.Name,
		Provider:      account.Provider,
		SyncType:      models.CloudSyncTypeManual,
		ResourceTypes: strings.Join(resourceTypes, ","),
		Status:        models.CloudSyncLogStatusRunning,
		StartTime:     time.Now().Unix(),
		Operator:      operator,
	}
	if err := syncLog.Add(m.ctx); err != nil {
		return nil, err
	}

	// 获取云服务提供者
	provider, err := m.GetProvider(account)
	if err != nil {
		syncLog.Status = models.CloudSyncLogStatusFailed
		syncLog.ErrorMessage = err.Error()
		syncLog.EndTime = time.Now().Unix()
		syncLog.Update(m.ctx)
		return syncLog, err
	}

	// 解析区域列表
	var regions []string
	json.Unmarshal([]byte(account.Regions), &regions)

	var syncErrors []string

	// 同步 ECS
	if contains(resourceTypes, "ecs") {
		result, err := m.syncECS(bgCtx, account, provider, regions)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("ECS: %v", err))
		} else {
			syncLog.EcsTotal = result.Total
			syncLog.EcsAdded = result.Added
			syncLog.EcsUpdated = result.Updated
			syncLog.EcsDeleted = result.Deleted
		}
	}

	// 同步 RDS
	if contains(resourceTypes, "rds") {
		result, err := m.syncRDS(bgCtx, account, provider, regions)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("RDS: %v", err))
		} else {
			syncLog.RdsTotal = result.Total
			syncLog.RdsAdded = result.Added
			syncLog.RdsUpdated = result.Updated
			syncLog.RdsDeleted = result.Deleted
		}
	}

	// 更新同步日志
	syncLog.EndTime = time.Now().Unix()
	syncLog.Duration = int(syncLog.EndTime - syncLog.StartTime)

	if len(syncErrors) > 0 {
		if len(syncErrors) == len(resourceTypes) {
			syncLog.Status = models.CloudSyncLogStatusFailed
		} else {
			syncLog.Status = models.CloudSyncLogStatusPartialSuccess
		}
		syncLog.ErrorMessage = strings.Join(syncErrors, "; ")
	} else {
		syncLog.Status = models.CloudSyncLogStatusSuccess
	}

	syncLog.Update(m.ctx)

	// 更新账号同步状态
	account.LastSyncTime = syncLog.EndTime
	account.LastSyncStatus = syncLog.Status
	if syncLog.Status == models.CloudSyncLogStatusFailed {
		account.LastSyncError = syncLog.ErrorMessage
	} else {
		account.LastSyncError = ""
	}
	account.Update(m.ctx, "last_sync_time", "last_sync_status", "last_sync_error")

	return syncLog, nil
}

// syncECS 同步 ECS 资源
func (m *Manager) syncECS(bgCtx context.Context, account *models.CloudAccount, provider CloudProvider, regions []string) (*SyncResult, error) {
	result := &SyncResult{}

	// 获取现有实例ID列表
	existingIds, _ := models.CloudECSGetInstanceIdsByAccountId(m.ctx, account.Id)
	existingIdSet := make(map[string]bool)
	for _, id := range existingIds {
		existingIdSet[id] = true
	}
	syncedIds := make(map[string]bool)

	for _, region := range regions {
		ecsList, err := provider.ListECS(bgCtx, region, nil)
		if err != nil {
			logger.Warningf("list ECS failed for region %s: %v", region, err)
			result.Errors = append(result.Errors, err)
			continue
		}

		for _, ecs := range ecsList {
			ecs.AccountId = account.Id
			syncedIds[ecs.InstanceId] = true

			// 查找是否存在
			existing, _ := models.CloudECSGetByInstanceId(m.ctx, account.Provider, ecs.InstanceId)
			if existing != nil {
				// 更新
				ecs.Id = existing.Id
				ecs.Update(m.ctx)
				result.Updated++
			} else {
				// 新增
				ecs.Add(m.ctx)
				result.Added++
			}
			result.Total++
		}
	}

	// 删除不存在的资源
	var toDelete []string
	for id := range existingIdSet {
		if !syncedIds[id] {
			toDelete = append(toDelete, id)
		}
	}
	if len(toDelete) > 0 {
		models.CloudECSDelByProvider(m.ctx, account.Provider, toDelete)
		result.Deleted = len(toDelete)
	}

	return result, nil
}

// syncRDS 同步 RDS 资源
func (m *Manager) syncRDS(bgCtx context.Context, account *models.CloudAccount, provider CloudProvider, regions []string) (*SyncResult, error) {
	result := &SyncResult{}

	// 获取现有实例ID列表
	existingIds, _ := models.CloudRDSGetInstanceIdsByAccountId(m.ctx, account.Id)
	existingIdSet := make(map[string]bool)
	for _, id := range existingIds {
		existingIdSet[id] = true
	}
	syncedIds := make(map[string]bool)

	for _, region := range regions {
		rdsList, err := provider.ListRDS(bgCtx, region, nil)
		if err != nil {
			logger.Warningf("list RDS failed for region %s: %v", region, err)
			result.Errors = append(result.Errors, err)
			continue
		}

		for _, rds := range rdsList {
			rds.AccountId = account.Id
			syncedIds[rds.InstanceId] = true

			// 查找是否存在
			existing, _ := models.CloudRDSGetByInstanceId(m.ctx, account.Provider, rds.InstanceId)
			if existing != nil {
				// 更新
				rds.Id = existing.Id
				rds.Update(m.ctx)
				result.Updated++
			} else {
				// 新增
				rds.Add(m.ctx)
				result.Added++
			}
			result.Total++
		}
	}

	// 删除不存在的资源
	var toDelete []string
	for id := range existingIdSet {
		if !syncedIds[id] {
			toDelete = append(toDelete, id)
		}
	}
	if len(toDelete) > 0 {
		models.CloudRDSDelByProvider(m.ctx, account.Provider, toDelete)
		result.Deleted = len(toDelete)
	}

	return result, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// n9e-2kai: SyncRDSSlowLogs 同步指定 RDS 实例的慢日志明细到 Detail 表
// 新架构：先同步明细，再由单独任务聚合生成报表
// 注意：华为云 API 有时间限制，每次请求最多查询 24 小时的数据，因此需要按天循环调用
func (m *Manager) SyncRDSSlowLogs(bgCtx context.Context, rds *models.CloudRDS, startTime, endTime time.Time) (int, error) {
	// 获取云账号
	account, err := models.CloudAccountGet(m.ctx, rds.AccountId)
	if err != nil {
		return 0, fmt.Errorf("get account failed: %w", err)
	}

	// 获取云服务提供者
	provider, err := m.GetProvider(account)
	if err != nil {
		return 0, fmt.Errorf("get provider failed: %w", err)
	}

	// 目前只支持华为云
	huaweiAdapter, ok := provider.(*HuaweiAdapter)
	if !ok {
		return 0, fmt.Errorf("slow log sync only supports Huawei Cloud")
	}

	// n9e-2kai: 按天循环调用华为云 API，因为 API 有 24 小时时间限制
	totalLogs := 0
	currentStart := startTime
	loc := startTime.Location()

	for currentStart.Before(endTime) {
		// 计算当天的结束时间（当天 23:59:59 或 endTime，取较小者）
		currentEnd := time.Date(currentStart.Year(), currentStart.Month(), currentStart.Day(), 23, 59, 59, 999999999, loc)
		if currentEnd.After(endTime) {
			currentEnd = endTime
		}

		// 格式化时间 - 华为云要求格式: yyyy-mm-ddThh:mm:ss+0800 (时区无冒号)
		startTimeStr := currentStart.Format("2006-01-02T15:04:05-0700")
		endTimeStr := currentEnd.Format("2006-01-02T15:04:05-0700")

		logger.Infof("syncing slow logs for RDS %s, date range: %s to %s", rds.InstanceName, startTimeStr, endTimeStr)

		// 分页获取慢日志明细 - 使用 lineNum 进行分页
		var dayLogs []huawei.SlowLogDetail
		var lineNum string = ""
		const pageSize int32 = 100
		loopCount := 0

		for {
			slowLogs, lastLineNum, err := huaweiAdapter.ListSlowLogDetails(
				bgCtx,
				rds.Region,
				rds.InstanceId,
				startTimeStr,
				endTimeStr,
				"",       // 不过滤数据库
				"",       // 不过滤 SQL 类型
				pageSize, // 每页最多 100 条
				lineNum,
			)
			if err != nil {
				logger.Warningf("list slow log details failed for date %s: %v", currentStart.Format("2006-01-02"), err)
				break // 跳过当天错误，继续下一天
			}

			if len(slowLogs) == 0 {
				break
			}

			dayLogs = append(dayLogs, slowLogs...)

			// 如果返回数量小于 pageSize 或没有 lastLineNum，说明已经是最后一页
			if int32(len(slowLogs)) < pageSize || lastLineNum == "" {
				break
			}
			lineNum = lastLineNum
			loopCount++

			// 防止无限循环的安全限制
			if loopCount >= 100 {
				logger.Warningf("RDS %s has more than 10000 slow logs on %s, synced %d records", rds.InstanceName, currentStart.Format("2006-01-02"), len(dayLogs))
				break
			}
		}

		// 如果当天有数据，保存到数据库
		if len(dayLogs) > 0 {
			startUnix := currentStart.Unix()
			endUnix := currentEnd.Unix()

			// 删除该时间范围的旧明细数据，避免重复
			if err := models.CloudRDSSlowLogDetailDeleteByTimeRange(m.ctx, rds.InstanceId, startUnix, endUnix); err != nil {
				logger.Warningf("delete old slow logs failed for date %s: %v", currentStart.Format("2006-01-02"), err)
			}

			// 批量保存明细
			details := make([]models.CloudRDSSlowLogDetail, 0, len(dayLogs))
			for _, sl := range dayLogs {
				executedAt := parseSlowLogTime(sl.StartTime)

				detail := models.CloudRDSSlowLogDetail{
					AccountId:    rds.AccountId,
					InstanceId:   rds.InstanceId,
					InstanceName: rds.InstanceName,
					Region:       rds.Region,
					SqlText:      sl.SqlText,
					SqlType:      sl.SqlType,
					Database:     sl.Database,
					ExecutedAt:   executedAt,
					ExecuteTime:  sl.ExecuteTime,
					LockTime:     sl.LockTime,
					RowsExamined: int64(sl.RowsExamined),
					RowsSent:     int64(sl.RowsSent),
					Users:        sl.Users,
					ClientIP:     sl.ClientIP,
					SyncTime:     time.Now().Unix(),
				}
				details = append(details, detail)
			}

			if err := models.CloudRDSSlowLogDetailBatchAdd(m.ctx, details); err != nil {
				logger.Warningf("batch add slow logs failed for date %s: %v", currentStart.Format("2006-01-02"), err)
			} else {
				logger.Infof("synced %d slow logs for RDS %s on %s", len(details), rds.InstanceName, currentStart.Format("2006-01-02"))
				totalLogs += len(details)
			}
		} else {
			logger.Infof("no slow logs found for RDS %s on %s", rds.InstanceName, currentStart.Format("2006-01-02"))
		}

		// 移动到下一天
		currentStart = time.Date(currentStart.Year(), currentStart.Month(), currentStart.Day()+1, 0, 0, 0, 0, loc)
	}

	if totalLogs == 0 {
		logger.Infof("no slow logs found for RDS %s in time range %s - %s", rds.InstanceName, startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
	}

	return totalLogs, nil
}

// parseSlowLogTime 解析华为云返回的时间字符串为 Unix 时间戳
// 注意：华为云 API 返回的无时区时间实际是 UTC 时间
func parseSlowLogTime(timeStr string) int64 {
	if timeStr == "" {
		return 0
	}

	// 华为云返回格式可能有多种：
	// - 2026-01-12T10:30:00+0800
	// - 2026-01-12T10:30:00+08:00
	// - 2026-01-12T10:30:00Z
	// - 2026-01-12T10:30:00 (无时区，华为云实际返回的是 UTC 时间)
	// - 2026-01-12 10:30:00
	layouts := []string{
		"2006-01-02T15:04:05+0800",  // 华为云常见格式（无冒号时区）
		"2006-01-02T15:04:05-0700",  // 通用无冒号时区格式
		"2006-01-02T15:04:05+08:00", // 有冒号时区格式
		"2006-01-02T15:04:05Z07:00", // RFC3339 格式
		time.RFC3339,                // Go 标准 RFC3339
		"2006-01-02T15:04:05Z",      // UTC 格式
		"2006-01-02T15:04:05",       // 无时区格式（华为云返回的 UTC 时间）
		"2006-01-02 15:04:05",       // 空格分隔格式
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return t.Unix()
		}
	}

	// 解析失败时输出警告
	logger.Warningf("failed to parse slow log time: '%s'", timeStr)
	return 0
}

// n9e-2kai: AggregateSlowLogReport 从 Detail 表聚合生成 Report 表
// periodType: "day", "week", "month"
func (m *Manager) AggregateSlowLogReport(bgCtx context.Context, rds *models.CloudRDS, periodType string, periodStart, periodEnd time.Time) error {
	startUnix := periodStart.Unix()
	endUnix := periodEnd.Unix()

	// 获取该时间范围内的明细数据
	details, err := models.CloudRDSSlowLogDetailGetByTimeRange(m.ctx, rds.InstanceId, startUnix, endUnix)
	if err != nil {
		return fmt.Errorf("get slow log details failed: %w", err)
	}

	if len(details) == 0 {
		logger.Infof("no slow log details found for RDS %s in period %s", rds.InstanceName, periodType)
		return nil
	}

	// 按 SQL 指纹聚合
	type aggregatedData struct {
		SqlText       string
		SqlType       string
		Database      string
		TotalTime     float64
		TotalLockTime float64
		TotalRowsSent int64
		TotalRowsExam int64
		ExecuteCount  int64
		MinTime       float64
		MaxTime       float64
		FirstSeenAt   int64
		LastSeenAt    int64
	}
	aggregatedMap := make(map[string]*aggregatedData)

	for _, d := range details {
		fingerprint := models.GenerateSQLFingerprint(d.SqlText)
		sqlHash := models.GenerateSQLHash(fingerprint)

		if ag, exists := aggregatedMap[sqlHash]; exists {
			ag.TotalTime += d.ExecuteTime
			ag.TotalLockTime += d.LockTime
			ag.TotalRowsSent += d.RowsSent
			ag.TotalRowsExam += d.RowsExamined
			ag.ExecuteCount++
			if d.ExecuteTime > ag.MaxTime {
				ag.MaxTime = d.ExecuteTime
			}
			if d.ExecuteTime < ag.MinTime {
				ag.MinTime = d.ExecuteTime
			}
			if d.ExecutedAt > ag.LastSeenAt {
				ag.LastSeenAt = d.ExecutedAt
			}
			if d.ExecutedAt < ag.FirstSeenAt {
				ag.FirstSeenAt = d.ExecutedAt
			}
		} else {
			aggregatedMap[sqlHash] = &aggregatedData{
				SqlText:       d.SqlText,
				SqlType:       d.SqlType,
				Database:      d.Database,
				TotalTime:     d.ExecuteTime,
				TotalLockTime: d.LockTime,
				TotalRowsSent: d.RowsSent,
				TotalRowsExam: d.RowsExamined,
				ExecuteCount:  1,
				MinTime:       d.ExecuteTime,
				MaxTime:       d.ExecuteTime,
				FirstSeenAt:   d.ExecutedAt,
				LastSeenAt:    d.ExecutedAt,
			}
		}
	}

	// 计算总执行次数
	var totalExecutions int64 = 0
	for _, ag := range aggregatedMap {
		totalExecutions += ag.ExecuteCount
	}

	// 删除该周期的旧报表数据
	if err := models.CloudRDSSlowLogReportDBDeleteByPeriod(m.ctx, rds.InstanceId, periodType, startUnix); err != nil {
		logger.Warningf("delete old report data failed: %v", err)
	}

	// 生成报表记录
	reports := make([]models.CloudRDSSlowLogReportDB, 0, len(aggregatedMap))
	for sqlHash, ag := range aggregatedMap {
		fingerprint := models.GenerateSQLFingerprint(ag.SqlText)

		report := models.CloudRDSSlowLogReportDB{
			AccountId:      rds.AccountId,
			RdsId:          rds.Id,
			InstanceId:     rds.InstanceId,
			InstanceName:   rds.InstanceName,
			Provider:       rds.Provider,
			Region:         rds.Region,
			SqlHash:        sqlHash,
			SqlFingerprint: fingerprint,
			SqlType:        ag.SqlType,
			Database:       ag.Database,
			SampleSql:      ag.SqlText,
			ExecuteCount:   ag.ExecuteCount,
			TotalTime:      ag.TotalTime,
			AvgTime:        ag.TotalTime / float64(ag.ExecuteCount),
			MaxTime:        ag.MaxTime,
			MinTime:        ag.MinTime,
			TotalLockTime:  ag.TotalLockTime,
			AvgLockTime:    ag.TotalLockTime / float64(ag.ExecuteCount),
			TotalRowsSent:  ag.TotalRowsSent,
			AvgRowsSent:    ag.TotalRowsSent / ag.ExecuteCount,
			TotalRowsExam:  ag.TotalRowsExam,
			AvgRowsExam:    ag.TotalRowsExam / ag.ExecuteCount,
			ExecuteRatio:   float64(ag.ExecuteCount) / float64(totalExecutions) * 100,
			FirstSeenAt:    ag.FirstSeenAt,
			LastSeenAt:     ag.LastSeenAt,
			PeriodType:     periodType,
			PeriodStart:    startUnix,
			PeriodEnd:      endUnix,
		}
		reports = append(reports, report)
	}

	if err := models.CloudRDSSlowLogReportDBBatchUpsert(m.ctx, reports); err != nil {
		return fmt.Errorf("batch insert reports failed: %w", err)
	}

	logger.Infof("aggregated %d slow log reports for RDS %s (period: %s)", len(reports), rds.InstanceName, periodType)
	return nil
}

// n9e-2kai: SyncAndAggregateSlowLogs 同步明细并生成报表（一站式调用）
func (m *Manager) SyncAndAggregateSlowLogs(bgCtx context.Context, rds *models.CloudRDS, startTime, endTime time.Time, periodType string) (int, error) {
	// 步骤1: 同步明细
	count, err := m.SyncRDSSlowLogs(bgCtx, rds, startTime, endTime)
	if err != nil {
		return 0, err
	}

	// 步骤2: 聚合生成报表
	if err := m.AggregateSlowLogReport(bgCtx, rds, periodType, startTime, endTime); err != nil {
		logger.Warningf("aggregate slow log report failed for RDS %s: %v", rds.InstanceName, err)
		// 聚合失败不影响同步结果
	}

	return count, nil
}

// n9e-2kai: SyncAllRDSSlowLogs 同步所有 RDS 实例的慢日志并生成报表（每日定时任务调用）
func (m *Manager) SyncAllRDSSlowLogs(bgCtx context.Context, startTime, endTime time.Time, periodType string) {
	// 获取所有启用的云账号
	accounts, err := models.CloudAccountGets(m.ctx, "", "", nil, 0, 0)
	if err != nil {
		logger.Errorf("get cloud accounts failed: %v", err)
		return
	}

	for _, account := range accounts {
		if !account.Enabled {
			continue
		}

		// 只处理华为云账号
		if account.Provider != "huawei" {
			continue
		}

		// 获取该账号下的所有 RDS 实例
		rdsList, err := models.CloudRDSGetsByAccountId(m.ctx, account.Id)
		if err != nil {
			logger.Warningf("get RDS list for account %s failed: %v", account.Name, err)
			continue
		}

		for _, rds := range rdsList {
			// n9e-2kai: 只同步实例名称包含"核心"关键字的 RDS 实例
			if !strings.Contains(rds.InstanceName, "核心") {
				logger.Debugf("skip RDS %s: instance name does not contain '核心'", rds.InstanceName)
				continue
			}

			count, err := m.SyncAndAggregateSlowLogs(bgCtx, &rds, startTime, endTime, periodType)
			if err != nil {
				logger.Warningf("sync slow logs for RDS %s failed: %v", rds.InstanceName, err)
			} else {
				logger.Infof("synced %d slow log details for RDS %s", count, rds.InstanceName)
			}
		}
	}

	// 清理旧的明细数据（保留 7 天）
	if deleted, err := models.CloudRDSSlowLogDetailCleanup(m.ctx, 7); err != nil {
		logger.Warningf("cleanup old slow log details failed: %v", err)
	} else if deleted > 0 {
		logger.Infof("cleaned up %d old slow log detail records", deleted)
	}
}

// n9e-2kai: StartSlowLogSyncTask 启动慢日志定时同步任务
func (m *Manager) StartSlowLogSyncTask() {
	go func() {
		// 计算距离明天凌晨 2 点的时间
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(next.Sub(now))

		// 首次执行
		m.runDailySlowLogSync()

		// 之后每天执行一次
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			m.runDailySlowLogSync()
		}
	}()
	logger.Info("RDS slow log sync task started, will run daily at 02:00")
}

// runDailySlowLogSync 执行每日慢日志同步
func (m *Manager) runDailySlowLogSync() {
	logger.Info("starting daily RDS slow log sync task")

	// 同步昨天的数据
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	startTime := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	m.SyncAllRDSSlowLogs(ctx, startTime, endTime, "day")

	// n9e-2kai: 同步完成后执行优化判定（基于置信度+动态观察期）
	if result, err := models.DailyOptimizationJudgment(m.ctx); err != nil {
		logger.Warningf("slow SQL optimization judgment failed: %v", err)
	} else {
		logger.Infof("slow SQL optimization judgment completed: active=%d, reverted=%d, observing=%d, optimized=%d",
			result.ActiveUpdatedCount, result.RevertedFromObserving,
			result.MovedToObservingCount, result.ConfirmedOptimizedCount)
	}

	logger.Info("daily RDS slow log sync task completed")
}

// SyncAccountByConfig 基于同步配置同步单个账号的资源
func (m *Manager) SyncAccountByConfig(bgCtx context.Context, account *models.CloudAccount, config *models.CloudSyncConfig, operator string, startTimeSec, endTimeSec int64) (*models.CloudSyncLog, error) {
	// 检查是否正在同步
	m.syncMutex.Lock()
	if m.syncing[account.Id] {
		m.syncMutex.Unlock()
		return nil, fmt.Errorf("account %s is syncing", account.Name)
	}
	m.syncing[account.Id] = true
	m.syncMutex.Unlock()

	defer func() {
		m.syncMutex.Lock()
		delete(m.syncing, account.Id)
		m.syncMutex.Unlock()
	}()

	// 确定要同步的区域
	var regions []string
	if len(config.RegionsList) > 0 {
		// 使用配置中指定的区域
		regions = config.RegionsList
	} else {
		// 使用账号的区域列表
		json.Unmarshal([]byte(account.Regions), &regions)
	}

	resourceTypes := []string{config.ResourceType}

	// 创建同步日志
	syncLog := &models.CloudSyncLog{
		AccountId:     account.Id,
		AccountName:   account.Name,
		Provider:      account.Provider,
		SyncType:      models.CloudSyncTypeManual,
		ResourceTypes: config.ResourceType,
		Status:        models.CloudSyncLogStatusRunning,
		StartTime:     time.Now().Unix(),
		Operator:      operator,
	}
	if err := syncLog.Add(m.ctx); err != nil {
		config.UpdateSyncStatus(m.ctx, models.SyncStatusFailed, err.Error(), 0, 0, 0, 0)
		return nil, err
	}

	// 获取云服务提供者
	provider, err := m.GetProvider(account)
	if err != nil {
		syncLog.Status = models.CloudSyncLogStatusFailed
		syncLog.ErrorMessage = err.Error()
		syncLog.EndTime = time.Now().Unix()
		syncLog.Update(m.ctx)
		config.UpdateSyncStatus(m.ctx, models.SyncStatusFailed, err.Error(), 0, 0, 0, 0)
		return syncLog, err
	}

	var syncErrors []string
	var result *SyncResult

	// 根据资源类型同步
	switch config.ResourceType {
	case "ecs":
		result, err = m.syncECS(bgCtx, account, provider, regions)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("ECS: %v", err))
		} else {
			syncLog.EcsTotal = result.Total
			syncLog.EcsAdded = result.Added
			syncLog.EcsUpdated = result.Updated
			syncLog.EcsDeleted = result.Deleted
		}
	case "rds":
		result, err = m.syncRDS(bgCtx, account, provider, regions)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("RDS: %v", err))
		} else {
			syncLog.RdsTotal = result.Total
			syncLog.RdsAdded = result.Added
			syncLog.RdsUpdated = result.Updated
			syncLog.RdsDeleted = result.Deleted
		}
	case "rds_slowlog":
		// n9e-2kai: 同步 RDS 慢日志并生成报表
		result = &SyncResult{}
		var startTime, endTime time.Time
		if startTimeSec > 0 && endTimeSec > 0 {
			startTime = time.Unix(startTimeSec, 0)
			endTime = time.Unix(endTimeSec, 0)
		} else {
			// 默认同步昨天的数据
			now := time.Now()
			yesterday := now.AddDate(0, 0, -1)
			startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
			endTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		}

		// 获取该账号下的所有 RDS 实例
		rdsList, rdsErr := models.CloudRDSGetsByAccountId(m.ctx, account.Id)
		if rdsErr != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("RDS SlowLog: get RDS list failed: %v", rdsErr))
		} else {
			for _, rds := range rdsList {
				// n9e-2kai: 只同步实例名称包含"核心"关键字的 RDS 实例
				if !strings.Contains(rds.InstanceName, "核心") {
					logger.Debugf("skip RDS %s: instance name does not contain '核心'", rds.InstanceName)
					continue
				}
				count, slErr := m.SyncAndAggregateSlowLogs(bgCtx, &rds, startTime, endTime, "day")
				if slErr != nil {
					logger.Warningf("sync slow logs for RDS %s failed: %v", rds.InstanceName, slErr)
				} else {
					result.Total += count
					result.Added += count
				}
			}
			syncLog.RdsTotal = result.Total
			syncLog.RdsAdded = result.Added

			// n9e-2kai: 手动同步完成后也执行优化判定
			if judgeResult, judgeErr := models.DailyOptimizationJudgment(m.ctx); judgeErr != nil {
				logger.Warningf("slow SQL optimization judgment failed: %v", judgeErr)
			} else {
				logger.Infof("slow SQL optimization judgment completed: active=%d, reverted=%d, observing=%d, optimized=%d",
					judgeResult.ActiveUpdatedCount, judgeResult.RevertedFromObserving,
					judgeResult.MovedToObservingCount, judgeResult.ConfirmedOptimizedCount)
			}
		}
	}

	// 更新同步日志
	syncLog.EndTime = time.Now().Unix()
	syncLog.Duration = int(syncLog.EndTime - syncLog.StartTime)

	if len(syncErrors) > 0 {
		if len(syncErrors) == len(resourceTypes) {
			syncLog.Status = models.CloudSyncLogStatusFailed
		} else {
			syncLog.Status = models.CloudSyncLogStatusPartialSuccess
		}
		syncLog.ErrorMessage = strings.Join(syncErrors, "; ")
	} else {
		syncLog.Status = models.CloudSyncLogStatusSuccess
	}

	syncLog.Update(m.ctx)

	// 更新配置同步状态
	var syncStatus int
	if syncLog.Status == models.CloudSyncLogStatusFailed {
		syncStatus = models.SyncStatusFailed
	} else {
		syncStatus = models.SyncStatusSuccess
	}

	total := 0
	added := 0
	updated := 0
	deleted := 0
	if result != nil {
		total = result.Total
		added = result.Added
		updated = result.Updated
		deleted = result.Deleted
	}
	config.UpdateSyncStatus(m.ctx, syncStatus, syncLog.ErrorMessage, total, added, updated, deleted)

	// 更新账号同步状态
	account.LastSyncTime = syncLog.EndTime
	account.LastSyncStatus = syncLog.Status
	if syncLog.Status == models.CloudSyncLogStatusFailed {
		account.LastSyncError = syncLog.ErrorMessage
	} else {
		account.LastSyncError = ""
	}
	account.Update(m.ctx, "last_sync_time", "last_sync_status", "last_sync_error")

	return syncLog, nil
}
