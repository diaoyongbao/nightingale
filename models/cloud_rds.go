// n9e-2kai: 云数据库资源模型
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudRDS 云数据库资源
type CloudRDS struct {
	Id        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountId int64  `json:"account_id" gorm:"not null;index"`
	Provider  string `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:idx_rds_provider_instance"`
	Region    string `json:"region" gorm:"type:varchar(64);not null;index"`

	// 资源唯一标识
	InstanceId   string `json:"instance_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_rds_provider_instance"`
	InstanceName string `json:"instance_name" gorm:"type:varchar(256)"`

	// 基础信息
	Status        string `json:"status" gorm:"type:varchar(32);default:'unknown';index"`
	Engine        string `json:"engine" gorm:"type:varchar(32);not null;index"` // mysql, postgresql, sqlserver, etc.
	EngineVersion string `json:"engine_version" gorm:"type:varchar(32)"`
	InstanceType  string `json:"instance_type" gorm:"type:varchar(64)"`

	// 配置信息
	CPU         int    `json:"cpu" gorm:"default:0"`
	Memory      int    `json:"memory" gorm:"default:0"`              // MB
	Storage     int    `json:"storage" gorm:"default:0"`             // GB
	StorageType string `json:"storage_type" gorm:"type:varchar(32)"` // ssd, ultrahigh, etc.

	// 网络信息
	VpcId     string `json:"vpc_id" gorm:"type:varchar(128)"`
	SubnetId  string `json:"subnet_id" gorm:"type:varchar(128)"`
	PrivateIp string `json:"private_ip" gorm:"type:varchar(64)"`
	PublicIp  string `json:"public_ip" gorm:"type:varchar(64)"`
	Port      int    `json:"port" gorm:"default:0"`

	// 高可用配置
	HaMode           string `json:"ha_mode" gorm:"type:varchar(32)"` // single, ha, readonly
	MasterInstanceId string `json:"master_instance_id" gorm:"type:varchar(128);index"`
	AvailabilityZone string `json:"availability_zone" gorm:"type:varchar(128)"`

	// 计费信息
	ChargeType string `json:"charge_type" gorm:"type:varchar(32)"`
	ExpireTime int64  `json:"expire_time" gorm:"default:0"`

	// 备份配置
	BackupPolicy string `json:"-" gorm:"type:text"`

	// 标签
	Tags    string            `json:"-" gorm:"type:text"`
	TagsMap map[string]string `json:"tags" gorm:"-"`

	// 元数据
	RawData    string `json:"-" gorm:"type:text"`
	SyncTime   int64  `json:"sync_time" gorm:"not null;index"`
	CreateTime int64  `json:"create_time" gorm:"default:0"` // 实例创建时间（云端）
}

func (CloudRDS) TableName() string {
	return "cloud_rds"
}

// RDS 引擎类型
const (
	RDSEngineMySQL      = "mysql"
	RDSEnginePostgreSQL = "postgresql"
	RDSEngineSQLServer  = "sqlserver"
	RDSEngineMariaDB    = "mariadb"
)

// RDS 高可用模式
const (
	RDSHaModeSingle   = "single"
	RDSHaModeHA       = "ha"
	RDSHaModeReadOnly = "readonly"
)

// ParseTags 解析标签
func (r *CloudRDS) ParseTags() error {
	if r.Tags == "" {
		r.TagsMap = make(map[string]string)
		return nil
	}
	return json.Unmarshal([]byte(r.Tags), &r.TagsMap)
}

// SerializeTags 序列化标签
func (r *CloudRDS) SerializeTags() error {
	if len(r.TagsMap) == 0 {
		r.Tags = "{}"
		return nil
	}
	data, err := json.Marshal(r.TagsMap)
	if err != nil {
		return err
	}
	r.Tags = string(data)
	return nil
}

// Add 添加 RDS 资源
func (r *CloudRDS) Add(c *ctx.Context) error {
	r.SyncTime = time.Now().Unix()
	return Insert(c, r)
}

// Update 更新 RDS 资源
func (r *CloudRDS) Update(c *ctx.Context) error {
	r.SyncTime = time.Now().Unix()
	return DB(c).Model(r).Updates(r).Error
}

// CloudRDSGet 根据 ID 获取 RDS
func CloudRDSGet(c *ctx.Context, id int64) (*CloudRDS, error) {
	var rds CloudRDS
	err := DB(c).Where("id = ?", id).First(&rds).Error
	if err != nil {
		return nil, err
	}
	rds.ParseTags()
	return &rds, nil
}

// CloudRDSGetByInstanceId 根据云厂商和实例ID获取 RDS
func CloudRDSGetByInstanceId(c *ctx.Context, provider, instanceId string) (*CloudRDS, error) {
	var rds CloudRDS
	err := DB(c).Where("provider = ? AND instance_id = ?", provider, instanceId).First(&rds).Error
	if err != nil {
		return nil, err
	}
	rds.ParseTags()
	return &rds, nil
}

// CloudRDSGets 获取 RDS 列表
// CloudRDSGets 获取 RDS 列表
func CloudRDSGets(c *ctx.Context, accountId int64, provider, region, engine, status, query, owner string, limit, offset int) ([]CloudRDS, error) {
	var rdsList []CloudRDS
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

	if engine != "" && engine != "all" {
		session = session.Where("engine = ?", engine)
	}

	if status != "" && status != "all" {
		session = session.Where("status = ?", status)
	}

	if query != "" {
		session = session.Where("instance_name like ? or instance_id like ? or private_ip like ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	if owner != "" && owner != "all" {
		session = session.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&rdsList).Error
	if err != nil {
		return nil, err
	}

	for i := range rdsList {
		rdsList[i].ParseTags()
	}

	return rdsList, nil
}

// CloudRDSCount 获取 RDS 总数
// CloudRDSCount 获取 RDS 总数
func CloudRDSCount(c *ctx.Context, accountId int64, provider, region, engine, status, query, owner string) (int64, error) {
	session := DB(c).Model(&CloudRDS{})

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if region != "" && region != "all" {
		session = session.Where("region = ?", region)
	}

	if engine != "" && engine != "all" {
		session = session.Where("engine = ?", engine)
	}

	if status != "" && status != "all" {
		session = session.Where("status = ?", status)
	}

	if query != "" {
		session = session.Where("instance_name like ? or instance_id like ? or private_ip like ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	if owner != "" && owner != "all" {
		session = session.Where("instance_id IN (SELECT instance_id FROM cloud_rds_owner WHERE owner = ?)", owner)
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// CloudRDSStats 获取 RDS 统计信息
func CloudRDSStats(c *ctx.Context, accountId int64, provider string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	session := DB(c).Model(&CloudRDS{})
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

	// 按引擎统计
	type EngineCount struct {
		Engine string
		Count  int64
	}
	var engineCounts []EngineCount
	DB(c).Model(&CloudRDS{}).Select("engine, count(*) as count").Group("engine").Scan(&engineCounts)
	byEngine := make(map[string]int64)
	for _, ec := range engineCounts {
		byEngine[ec.Engine] = ec.Count
	}
	stats["by_engine"] = byEngine

	// 按状态统计
	type StatusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []StatusCount
	DB(c).Model(&CloudRDS{}).Select("status, count(*) as count").Group("status").Scan(&statusCounts)
	byStatus := make(map[string]int64)
	for _, sc := range statusCounts {
		byStatus[sc.Status] = sc.Count
	}
	stats["by_status"] = byStatus

	// 按高可用模式统计
	type HaModeCount struct {
		HaMode string
		Count  int64
	}
	var haModeCounts []HaModeCount
	DB(c).Model(&CloudRDS{}).Select("ha_mode, count(*) as count").Group("ha_mode").Scan(&haModeCounts)
	byHaMode := make(map[string]int64)
	for _, hc := range haModeCounts {
		byHaMode[hc.HaMode] = hc.Count
	}
	stats["by_ha_mode"] = byHaMode

	return stats, nil
}

// CloudRDSDelByAccountId 删除指定账号的所有 RDS
func CloudRDSDelByAccountId(c *ctx.Context, accountId int64) error {
	return DB(c).Where("account_id = ?", accountId).Delete(new(CloudRDS)).Error
}

// CloudRDSDelByProvider 删除指定云厂商和实例ID的 RDS
func CloudRDSDelByProvider(c *ctx.Context, provider string, instanceIds []string) error {
	if len(instanceIds) == 0 {
		return nil
	}
	return DB(c).Where("provider = ? AND instance_id IN ?", provider, instanceIds).Delete(new(CloudRDS)).Error
}

// CloudRDSGetInstanceIdsByAccountId 获取账号下所有实例ID
func CloudRDSGetInstanceIdsByAccountId(c *ctx.Context, accountId int64) ([]string, error) {
	var instanceIds []string
	err := DB(c).Model(&CloudRDS{}).Where("account_id = ?", accountId).Pluck("instance_id", &instanceIds).Error
	return instanceIds, err
}

// n9e-2kai: CloudRDSGetsByAccountId 根据账号 ID 获取所有 RDS 实例
func CloudRDSGetsByAccountId(c *ctx.Context, accountId int64) ([]CloudRDS, error) {
	var rdsList []CloudRDS
	err := DB(c).Where("account_id = ?", accountId).Find(&rdsList).Error
	if err != nil {
		return nil, err
	}
	for i := range rdsList {
		rdsList[i].ParseTags()
	}
	return rdsList, nil
}

// CloudRDSGetReplicas 获取只读实例列表
func CloudRDSGetReplicas(c *ctx.Context, masterInstanceId string) ([]CloudRDS, error) {
	var replicas []CloudRDS
	err := DB(c).Where("master_instance_id = ?", masterInstanceId).Find(&replicas).Error
	if err != nil {
		return nil, err
	}
	for i := range replicas {
		replicas[i].ParseTags()
	}
	return replicas, nil
}
