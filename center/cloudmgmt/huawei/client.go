// n9e-2kai: 华为云客户端封装
package huawei

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	ecsModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	ecsRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/region"
	rds "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/rds/v3"
	rdsModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/rds/v3/model"
	rdsRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/rds/v3/region"
)

// HuaweiClient 华为云客户端
type HuaweiClient struct {
	accessKey string
	secretKey string
	regions   []string

	// 客户端缓存
	mu         sync.RWMutex
	ecsClients map[string]*ecs.EcsClient
	rdsClients map[string]*rds.RdsClient
}

// NewHuaweiClient 创建华为云客户端
func NewHuaweiClient(accessKey, secretKey string, regions []string) *HuaweiClient {
	return &HuaweiClient{
		accessKey:  accessKey,
		secretKey:  secretKey,
		regions:    regions,
		ecsClients: make(map[string]*ecs.EcsClient),
		rdsClients: make(map[string]*rds.RdsClient),
	}
}

// GetName 获取云厂商标识
func (c *HuaweiClient) GetName() string {
	return "huawei"
}

// getAuth 获取认证信息
func (c *HuaweiClient) getAuth() *basic.Credentials {
	return basic.NewCredentialsBuilder().
		WithAk(c.accessKey).
		WithSk(c.secretKey).
		Build()
}

// getECSClient 获取 ECS 客户端
func (c *HuaweiClient) getECSClient(regionCode string) (client *ecs.EcsClient, err error) {
	c.mu.RLock()
	client, ok := c.ecsClients[regionCode]
	c.mu.RUnlock()
	if ok {
		return client, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if client, ok = c.ecsClients[regionCode]; ok {
		return client, nil
	}

	// 获取区域
	region, err := ecsRegion.SafeValueOf(regionCode)
	if err != nil {
		return nil, fmt.Errorf("unsupported ECS region: %s", regionCode)
	}

	// 创建客户端 - 华为云 SDK 在网络超时时会 panic，需要捕获
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to create ECS client for region %s: %v", regionCode, r)
			client = nil
		}
	}()

	client = ecs.NewEcsClient(
		ecs.EcsClientBuilder().
			WithRegion(region).
			WithCredential(c.getAuth()).
			Build())

	c.ecsClients[regionCode] = client
	return client, nil
}

// getRDSClient 获取 RDS 客户端
func (c *HuaweiClient) getRDSClient(regionCode string) (client *rds.RdsClient, err error) {
	c.mu.RLock()
	client, ok := c.rdsClients[regionCode]
	c.mu.RUnlock()
	if ok {
		return client, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if client, ok = c.rdsClients[regionCode]; ok {
		return client, nil
	}

	// 获取区域
	region, err := rdsRegion.SafeValueOf(regionCode)
	if err != nil {
		return nil, fmt.Errorf("unsupported RDS region: %s", regionCode)
	}

	// 创建客户端 - 华为云 SDK 在网络超时时会 panic，需要捕获
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to create RDS client for region %s: %v", regionCode, r)
			client = nil
		}
	}()

	client = rds.NewRdsClient(
		rds.RdsClientBuilder().
			WithRegion(region).
			WithCredential(c.getAuth()).
			Build())

	c.rdsClients[regionCode] = client
	return client, nil
}

// TestConnection 测试连接
func (c *HuaweiClient) TestConnection(ctx context.Context) error {
	if len(c.regions) == 0 {
		return fmt.Errorf("no regions configured")
	}

	if c.accessKey == "" || c.secretKey == "" {
		return fmt.Errorf("access_key or secret_key is empty")
	}

	// 使用第一个区域测试连接
	region := c.regions[0]
	client, err := c.getECSClient(region)
	if err != nil {
		return fmt.Errorf("failed to create ECS client: %v", err)
	}

	// 尝试列出一个服务器来验证凭证
	request := &ecsModel.ListServersDetailsRequest{
		Limit: int32Ptr(1),
	}

	_, err = client.ListServersDetails(request)
	if err != nil {
		return fmt.Errorf("connection test failed: %v", err)
	}

	return nil
}

// GetRegionsInternal 获取区域列表(内部类型)
func (c *HuaweiClient) GetRegionsInternal() []Region {
	return GetAvailableRegions()
}

// ListECSInternal 获取 ECS 实例列表
func (c *HuaweiClient) ListECSInternal(ctx context.Context, region string) ([]models.CloudECS, error) {
	client, err := c.getECSClient(region)
	if err != nil {
		return nil, err
	}

	var result []models.CloudECS
	var offset int32 = 0
	limit := int32(100)

	for {
		request := &ecsModel.ListServersDetailsRequest{
			Limit:  &limit,
			Offset: &offset,
		}

		response, err := client.ListServersDetails(request)
		if err != nil {
			return nil, fmt.Errorf("failed to list ECS: %v", err)
		}

		if response.Servers == nil || len(*response.Servers) == 0 {
			break
		}

		for _, server := range *response.Servers {
			ecsInstance := c.convertToCloudECS(&server, region)
			result = append(result, ecsInstance)
		}

		if len(*response.Servers) < int(limit) {
			break
		}
		offset += limit
	}

	return result, nil
}

// convertToCloudECS 转换华为云 ECS 到通用模型
func (c *HuaweiClient) convertToCloudECS(server *ecsModel.ServerDetail, region string) models.CloudECS {
	ecsInstance := models.CloudECS{
		Provider:     "huawei",
		Region:       region,
		InstanceId:   server.Id,
		InstanceName: server.Name,
		Status:       strings.ToLower(server.Status),
		SyncTime:     time.Now().Unix(),
	}

	// 规格信息
	if server.Flavor != nil {
		ecsInstance.InstanceType = server.Flavor.Id
		// Vcpus 是 string，需要转换
		if vcpus, err := strconv.Atoi(server.Flavor.Vcpus); err == nil {
			ecsInstance.CPU = vcpus
		}
		// Ram 也是 string，需要转换（MB 转 GB）
		if ram, err := strconv.Atoi(server.Flavor.Ram); err == nil {
			ecsInstance.Memory = ram / 1024
		}
	}

	// 提取 IP 地址 - 模型使用单个 IP 字段
	if server.Addresses != nil {
		for _, addrs := range server.Addresses {
			for _, addr := range addrs {
				if addr.OSEXTIPStype != nil {
					if addr.OSEXTIPStype.Value() == "fixed" {
						if ecsInstance.PrivateIp == "" {
							ecsInstance.PrivateIp = addr.Addr
						}
					} else if addr.OSEXTIPStype.Value() == "floating" {
						if ecsInstance.PublicIp == "" {
							ecsInstance.PublicIp = addr.Addr
						}
					}
				}
			}
		}
	}

	// 操作系统信息
	if server.Metadata != nil {
		if osType, ok := server.Metadata["os_type"]; ok {
			ecsInstance.OsType = osType
		}
		if imageName, ok := server.Metadata["image_name"]; ok {
			ecsInstance.OsName = imageName
		}
	}

	// 计费方式
	if server.Metadata != nil {
		if chargingMode, ok := server.Metadata["charging_mode"]; ok {
			if chargingMode == "0" {
				ecsInstance.ChargeType = "postpaid"
			} else {
				ecsInstance.ChargeType = "prepaid"
			}
		}
	}

	// 创建时间
	if server.Created != "" {
		if t, err := time.Parse(time.RFC3339, server.Created); err == nil {
			ecsInstance.CreateTime = t.Unix()
		}
	}

	// 标签 - 模型使用 TagsMap
	if server.Tags != nil && len(*server.Tags) > 0 {
		ecsInstance.TagsMap = make(map[string]string)
		for _, tag := range *server.Tags {
			// 华为云标签格式为 "key=value" 或 "key.value"
			parts := strings.SplitN(tag, "=", 2)
			if len(parts) == 2 {
				ecsInstance.TagsMap[parts[0]] = parts[1]
			} else {
				parts = strings.SplitN(tag, ".", 2)
				if len(parts) == 2 {
					ecsInstance.TagsMap[parts[0]] = parts[1]
				}
			}
		}
		// 序列化标签
		if tagsData, err := json.Marshal(ecsInstance.TagsMap); err == nil {
			ecsInstance.Tags = string(tagsData)
		}
	}

	return ecsInstance
}

// ListRDSInternal 获取 RDS 实例列表
func (c *HuaweiClient) ListRDSInternal(ctx context.Context, region string) ([]models.CloudRDS, error) {
	client, err := c.getRDSClient(region)
	if err != nil {
		return nil, err
	}

	var result []models.CloudRDS
	var offset int32 = 0
	limit := int32(100)

	for {
		request := &rdsModel.ListInstancesRequest{
			Limit:  &limit,
			Offset: &offset,
		}

		response, err := client.ListInstances(request)
		if err != nil {
			return nil, fmt.Errorf("failed to list RDS: %v", err)
		}

		if response.Instances == nil || len(*response.Instances) == 0 {
			break
		}

		for _, instance := range *response.Instances {
			rdsInstance := c.convertToCloudRDS(&instance, region)
			result = append(result, rdsInstance)
		}

		if len(*response.Instances) < int(limit) {
			break
		}
		offset += limit
	}

	return result, nil
}

// convertToCloudRDS 转换华为云 RDS 到通用模型
func (c *HuaweiClient) convertToCloudRDS(instance *rdsModel.InstanceResponse, region string) models.CloudRDS {
	rdsInstance := models.CloudRDS{
		Provider:     "huawei",
		Region:       region,
		InstanceId:   instance.Id,
		InstanceName: instance.Name,
		Status:       instance.Status, // 保持原始状态值（如 ACTIVE, BUILD, FAILED）
		SyncTime:     time.Now().Unix(),
	}

	// 数据库引擎 - DatastoreType 是结构体，需要用 Value() 方法
	if instance.Datastore != nil {
		rdsInstance.Engine = strings.ToLower(instance.Datastore.Type.Value())
		rdsInstance.EngineVersion = instance.Datastore.Version
	}

	// 规格信息
	if instance.FlavorRef != "" {
		rdsInstance.InstanceType = instance.FlavorRef
	}
	// CPU 是 *string，需要解引用后转换
	if instance.Cpu != nil {
		if cpu, err := strconv.Atoi(*instance.Cpu); err == nil {
			rdsInstance.CPU = cpu
		}
	}
	// Mem 是 *string，单位已经是 GB
	if instance.Mem != nil {
		if mem, err := strconv.Atoi(*instance.Mem); err == nil {
			rdsInstance.Memory = mem // 单位已经是 GB，不需要转换
		}
	}

	// 存储 - VolumeType 是结构体
	if instance.Volume != nil {
		rdsInstance.Storage = int(instance.Volume.Size)
		rdsInstance.StorageType = instance.Volume.Type.Value()
	}

	// 端口
	rdsInstance.Port = int(instance.Port)

	// IP 地址 - PrivateIps/PublicIps 是 []string
	if len(instance.PrivateIps) > 0 {
		rdsInstance.PrivateIp = instance.PrivateIps[0]
	}
	if len(instance.PublicIps) > 0 {
		rdsInstance.PublicIp = instance.PublicIps[0]
	}

	// VPC 和子网
	if instance.VpcId != "" {
		rdsInstance.VpcId = instance.VpcId
	}
	if instance.SubnetId != "" {
		rdsInstance.SubnetId = instance.SubnetId
	}

	// 计费方式
	if instance.ChargeInfo != nil {
		chargeMode := instance.ChargeInfo.ChargeMode.Value()
		if chargeMode == "postPaid" || chargeMode == "0" {
			rdsInstance.ChargeType = "postpaid" // 按需付费
		} else if chargeMode == "prePaid" || chargeMode == "1" {
			rdsInstance.ChargeType = "prepaid" // 包年包月
		} else {
			rdsInstance.ChargeType = chargeMode
		}
	}

	// 到期时间
	if instance.ExpirationTime != nil && *instance.ExpirationTime != "" {
		if t, err := time.Parse(time.RFC3339, *instance.ExpirationTime); err == nil {
			rdsInstance.ExpireTime = t.Unix()
		}
	}

	// 创建时间
	if instance.Created != "" {
		if t, err := time.Parse(time.RFC3339, instance.Created); err == nil {
			rdsInstance.CreateTime = t.Unix()
		}
	}

	// 高可用模式
	if instance.Type != "" {
		switch instance.Type {
		case "Single":
			rdsInstance.HaMode = "single"
		case "Ha":
			rdsInstance.HaMode = "ha"
		case "Replica":
			rdsInstance.HaMode = "readonly"
		default:
			rdsInstance.HaMode = strings.ToLower(instance.Type)
		}
	}

	// 标签 - 模型使用 TagsMap
	if instance.Tags != nil && len(instance.Tags) > 0 {
		rdsInstance.TagsMap = make(map[string]string)
		for _, tag := range instance.Tags {
			rdsInstance.TagsMap[tag.Key] = tag.Value
		}
		// 序列化标签
		if tagsData, err := json.Marshal(rdsInstance.TagsMap); err == nil {
			rdsInstance.Tags = string(tagsData)
		}
	}

	return rdsInstance
}

// 辅助函数
func int32Ptr(v int32) *int32 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

// detectSqlType 从 SQL 语句中自动检测 SQL 类型
func detectSqlType(sql string) string {
	if sql == "" {
		return "UNKNOWN"
	}
	// 清理并转换为大写
	sql = strings.TrimSpace(strings.ToUpper(sql))

	// 检测常见的 SQL 类型
	switch {
	case strings.HasPrefix(sql, "SELECT"):
		return "SELECT"
	case strings.HasPrefix(sql, "INSERT"):
		return "INSERT"
	case strings.HasPrefix(sql, "UPDATE"):
		return "UPDATE"
	case strings.HasPrefix(sql, "DELETE"):
		return "DELETE"
	case strings.HasPrefix(sql, "CREATE"):
		return "CREATE"
	case strings.HasPrefix(sql, "ALTER"):
		return "ALTER"
	case strings.HasPrefix(sql, "DROP"):
		return "DROP"
	case strings.HasPrefix(sql, "TRUNCATE"):
		return "TRUNCATE"
	case strings.HasPrefix(sql, "COMMIT"):
		return "COMMIT"
	case strings.HasPrefix(sql, "ROLLBACK"):
		return "ROLLBACK"
	case strings.HasPrefix(sql, "SET"):
		return "SET"
	case strings.HasPrefix(sql, "SHOW"):
		return "SHOW"
	case strings.HasPrefix(sql, "CALL"):
		return "CALL"
	case strings.HasPrefix(sql, "EXPLAIN"):
		return "EXPLAIN"
	default:
		return "OTHER"
	}
}

// SlowLogStatistics 慢日志统计结果
type SlowLogStatistics struct {
	SqlText         string  // 执行语句
	SqlType         string  // 语句类型: SELECT, INSERT, UPDATE, DELETE
	Database        string  // 所属数据库
	ExecuteCount    int64   // 执行次数
	ExecuteRatio    float64 // 执行占比 (%)
	AvgTime         float64 // 平均执行时间(s)
	AvgLockTime     float64 // 平均等待锁的时间(s)
	AvgRowsExamined int64   // 平均扫描的行数量
	AvgRowsSent     int64   // 平均结果行统计数量
	Users           string  // 执行用户
	ClientIP        string  // 客户端IP
}

// ListSlowLogStatistics 获取 RDS 慢日志统计
func (c *HuaweiClient) ListSlowLogStatistics(ctx context.Context, region string, instanceId string, startTime, endTime string, database string, sqlType string, limit, offset int32) ([]SlowLogStatistics, int32, error) {
	client, err := c.getRDSClient(region)
	if err != nil {
		return nil, 0, err
	}

	// 构建请求体
	body := &rdsModel.SlowLogStatisticsForLtsRequest{
		StartTime: startTime,
		EndTime:   endTime,
	}

	if limit > 0 {
		body.Limit = &limit
	}
	if offset > 0 {
		body.Offset = &offset
	}
	if database != "" && database != "all" {
		body.Database = &database
	}
	// 处理 SQL 类型 - 使用枚举类型
	if sqlType != "" && sqlType != "all" && sqlType != "ALL" {
		typeEnum := rdsModel.GetSlowLogStatisticsForLtsRequestTypeEnum()
		switch strings.ToUpper(sqlType) {
		case "SELECT":
			body.Type = &typeEnum.SELECT
		case "INSERT":
			body.Type = &typeEnum.INSERT
		case "UPDATE":
			body.Type = &typeEnum.UPDATE
		case "DELETE":
			body.Type = &typeEnum.DELETE
		case "CREATE":
			body.Type = &typeEnum.CREATE
		}
	}

	// 按执行次数降序排列 - 使用枚举类型
	sort := "executeTimes"
	body.Sort = &sort
	orderEnum := rdsModel.GetSlowLogStatisticsForLtsRequestOrderEnum()
	body.Order = &orderEnum.DESC

	request := &rdsModel.ListSlowLogStatisticsForLtsRequest{
		InstanceId: instanceId,
		Body:       body,
	}

	response, err := client.ListSlowLogStatisticsForLts(request)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list slow log statistics: %v", err)
	}

	var result []SlowLogStatistics
	var totalCount int32 = 0

	if response.SlowLogList != nil {
		totalCount = int32(len(*response.SlowLogList))
		for _, log := range *response.SlowLogList {
			stat := SlowLogStatistics{}

			if log.QuerySample != nil {
				stat.SqlText = *log.QuerySample
			}
			if log.Type != nil {
				stat.SqlType = *log.Type
			}
			if log.Database != nil {
				stat.Database = *log.Database
			}
			if log.Count != nil {
				// Count 是 string，需要转换
				if count, err := strconv.ParseInt(*log.Count, 10, 64); err == nil {
					stat.ExecuteCount = count
				}
			}
			if log.Time != nil {
				// Time 是 string，需要转换
				if t, err := strconv.ParseFloat(*log.Time, 64); err == nil {
					stat.AvgTime = t
				}
			}
			if log.LockTime != nil {
				if t, err := strconv.ParseFloat(*log.LockTime, 64); err == nil {
					stat.AvgLockTime = t
				}
			}
			// RowsExamined 和 RowsSent 是 *int64
			if log.RowsExamined != nil {
				stat.AvgRowsExamined = *log.RowsExamined
			}
			if log.RowsSent != nil {
				stat.AvgRowsSent = *log.RowsSent
			}
			if log.Users != nil {
				stat.Users = *log.Users
			}
			if log.ClientIp != nil {
				stat.ClientIP = *log.ClientIp
			}

			result = append(result, stat)
		}
	}

	return result, totalCount, nil
}

// SlowLogDetail 慢日志明细记录
type SlowLogDetail struct {
	SqlText      string  // 执行语句
	SqlType      string  // 语句类型: SELECT, INSERT, UPDATE, DELETE
	Database     string  // 所属数据库
	ExecuteTime  float64 // 执行时间(s)
	LockTime     float64 // 等待锁时间(s)
	RowsSent     int64   // 结果行数
	RowsExamined int64   // 扫描行数
	StartTime    string  // 发生时间
	Users        string  // 执行用户
	ClientIP     string  // 客户端IP
	LineNum      string  // 日志行号(用于分页)
}

// ListSlowLogDetails 获取 RDS 慢日志明细
func (c *HuaweiClient) ListSlowLogDetails(ctx context.Context, region string, instanceId string, startTime, endTime string, database string, sqlType string, limit int32, lineNum string) ([]SlowLogDetail, string, error) {
	client, err := c.getRDSClient(region)
	if err != nil {
		return nil, "", err
	}

	// 构建请求体
	body := &rdsModel.SlowlogForLtsRequest{
		StartTime: startTime,
		EndTime:   endTime,
	}

	if limit > 0 {
		body.Limit = &limit
	}
	if lineNum != "" {
		body.LineNum = &lineNum
	}
	if database != "" && database != "all" {
		body.Database = &database
	}
	// 处理 SQL 类型 - 使用枚举类型
	if sqlType != "" && sqlType != "all" && sqlType != "ALL" {
		typeEnum := rdsModel.GetSlowlogForLtsRequestTypeEnum()
		switch strings.ToUpper(sqlType) {
		case "SELECT":
			body.Type = &typeEnum.SELECT
		case "INSERT":
			body.Type = &typeEnum.INSERT
		case "UPDATE":
			body.Type = &typeEnum.UPDATE
		case "DELETE":
			body.Type = &typeEnum.DELETE
		case "CREATE":
			body.Type = &typeEnum.CREATE
		}
	}

	request := &rdsModel.ListSlowlogForLtsRequest{
		InstanceId: instanceId,
		Body:       body,
	}

	response, err := client.ListSlowlogForLts(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list slow log details: %v", err)
	}

	var result []SlowLogDetail
	var lastLineNum string

	if response.SlowLogList != nil {
		for _, log := range *response.SlowLogList {
			detail := SlowLogDetail{}

			if log.QuerySample != nil {
				detail.SqlText = *log.QuerySample
			}
			if log.Type != nil && *log.Type != "" {
				detail.SqlType = *log.Type
			} else {
				// 当 sql_type 为空时，从 SQL 语句中自动检测类型
				detail.SqlType = detectSqlType(detail.SqlText)
			}
			if log.Database != nil {
				detail.Database = *log.Database
			}
			if log.Time != nil {
				timeStr := strings.TrimSpace(*log.Time)
				// 华为云返回格式如 "16.83885 s"（有空格+单位）
				// 移除可能的单位后缀（如 " s", "s", " ms", "ms"）
				timeStr = strings.TrimSuffix(timeStr, " s")  // 先移除 " s"
				timeStr = strings.TrimSuffix(timeStr, "s")   // 再移除单独的 "s"
				timeStr = strings.TrimSuffix(timeStr, " ms") // 移除 " ms"
				timeStr = strings.TrimSuffix(timeStr, "ms")  // 移除单独的 "ms"
				timeStr = strings.TrimSpace(timeStr)
				if t, err := strconv.ParseFloat(timeStr, 64); err == nil {
					detail.ExecuteTime = t
				}
			}
			if log.LockTime != nil {
				if t, err := strconv.ParseFloat(*log.LockTime, 64); err == nil {
					detail.LockTime = t
				}
			}
			if log.RowsSent != nil {
				if r, err := strconv.ParseInt(*log.RowsSent, 10, 64); err == nil {
					detail.RowsSent = r
				}
			}
			if log.RowsExamined != nil {
				if r, err := strconv.ParseInt(*log.RowsExamined, 10, 64); err == nil {
					detail.RowsExamined = r
				}
			}
			if log.StartTime != nil {
				detail.StartTime = *log.StartTime
			}
			if log.Users != nil {
				detail.Users = *log.Users
			}
			if log.ClientIp != nil {
				detail.ClientIP = *log.ClientIp
			}
			if log.LineNum != nil {
				detail.LineNum = *log.LineNum
				lastLineNum = *log.LineNum
			}

			result = append(result, detail)
		}
	}

	return result, lastLineNum, nil
}
