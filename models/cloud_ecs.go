// n9e-2kai: 云主机资源模型
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudECS 云主机资源
type CloudECS struct {
	Id        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountId int64  `json:"account_id" gorm:"not null;index"`
	Provider  string `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:idx_provider_instance"`
	Region    string `json:"region" gorm:"type:varchar(64);not null;index"`

	// 资源唯一标识
	InstanceId   string `json:"instance_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_provider_instance"`
	InstanceName string `json:"instance_name" gorm:"type:varchar(256)"`

	// 基础信息
	Status       string `json:"status" gorm:"type:varchar(32);default:'unknown';index"` // running, stopped, etc.
	InstanceType string `json:"instance_type" gorm:"type:varchar(64)"`
	ImageId      string `json:"image_id" gorm:"type:varchar(128)"`
	ImageName    string `json:"image_name" gorm:"type:varchar(256)"`

	// 配置信息
	CPU    int    `json:"cpu" gorm:"default:0"`
	Memory int    `json:"memory" gorm:"default:0"` // MB
	OsType string `json:"os_type" gorm:"type:varchar(32)"`
	OsName string `json:"os_name" gorm:"type:varchar(256)"`

	// 网络信息
	VpcId             string   `json:"vpc_id" gorm:"type:varchar(128)"`
	SubnetId          string   `json:"subnet_id" gorm:"type:varchar(128)"`
	PrivateIp         string   `json:"private_ip" gorm:"type:varchar(64);index"`
	PublicIp          string   `json:"public_ip" gorm:"type:varchar(64);index"`
	SecurityGroups    string   `json:"-" gorm:"type:text"`
	SecurityGroupList []string `json:"security_groups" gorm:"-"`
	AvailabilityZone  string   `json:"availability_zone" gorm:"type:varchar(128)"`

	// 计费信息
	ChargeType string `json:"charge_type" gorm:"type:varchar(32)"`
	ExpireTime int64  `json:"expire_time" gorm:"default:0"`

	// 标签
	Tags    string            `json:"-" gorm:"type:text"`
	TagsMap map[string]string `json:"tags" gorm:"-"`

	// 元数据
	RawData    string `json:"-" gorm:"type:text"`
	SyncTime   int64  `json:"sync_time" gorm:"not null;index"`
	CreateTime int64  `json:"create_time" gorm:"default:0"` // 实例创建时间（云端）
}

func (CloudECS) TableName() string {
	return "cloud_ecs"
}

// ECS 状态常量
const (
	ECSStatusRunning   = "running"
	ECSStatusStopped   = "stopped"
	ECSStatusStarting  = "starting"
	ECSStatusStopping  = "stopping"
	ECSStatusRebooting = "rebooting"
	ECSStatusCreating  = "creating"
	ECSStatusDeleting  = "deleting"
	ECSStatusError     = "error"
	ECSStatusUnknown   = "unknown"
)

// ParseTags 解析标签
func (e *CloudECS) ParseTags() error {
	if e.Tags == "" {
		e.TagsMap = make(map[string]string)
		return nil
	}
	return json.Unmarshal([]byte(e.Tags), &e.TagsMap)
}

// SerializeTags 序列化标签
func (e *CloudECS) SerializeTags() error {
	if len(e.TagsMap) == 0 {
		e.Tags = "{}"
		return nil
	}
	data, err := json.Marshal(e.TagsMap)
	if err != nil {
		return err
	}
	e.Tags = string(data)
	return nil
}

// ParseSecurityGroups 解析安全组列表
func (e *CloudECS) ParseSecurityGroups() error {
	if e.SecurityGroups == "" {
		e.SecurityGroupList = []string{}
		return nil
	}
	return json.Unmarshal([]byte(e.SecurityGroups), &e.SecurityGroupList)
}

// Add 添加 ECS 资源
func (e *CloudECS) Add(c *ctx.Context) error {
	e.SyncTime = time.Now().Unix()
	return Insert(c, e)
}

// Update 更新 ECS 资源
func (e *CloudECS) Update(c *ctx.Context) error {
	e.SyncTime = time.Now().Unix()
	return DB(c).Model(e).Updates(e).Error
}

// CloudECSGet 根据 ID 获取 ECS
func CloudECSGet(c *ctx.Context, id int64) (*CloudECS, error) {
	var ecs CloudECS
	err := DB(c).Where("id = ?", id).First(&ecs).Error
	if err != nil {
		return nil, err
	}
	ecs.ParseTags()
	ecs.ParseSecurityGroups()
	return &ecs, nil
}

// CloudECSGetByInstanceId 根据云厂商和实例ID获取 ECS
func CloudECSGetByInstanceId(c *ctx.Context, provider, instanceId string) (*CloudECS, error) {
	var ecs CloudECS
	err := DB(c).Where("provider = ? AND instance_id = ?", provider, instanceId).First(&ecs).Error
	if err != nil {
		return nil, err
	}
	ecs.ParseTags()
	ecs.ParseSecurityGroups()
	return &ecs, nil
}

// CloudECSGets 获取 ECS 列表
func CloudECSGets(c *ctx.Context, accountId int64, provider, region, status, query string, limit, offset int) ([]CloudECS, error) {
	var ecsList []CloudECS
	session := DB(c).Order("id desc")

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if region != "" && region != "all" {
		session = session.Where("region = ?", region)
	}

	if status != "" && status != "all" {
		session = session.Where("status = ?", status)
	}

	if query != "" {
		session = session.Where("instance_name like ? or instance_id like ? or private_ip like ? or public_ip like ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&ecsList).Error
	if err != nil {
		return nil, err
	}

	for i := range ecsList {
		ecsList[i].ParseTags()
		ecsList[i].ParseSecurityGroups()
	}

	return ecsList, nil
}

// CloudECSCount 获取 ECS 总数
func CloudECSCount(c *ctx.Context, accountId int64, provider, region, status, query string) (int64, error) {
	session := DB(c).Model(&CloudECS{})

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if region != "" && region != "all" {
		session = session.Where("region = ?", region)
	}

	if status != "" && status != "all" {
		session = session.Where("status = ?", status)
	}

	if query != "" {
		session = session.Where("instance_name like ? or instance_id like ? or private_ip like ? or public_ip like ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// CloudECSStats 获取 ECS 统计信息
func CloudECSStats(c *ctx.Context, accountId int64, provider string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	session := DB(c).Model(&CloudECS{})
	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}
	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	// 总数
	var total int64
	session.Count(&total)
	stats["total"] = total

	// 按状态统计
	type StatusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []StatusCount
	DB(c).Model(&CloudECS{}).Select("status, count(*) as count").Group("status").Scan(&statusCounts)
	byStatus := make(map[string]int64)
	for _, sc := range statusCounts {
		byStatus[sc.Status] = sc.Count
	}
	stats["by_status"] = byStatus

	// 按云厂商统计
	type ProviderCount struct {
		Provider string
		Count    int64
	}
	var providerCounts []ProviderCount
	DB(c).Model(&CloudECS{}).Select("provider, count(*) as count").Group("provider").Scan(&providerCounts)
	byProvider := make(map[string]int64)
	for _, pc := range providerCounts {
		byProvider[pc.Provider] = pc.Count
	}
	stats["by_provider"] = byProvider

	// 按区域统计
	type RegionCount struct {
		Region string
		Count  int64
	}
	var regionCounts []RegionCount
	DB(c).Model(&CloudECS{}).Select("region, count(*) as count").Group("region").Scan(&regionCounts)
	byRegion := make(map[string]int64)
	for _, rc := range regionCounts {
		byRegion[rc.Region] = rc.Count
	}
	stats["by_region"] = byRegion

	return stats, nil
}

// CloudECSDelByAccountId 删除指定账号的所有 ECS
func CloudECSDelByAccountId(c *ctx.Context, accountId int64) error {
	return DB(c).Where("account_id = ?", accountId).Delete(new(CloudECS)).Error
}

// CloudECSDelByProvider 删除指定云厂商和实例ID的 ECS
func CloudECSDelByProvider(c *ctx.Context, provider string, instanceIds []string) error {
	if len(instanceIds) == 0 {
		return nil
	}
	return DB(c).Where("provider = ? AND instance_id IN ?", provider, instanceIds).Delete(new(CloudECS)).Error
}

// CloudECSGetInstanceIdsByAccountId 获取账号下所有实例ID
func CloudECSGetInstanceIdsByAccountId(c *ctx.Context, accountId int64) ([]string, error) {
	var instanceIds []string
	err := DB(c).Model(&CloudECS{}).Where("account_id = ?", accountId).Pluck("instance_id", &instanceIds).Error
	return instanceIds, err
}
