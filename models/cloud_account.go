// n9e-2kai: 云账号配置模型
package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"github.com/toolkits/pkg/str"
)

// CloudAccount 云账号配置
type CloudAccount struct {
	Id          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	Provider    string `json:"provider" gorm:"type:varchar(32);not null;default:'huawei';index"` // huawei, aliyun, tencent, volcengine
	Description string `json:"description" gorm:"type:varchar(500)"`

	// 认证信息（加密存储）
	AccessKey      string `json:"-" gorm:"type:varchar(256);not null"` // 加密存储，不返回给前端
	SecretKey      string `json:"-" gorm:"type:varchar(512);not null"` // 加密存储，不返回给前端
	PlainAccessKey string `json:"access_key" gorm:"-"`                 // 接收前端输入/返回脱敏值
	PlainSecretKey string `json:"secret_key,omitempty" gorm:"-"`       // 接收前端输入，返回时不包含

	// 区域配置（JSON 数组）
	Regions       string   `json:"-" gorm:"type:text"` // JSON 格式存储
	RegionList    []string `json:"regions" gorm:"-"`   // JSON 解析后的区域列表
	DefaultRegion string   `json:"default_region" gorm:"type:varchar(64)"`

	// 同步配置
	SyncEnabled    bool   `json:"sync_enabled" gorm:"default:true"`
	SyncInterval   int    `json:"sync_interval" gorm:"default:300"` // 同步间隔(秒)，默认5分钟
	LastSyncTime   int64  `json:"last_sync_time" gorm:"default:0"`
	LastSyncStatus int    `json:"last_sync_status" gorm:"default:0"` // 0:未同步, 1:成功, 2:失败
	LastSyncError  string `json:"last_sync_error,omitempty" gorm:"type:text"`

	// 状态字段
	Enabled        bool   `json:"enabled" gorm:"default:true"`
	HealthStatus   int    `json:"health_status" gorm:"default:0"` // 0:未知, 1:健康, 2:异常
	LastCheckTime  int64  `json:"last_check_time" gorm:"default:0"`
	LastCheckError string `json:"last_check_error,omitempty" gorm:"type:text"`

	// 元数据
	CreateAt int64  `json:"create_at" gorm:"not null"`
	CreateBy string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt int64  `json:"update_at" gorm:"not null"`
	UpdateBy string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (CloudAccount) TableName() string {
	return "cloud_account"
}

// 云厂商常量
const (
	CloudProviderHuawei     = "huawei"
	CloudProviderAliyun     = "aliyun"
	CloudProviderTencent    = "tencent"
	CloudProviderVolcengine = "volcengine"
)

// 同步状态常量
const (
	CloudSyncStatusNotSynced = 0
	CloudSyncStatusSuccess   = 1
	CloudSyncStatusFailed    = 2
)

// 健康状态常量
const (
	CloudHealthUnknown = 0
	CloudHealthHealthy = 1
	CloudHealthFailed  = 2
)

// 支持的云厂商列表
var SupportedProviders = []string{
	CloudProviderHuawei,
	CloudProviderAliyun,
	CloudProviderTencent,
	CloudProviderVolcengine,
}

// Verify 验证账号配置
func (a *CloudAccount) Verify() error {
	if str.Dangerous(a.Name) {
		return errors.New("Name has invalid characters")
	}

	if a.Name == "" {
		return errors.New("Name is required")
	}

	if a.Provider == "" {
		a.Provider = CloudProviderHuawei
	}

	// 验证云厂商类型
	valid := false
	for _, p := range SupportedProviders {
		if a.Provider == p {
			valid = true
			break
		}
	}
	if !valid {
		return errors.Errorf("Invalid provider, must be one of: %v", SupportedProviders)
	}

	if a.AccessKey == "" && a.PlainAccessKey == "" {
		return errors.New("AccessKey is required")
	}

	// 同步间隔最小60秒
	if a.SyncInterval < 60 {
		a.SyncInterval = 300
	}

	return nil
}

// SetCredentials 设置加密凭证
func (a *CloudAccount) SetCredentials(accessKey, secretKey string) error {
	encAK, err := EncryptPassword(accessKey)
	if err != nil {
		return errors.Wrap(err, "encrypt access_key failed")
	}
	a.AccessKey = encAK

	encSK, err := EncryptPassword(secretKey)
	if err != nil {
		return errors.Wrap(err, "encrypt secret_key failed")
	}
	a.SecretKey = encSK

	return nil
}

// GetCredentials 获取解密后的凭证
func (a *CloudAccount) GetCredentials() (accessKey, secretKey string, err error) {
	accessKey, err = DecryptPassword(a.AccessKey)
	if err != nil {
		return "", "", errors.Wrap(err, "decrypt access_key failed")
	}

	secretKey, err = DecryptPassword(a.SecretKey)
	if err != nil {
		return "", "", errors.Wrap(err, "decrypt secret_key failed")
	}

	return accessKey, secretKey, nil
}

// MaskAccessKey 脱敏 AccessKey
func (a *CloudAccount) MaskAccessKey() string {
	ak, err := DecryptPassword(a.AccessKey)
	if err != nil || len(ak) < 8 {
		return "****"
	}
	return ak[:4] + "****" + ak[len(ak)-4:]
}

// ParseRegions 解析区域列表
func (a *CloudAccount) ParseRegions() error {
	if a.Regions == "" {
		a.RegionList = []string{}
		return nil
	}
	return json.Unmarshal([]byte(a.Regions), &a.RegionList)
}

// SerializeRegions 序列化区域列表
func (a *CloudAccount) SerializeRegions() error {
	if len(a.RegionList) == 0 {
		a.Regions = "[]"
		return nil
	}
	data, err := json.Marshal(a.RegionList)
	if err != nil {
		return err
	}
	a.Regions = string(data)
	return nil
}

// Add 添加云账号
func (a *CloudAccount) Add(c *ctx.Context) error {
	if err := a.Verify(); err != nil {
		return err
	}

	// 加密凭证
	if a.PlainAccessKey != "" {
		if err := a.SetCredentials(a.PlainAccessKey, a.PlainSecretKey); err != nil {
			return err
		}
	}

	// 序列化区域
	if err := a.SerializeRegions(); err != nil {
		return err
	}

	now := time.Now().Unix()
	a.CreateAt = now
	a.UpdateAt = now
	return Insert(c, a)
}

// Update 更新云账号
func (a *CloudAccount) Update(c *ctx.Context, selectFields ...string) error {
	if err := a.Verify(); err != nil {
		return err
	}

	// 序列化区域
	if err := a.SerializeRegions(); err != nil {
		return err
	}

	a.UpdateAt = time.Now().Unix()
	return DB(c).Model(a).Select(selectFields).Updates(a).Error
}

// CloudAccountGet 根据 ID 获取云账号
func CloudAccountGet(c *ctx.Context, id int64) (*CloudAccount, error) {
	var account CloudAccount
	err := DB(c).Where("id = ?", id).First(&account).Error
	if err != nil {
		return nil, err
	}
	// 解析区域列表
	account.ParseRegions()
	// 设置脱敏的 AccessKey
	account.PlainAccessKey = account.MaskAccessKey()
	return &account, nil
}

// CloudAccountGetByName 根据名称获取云账号
func CloudAccountGetByName(c *ctx.Context, name string) (*CloudAccount, error) {
	var account CloudAccount
	err := DB(c).Where("name = ?", name).First(&account).Error
	if err != nil {
		return nil, err
	}
	account.ParseRegions()
	account.PlainAccessKey = account.MaskAccessKey()
	return &account, nil
}

// CloudAccountGets 获取云账号列表
func CloudAccountGets(c *ctx.Context, provider, query string, enabled *bool, limit, offset int) ([]CloudAccount, error) {
	var accounts []CloudAccount
	session := DB(c).Order("id desc")

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if query != "" {
		session = session.Where("name like ? or description like ?",
			"%"+query+"%", "%"+query+"%")
	}

	if enabled != nil {
		session = session.Where("enabled = ?", *enabled)
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&accounts).Error
	if err != nil {
		return nil, err
	}

	// 处理每个账号
	for i := range accounts {
		accounts[i].ParseRegions()
		accounts[i].PlainAccessKey = accounts[i].MaskAccessKey()
	}

	return accounts, nil
}

// CloudAccountCount 获取云账号总数
func CloudAccountCount(c *ctx.Context, provider, query string, enabled *bool) (int64, error) {
	session := DB(c).Model(&CloudAccount{})

	if provider != "" && provider != "all" {
		session = session.Where("provider = ?", provider)
	}

	if query != "" {
		session = session.Where("name like ? or description like ?",
			"%"+query+"%", "%"+query+"%")
	}

	if enabled != nil {
		session = session.Where("enabled = ?", *enabled)
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// CloudAccountDel 批量删除云账号
func CloudAccountDel(c *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(c).Where("id in ?", ids).Delete(new(CloudAccount)).Error
}

// CloudAccountGetsEnabled 获取所有启用的云账号
func CloudAccountGetsEnabled(c *ctx.Context) ([]CloudAccount, error) {
	var accounts []CloudAccount
	err := DB(c).Where("enabled = ?", true).Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		accounts[i].ParseRegions()
	}
	return accounts, nil
}

// CloudAccountGetsSyncEnabled 获取所有启用同步的云账号
func CloudAccountGetsSyncEnabled(c *ctx.Context) ([]CloudAccount, error) {
	var accounts []CloudAccount
	err := DB(c).Where("enabled = ? AND sync_enabled = ?", true, true).Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		accounts[i].ParseRegions()
	}
	return accounts, nil
}

// UpdateSyncStatus 更新同步状态
func (a *CloudAccount) UpdateSyncStatus(c *ctx.Context, status int, errMsg string) error {
	a.LastSyncTime = time.Now().Unix()
	a.LastSyncStatus = status
	a.LastSyncError = errMsg
	return DB(c).Model(a).Select("last_sync_time", "last_sync_status", "last_sync_error").Updates(a).Error
}

// UpdateHealthStatus 更新健康状态
func (a *CloudAccount) UpdateHealthStatus(c *ctx.Context, status int, errMsg string) error {
	a.HealthStatus = status
	a.LastCheckTime = time.Now().Unix()
	a.LastCheckError = errMsg
	return DB(c).Model(a).Select("health_status", "last_check_time", "last_check_error").Updates(a).Error
}
