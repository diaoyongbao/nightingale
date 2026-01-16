// n9e-2kai: 云同步配置模型
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// CloudSyncConfig 云同步配置
type CloudSyncConfig struct {
	Id        int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountId int64 `json:"account_id" gorm:"not null;uniqueIndex:idx_account_resource"`

	// 资源类型: ecs, rds, etc.
	ResourceType string `json:"resource_type" gorm:"type:varchar(32);not null;uniqueIndex:idx_account_resource"`

	// 同步配置
	Enabled      bool `json:"enabled" gorm:"default:true"`
	SyncInterval int  `json:"sync_interval" gorm:"default:300"` // 同步间隔（秒）

	// 区域过滤（JSON 数组，为空表示同步所有区域）
	Regions     string   `json:"-" gorm:"type:text"`
	RegionsList []string `json:"regions" gorm:"-"`

	// 过滤条件（JSON 格式）
	Filters    string                 `json:"-" gorm:"type:text"`
	FiltersMap map[string]interface{} `json:"filters" gorm:"-"`

	// 状态
	LastSyncTime   int64  `json:"last_sync_time" gorm:"default:0"`
	LastSyncStatus int    `json:"last_sync_status" gorm:"default:0"` // 0:未同步 1:成功 2:失败
	LastSyncError  string `json:"last_sync_error" gorm:"type:text"`

	// 统计
	LastSyncCount   int `json:"last_sync_count" gorm:"default:0"`
	LastSyncAdded   int `json:"last_sync_added" gorm:"default:0"`
	LastSyncUpdated int `json:"last_sync_updated" gorm:"default:0"`
	LastSyncDeleted int `json:"last_sync_deleted" gorm:"default:0"`

	// 元数据
	CreateAt int64  `json:"create_at" gorm:"autoCreateTime"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64)"`
	UpdateAt int64  `json:"update_at" gorm:"autoUpdateTime"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64)"`
}

func (CloudSyncConfig) TableName() string {
	return "cloud_sync_config"
}

// 同步状态常量
const (
	SyncStatusNotSynced = 0
	SyncStatusSuccess   = 1
	SyncStatusFailed    = 2
	SyncStatusRunning   = 3
)

// 资源类型常量
const (
	ResourceTypeECS = "ecs"
	ResourceTypeRDS = "rds"
)

// ParseRegions 解析区域列表
func (c *CloudSyncConfig) ParseRegions() error {
	if c.Regions == "" {
		c.RegionsList = []string{}
		return nil
	}
	return json.Unmarshal([]byte(c.Regions), &c.RegionsList)
}

// SerializeRegions 序列化区域列表
func (c *CloudSyncConfig) SerializeRegions() error {
	if len(c.RegionsList) == 0 {
		c.Regions = "[]"
		return nil
	}
	data, err := json.Marshal(c.RegionsList)
	if err != nil {
		return err
	}
	c.Regions = string(data)
	return nil
}

// ParseFilters 解析过滤条件
func (c *CloudSyncConfig) ParseFilters() error {
	if c.Filters == "" {
		c.FiltersMap = make(map[string]interface{})
		return nil
	}
	return json.Unmarshal([]byte(c.Filters), &c.FiltersMap)
}

// SerializeFilters 序列化过滤条件
func (c *CloudSyncConfig) SerializeFilters() error {
	if len(c.FiltersMap) == 0 {
		c.Filters = "{}"
		return nil
	}
	data, err := json.Marshal(c.FiltersMap)
	if err != nil {
		return err
	}
	c.Filters = string(data)
	return nil
}

// Add 添加同步配置
func (c *CloudSyncConfig) Add(ctx *ctx.Context) error {
	c.SerializeRegions()
	c.SerializeFilters()
	return Insert(ctx, c)
}

// Update 更新同步配置
func (c *CloudSyncConfig) Update(ctx *ctx.Context, selectFields ...string) error {
	c.SerializeRegions()
	c.SerializeFilters()
	c.UpdateAt = time.Now().Unix()
	return DB(ctx).Model(c).Select(selectFields).Updates(c).Error
}

// UpdateSyncStatus 更新同步状态
func (c *CloudSyncConfig) UpdateSyncStatus(ctx *ctx.Context, status int, errMsg string, count, added, updated, deleted int) error {
	c.LastSyncTime = time.Now().Unix()
	c.LastSyncStatus = status
	c.LastSyncError = errMsg
	c.LastSyncCount = count
	c.LastSyncAdded = added
	c.LastSyncUpdated = updated
	c.LastSyncDeleted = deleted
	return DB(ctx).Model(c).Updates(map[string]interface{}{
		"last_sync_time":    c.LastSyncTime,
		"last_sync_status":  c.LastSyncStatus,
		"last_sync_error":   c.LastSyncError,
		"last_sync_count":   c.LastSyncCount,
		"last_sync_added":   c.LastSyncAdded,
		"last_sync_updated": c.LastSyncUpdated,
		"last_sync_deleted": c.LastSyncDeleted,
	}).Error
}

// CloudSyncConfigGet 根据 ID 获取同步配置
func CloudSyncConfigGet(c *ctx.Context, id int64) (*CloudSyncConfig, error) {
	var config CloudSyncConfig
	err := DB(c).Where("id = ?", id).First(&config).Error
	if err != nil {
		return nil, err
	}
	config.ParseRegions()
	config.ParseFilters()
	return &config, nil
}

// CloudSyncConfigGetByAccountAndResource 根据账号和资源类型获取同步配置
func CloudSyncConfigGetByAccountAndResource(c *ctx.Context, accountId int64, resourceType string) (*CloudSyncConfig, error) {
	var config CloudSyncConfig
	err := DB(c).Where("account_id = ? AND resource_type = ?", accountId, resourceType).First(&config).Error
	if err != nil {
		return nil, err
	}
	config.ParseRegions()
	config.ParseFilters()
	return &config, nil
}

// CloudSyncConfigGets 获取同步配置列表
func CloudSyncConfigGets(c *ctx.Context, accountId int64, resourceType string, limit, offset int) ([]CloudSyncConfig, error) {
	var configs []CloudSyncConfig
	session := DB(c).Order("id desc")

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if resourceType != "" && resourceType != "all" {
		session = session.Where("resource_type = ?", resourceType)
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&configs).Error
	if err != nil {
		return nil, err
	}

	for i := range configs {
		configs[i].ParseRegions()
		configs[i].ParseFilters()
	}

	return configs, nil
}

// CloudSyncConfigCount 获取同步配置总数
func CloudSyncConfigCount(c *ctx.Context, accountId int64, resourceType string) (int64, error) {
	session := DB(c).Model(&CloudSyncConfig{})

	if accountId > 0 {
		session = session.Where("account_id = ?", accountId)
	}

	if resourceType != "" && resourceType != "all" {
		session = session.Where("resource_type = ?", resourceType)
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// CloudSyncConfigDel 删除同步配置
func CloudSyncConfigDel(c *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(c).Where("id IN ?", ids).Delete(new(CloudSyncConfig)).Error
}

// CloudSyncConfigDelByAccountId 删除账号的所有同步配置
func CloudSyncConfigDelByAccountId(c *ctx.Context, accountId int64) error {
	return DB(c).Where("account_id = ?", accountId).Delete(new(CloudSyncConfig)).Error
}

// CloudSyncConfigGetEnabled 获取所有启用的同步配置
func CloudSyncConfigGetEnabled(c *ctx.Context) ([]CloudSyncConfig, error) {
	var configs []CloudSyncConfig
	err := DB(c).Where("enabled = ?", true).Find(&configs).Error
	if err != nil {
		return nil, err
	}

	for i := range configs {
		configs[i].ParseRegions()
		configs[i].ParseFilters()
	}

	return configs, nil
}

// CloudSyncConfigGetNeedSync 获取需要同步的配置（超过同步间隔）
func CloudSyncConfigGetNeedSync(c *ctx.Context) ([]CloudSyncConfig, error) {
	var configs []CloudSyncConfig
	now := time.Now().Unix()

	// 查找启用且超过同步间隔的配置
	err := DB(c).Where("enabled = ? AND (last_sync_time = 0 OR ? - last_sync_time >= sync_interval)", true, now).Find(&configs).Error
	if err != nil {
		return nil, err
	}

	for i := range configs {
		configs[i].ParseRegions()
		configs[i].ParseFilters()
	}

	return configs, nil
}

// EnsureDefaultSyncConfigs 确保账号有默认的同步配置
func EnsureDefaultSyncConfigs(c *ctx.Context, accountId int64, createBy string) error {
	resourceTypes := []string{ResourceTypeECS, ResourceTypeRDS}

	for _, rt := range resourceTypes {
		var count int64
		DB(c).Model(&CloudSyncConfig{}).Where("account_id = ? AND resource_type = ?", accountId, rt).Count(&count)

		if count == 0 {
			config := &CloudSyncConfig{
				AccountId:    accountId,
				ResourceType: rt,
				Enabled:      true,
				SyncInterval: 300, // 默认5分钟
				CreateBy:     createBy,
			}
			config.Add(c)
		}
	}

	return nil
}

// CloudSyncConfigResetRunningStatus 重置所有"同步中"状态为"失败"
// 用于服务启动时清理上次未完成的同步任务
func CloudSyncConfigResetRunningStatus(c *ctx.Context) (int64, error) {
	result := DB(c).Model(&CloudSyncConfig{}).
		Where("last_sync_status = ?", SyncStatusRunning).
		Updates(map[string]interface{}{
			"last_sync_status": SyncStatusFailed,
			"last_sync_error":  "服务重启，同步任务中断",
		})
	return result.RowsAffected, result.Error
}

