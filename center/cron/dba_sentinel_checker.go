package cron

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
	"gopkg.in/gomail.v2"
)

// DBASentinelChecker DBA 哨兵检查器
type DBASentinelChecker struct {
	ctx      *ctx.Context
	interval time.Duration
	quit     chan struct{}
}

// NewDBASentinelChecker 创建哨兵检查器
func NewDBASentinelChecker(ctx *ctx.Context, interval time.Duration) *DBASentinelChecker {
	if interval == 0 {
		interval = 10 * time.Second // 默认 10 秒检查一次
	}

	return &DBASentinelChecker{
		ctx:      ctx,
		interval: interval,
		quit:     make(chan struct{}),
	}
}

// Start 启动哨兵检查
func (c *DBASentinelChecker) Start() {
	logger.Info("DBA sentinel checker started")

	ticker := time.NewTicker(c.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.checkAllRules()
			case <-c.quit:
				ticker.Stop()
				logger.Info("DBA sentinel checker stopped")
				return
			}
		}
	}()
}

// Stop 停止哨兵检查
func (c *DBASentinelChecker) Stop() {
	close(c.quit)
}

// checkAllRules 检查所有规则
func (c *DBASentinelChecker) checkAllRules() {
	rules, err := models.DBASentinelRuleGetsEnabled(c.ctx)
	if err != nil {
		logger.Errorf("failed to get sentinel rules: %v", err)
		return
	}

	for _, rule := range rules {
		// 检查是否应该执行
		if !rule.ShouldCheck() {
			continue
		}

		// 执行检查
		go c.checkRule(&rule)
	}
}

// checkRule 检查单个规则
func (c *DBASentinelChecker) checkRule(rule *models.DBASentinelRule) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic in checkRule: %v", r)
		}
	}()

	// 更新上次检查时间
	rule.UpdateLastCheckAt(c.ctx)

	// 获取 Archery 客户端
	config, err := models.GetArcheryClientFromDB(c.ctx, "")
	if err != nil {
		logger.Warningf("failed to get archery config: %v", err)
		return
	}
	if config == nil {
		return
	}

	client, err := dbm.NewArcheryClient(config)
	if err != nil {
		logger.Warningf("failed to create archery client: %v", err)
		return
	}

	// 根据规则类型执行检查
	switch rule.RuleType {
	case models.SentinelRuleTypeSlowQuery:
		c.checkSlowQuery(client, rule)
	case models.SentinelRuleTypeUncommittedTrx:
		c.checkUncommittedTrx(client, rule)
	case models.SentinelRuleTypeLockWait:
		c.checkLockWait(client, rule)
	}
}

// checkSlowQuery 检查慢查询 (processlist)
func (c *DBASentinelChecker) checkSlowQuery(client *dbm.ArcheryClient, rule *models.DBASentinelRule) {
	sessions, err := client.GetSessions(dbm.ArcherySessionListRequest{
		InstanceID: int(rule.InstanceId),
	})
	if err != nil {
		logger.Warningf("failed to get sessions for instance %d: %v", rule.InstanceId, err)
		return
	}

	for _, session := range sessions {
		matched, reason := rule.MatchSession(
			session.User,
			session.DB,
			session.Command,
			session.State,
			session.Info,
			session.Time,
		)

		if !matched {
			continue
		}

		// 记录并执行动作
		c.executeAction(client, rule, &session, reason)
	}
}

// checkUncommittedTrx 检查未提交事务
func (c *DBASentinelChecker) checkUncommittedTrx(client *dbm.ArcheryClient, rule *models.DBASentinelRule) {
	trxList, err := client.GetUncommittedTransactions(dbm.ArcheryUncommittedTrxRequest{
		InstanceID: int(rule.InstanceId),
	})
	if err != nil {
		logger.Warningf("failed to get uncommitted transactions for instance %d: %v", rule.InstanceId, err)
		return
	}

	for _, trx := range trxList {
		matched, reason := rule.MatchSession(
			trx.User,
			trx.DB,
			"",
			"",
			trx.SQLText,
			trx.RuntimeSec,
		)

		if !matched {
			continue
		}

		// 构造 session 对象
		session := &dbm.ArcherySession{
			ID:      trx.ProcesslistID,
			User:    trx.User,
			DB:      trx.DB,
			Command: "Query",
			Time:    trx.RuntimeSec,
			State:   "uncommitted",
			Info:    trx.SQLText,
			TrxID:   trx.TrxID,
		}

		// 记录并执行动作
		c.executeAction(client, rule, session, reason)
	}
}

// checkLockWait 检查锁等待
func (c *DBASentinelChecker) checkLockWait(client *dbm.ArcheryClient, rule *models.DBASentinelRule) {
	locks, err := client.GetLockWaits(dbm.ArcheryLockWaitRequest{
		InstanceID: int(rule.InstanceId),
	})
	if err != nil {
		logger.Warningf("failed to get lock waits for instance %d: %v", rule.InstanceId, err)
		return
	}

	for _, lock := range locks {
		// 检查等待时间是否超过阈值
		if lock.WaitingTime < rule.MaxTime {
			continue
		}

		matched, reason := rule.MatchSession(
			lock.WaitingUser,
			lock.WaitingDB,
			"",
			"",
			lock.WaitingQuery,
			lock.WaitingTime,
		)

		if !matched {
			continue
		}

		// 构造 session 对象 (等待锁的进程)
		session := &dbm.ArcherySession{
			ID:      lock.WaitingThreadID,
			User:    lock.WaitingUser,
			Host:    lock.WaitingHost,
			DB:      lock.WaitingDB,
			Command: "Query",
			Time:    lock.WaitingTime,
			State:   "lock_wait",
			Info:    lock.WaitingQuery,
		}

		reason = fmt.Sprintf("规则[%s]: 锁等待时间 %ds 超过阈值 %ds, 被阻塞者: %s@%s",
			rule.Name, lock.WaitingTime, rule.MaxTime, lock.BlockingUser, lock.BlockingHost)

		// 记录并执行动作
		c.executeAction(client, rule, session, reason)
	}
}

// executeAction 执行动作 (kill 或仅通知)
func (c *DBASentinelChecker) executeAction(client *dbm.ArcheryClient, rule *models.DBASentinelRule, session *dbm.ArcherySession, reason string) {
	// 创建日志记录
	log := &models.DBASentinelKillLog{
		RuleId:       rule.Id,
		RuleName:     rule.Name,
		InstanceId:   rule.InstanceId,
		ThreadId:     session.ID,
		User:         session.User,
		Host:         session.Host,
		Db:           session.DB,
		Command:      session.Command,
		Time:         session.Time,
		State:        session.State,
		SqlText:      session.Info,
		TrxId:        session.TrxID,
		KillReason:   reason,
		NotifyStatus: models.NotifyStatusSkipped,
	}

	// 执行 Kill 动作
	if rule.Action == models.SentinelActionKill {
		err := client.KillSessions(dbm.ArcheryKillSessionsRequest{
			InstanceID: int(rule.InstanceId),
			ThreadIDs:  []int64{session.ID},
		})

		if err != nil {
			log.KillResult = models.KillResultFailed
			log.ErrorMsg = err.Error()
			logger.Warningf("failed to kill session %d on instance %d: %v", session.ID, rule.InstanceId, err)
		} else {
			log.KillResult = models.KillResultSuccess
			logger.Infof("killed session %d on instance %d: %s", session.ID, rule.InstanceId, reason)
		}
	} else {
		log.KillResult = "skipped"
	}

	// 保存日志
	if err := log.Add(c.ctx); err != nil {
		logger.Warningf("failed to save kill log: %v", err)
	}

	// 发送通知
	c.sendNotification(rule, log)
}

// sendNotification 发送通知
func (c *DBASentinelChecker) sendNotification(rule *models.DBASentinelRule, log *models.DBASentinelKillLog) {
	// 获取通知渠道 IDs
	channelIds := rule.GetNotifyChannelIdList()
	if len(channelIds) == 0 {
		return
	}

	// 构造通知内容
	content := fmt.Sprintf(`【DBA 哨兵告警】
规则名称: %s
实例 ID: %d
线程 ID: %d
用户: %s
数据库: %s
执行时长: %ds
SQL: %.200s
Kill 原因: %s
Kill 结果: %s
操作时间: %s`,
		log.RuleName,
		log.InstanceId,
		log.ThreadId,
		log.User,
		log.Db,
		log.Time,
		log.SqlText,
		log.KillReason,
		log.KillResult,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// 遍历通知渠道发送
	for _, channelId := range channelIds {
		var channel models.NotifyChannelConfig
		err := models.DB(c.ctx).Where("id = ?", channelId).First(&channel).Error
		if err != nil {
			logger.Warningf("failed to get notify channel %d: %v", channelId, err)
			continue
		}

		// 发送通知
		err = c.sendToChannel(&channel, content, rule.GetNotifyUserGroupIdList())
		if err != nil {
			log.NotifyStatus = models.NotifyStatusFailed
			log.NotifyMessage = err.Error()
			logger.Warningf("failed to send notification via channel %s: %v", channel.Name, err)
		} else {
			log.NotifyStatus = models.NotifyStatusSent
			log.NotifyMessage = fmt.Sprintf("sent via %s", channel.Name)
		}
	}

	// 更新日志的通知状态
	if log.Id > 0 {
		models.DB(c.ctx).Model(log).Updates(map[string]interface{}{
			"notify_status":  log.NotifyStatus,
			"notify_message": log.NotifyMessage,
		})
	}
}

// sendToChannel 通过渠道发送通知
func (c *DBASentinelChecker) sendToChannel(channel *models.NotifyChannelConfig, content string, userGroupIds []int64) error {
	// 获取用户组成员的邮箱
	sendtos := []string{}
	for _, ugid := range userGroupIds {
		// 获取用户组成员的用户 ID
		userIds, err := models.MemberIds(c.ctx, ugid)
		if err != nil {
			continue
		}
		// 获取用户信息
		for _, userId := range userIds {
			var user models.User
			err := models.DB(c.ctx).Where("id = ?", userId).First(&user).Error
			if err != nil {
				continue
			}
			if user.Email != "" {
				sendtos = append(sendtos, user.Email)
			}
		}
	}

	// 根据渠道类型发送通知
	switch channel.RequestType {
	case "http":
		return c.sendHTTPNotification(channel, content)
	case "smtp":
		return c.sendSMTPNotification(channel, content, sendtos)
	default:
		logger.Infof("DBA sentinel: sending notification via channel %s (type: %s) to %v", channel.Name, channel.RequestType, sendtos)
		return nil
	}
}

// sendHTTPNotification 通过 HTTP 发送通知
func (c *DBASentinelChecker) sendHTTPNotification(channel *models.NotifyChannelConfig, content string) error {
	if channel.RequestConfig == nil || channel.RequestConfig.HTTPRequestConfig == nil {
		return fmt.Errorf("http request config not found for channel %s", channel.Name)
	}

	httpConfig := channel.RequestConfig.HTTPRequestConfig
	if httpConfig.URL == "" {
		return fmt.Errorf("url is empty for channel %s", channel.Name)
	}

	// 构造请求体
	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	// 创建 HTTP 客户端
	timeout := httpConfig.Timeout
	if timeout == 0 {
		timeout = 10000 // 默认 10 秒
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: httpConfig.TLS != nil && httpConfig.TLS.SkipVerify,
			},
		},
	}

	// 创建请求
	method := httpConfig.Method
	if method == "" {
		method = "POST"
	}
	req, err := http.NewRequest(method, httpConfig.URL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for k, v := range httpConfig.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	logger.Infof("DBA sentinel: HTTP notification sent successfully to %s, response: %s", httpConfig.URL, string(respBody))
	return nil
}

// sendSMTPNotification 通过 SMTP 发送邮件通知
func (c *DBASentinelChecker) sendSMTPNotification(channel *models.NotifyChannelConfig, content string, sendtos []string) error {
	if len(sendtos) == 0 {
		return fmt.Errorf("no recipients for email notification")
	}

	if channel.RequestConfig == nil || channel.RequestConfig.SMTPRequestConfig == nil {
		return fmt.Errorf("smtp request config not found for channel %s", channel.Name)
	}

	smtpConfig := channel.RequestConfig.SMTPRequestConfig
	if smtpConfig.Host == "" {
		return fmt.Errorf("smtp host is empty for channel %s", channel.Name)
	}

	// 使用 gomail 发送邮件
	m := gomail.NewMessage()
	m.SetHeader("From", smtpConfig.From)
	m.SetHeader("To", sendtos...)
	m.SetHeader("Subject", "【DBA 哨兵告警】")
	m.SetBody("text/plain", content)

	d := gomail.NewDialer(smtpConfig.Host, smtpConfig.Port, smtpConfig.Username, smtpConfig.Password)
	if smtpConfig.InsecureSkipVerify {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	logger.Infof("DBA sentinel: Email notification sent successfully to %v", sendtos)
	return nil
}

// ScheduleDBASentinelChecker 启动 DBA 哨兵检查定时任务
func ScheduleDBASentinelChecker(ctx *ctx.Context, interval time.Duration) *DBASentinelChecker {
	checker := NewDBASentinelChecker(ctx, interval)
	checker.Start()
	return checker
}
