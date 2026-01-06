package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/ccfos/nightingale/v6/pkg/secu"

	"github.com/pkg/errors"
	"github.com/toolkits/pkg/logger"
	"github.com/toolkits/pkg/str"
	"gorm.io/gorm"
)

// MiddlewareDatasource 中间件数据源模型
type MiddlewareDatasource struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"uniqueIndex;size:191;not null"`
	Type        string `json:"type" gorm:"index;size:64;not null"`
	Description string `json:"description" gorm:"size:500"`
	Address     string `json:"address" gorm:"size:500;not null"`
	Status      string `json:"status" gorm:"index;size:32;default:enabled"`

	// 连接配置
	Timeout            int  `json:"timeout" gorm:"default:5000"`
	ConnectTimeout     int  `json:"connect_timeout" gorm:"default:2000"`
	InsecureSkipVerify bool `json:"insecure_skip_verify" gorm:"default:false"`

	// 认证配置
	AuthType          string                 `json:"auth_type" gorm:"size:32;not null"`
	AuthConfig        string                 `json:"-" gorm:"column:auth_config;type:text"`
	AuthConfigJson    map[string]interface{} `json:"auth_config" gorm:"-"`
	AuthConfigEncoded string                 `json:"auth_config_encoded,omitempty" gorm:"column:auth_config_encoded;type:text"`

	// 扩展配置
	Settings        string                 `json:"-" gorm:"column:settings;type:text"`
	SettingsJson    map[string]interface{} `json:"settings" gorm:"-"`
	SettingsEncoded string                 `json:"settings_encoded,omitempty" gorm:"column:settings_encoded;type:text"`

	// 健康检查
	HealthCheckUrl      string `json:"health_check_url" gorm:"size:500"`
	HealthCheckInterval int    `json:"health_check_interval" gorm:"default:60"`
	LastHealthCheck     int64  `json:"last_health_check" gorm:"default:0"`
	HealthStatus        string `json:"health_status" gorm:"size:32;default:unknown"`
	HealthMessage       string `json:"health_message" gorm:"size:500"`

	// 元信息
	Tags      string `json:"tags" gorm:"size:500"`
	OrderNum  int    `json:"order_num" gorm:"default:0"`
	CreatedAt int64  `json:"created_at" gorm:"not null"`
	CreatedBy string `json:"created_by" gorm:"size:64;not null"`
	UpdatedAt int64  `json:"updated_at" gorm:"not null"`
	UpdatedBy string `json:"updated_by" gorm:"size:64;not null"`
}

func (MiddlewareDatasource) TableName() string {
	return "middleware_datasource"
}

// 中间件类型常量
const (
	MiddlewareTypeArchery    = "archery"
	MiddlewareTypeJumpServer = "jumpserver"
	MiddlewareTypeJenkins    = "jenkins"
	MiddlewareTypeGitLab     = "gitlab"
	MiddlewareTypeNacos      = "nacos"
	MiddlewareTypeConsul     = "consul"
)

// 认证类型常量
const (
	AuthTypeToken   = "token"
	AuthTypeBasic   = "basic"
	AuthTypeSession = "session"
	AuthTypeOAuth2  = "oauth2"
	AuthTypeNone    = "none"
)

// 状态常量
const (
	MiddlewareStatusEnabled  = "enabled"
	MiddlewareStatusDisabled = "disabled"
)

// 健康状态常量
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusUnknown   = "unknown"
)

// Verify 验证数据
func (mds *MiddlewareDatasource) Verify() error {
	if str.Dangerous(mds.Name) {
		return errors.New("Name has invalid characters")
	}

	if mds.Address == "" {
		return errors.New("Address is required")
	}

	if mds.Type == "" {
		return errors.New("Type is required")
	}

	if mds.AuthType == "" {
		return errors.New("AuthType is required")
	}

	return mds.FE2DB()
}

// Add 添加中间件数据源
func (mds *MiddlewareDatasource) Add(ctx *ctx.Context) error {
	if err := mds.Verify(); err != nil {
		return err
	}

	now := time.Now().Unix()
	mds.CreatedAt = now
	mds.UpdatedAt = now
	return Insert(ctx, mds)
}

// Update 更新中间件数据源
func (mds *MiddlewareDatasource) Update(ctx *ctx.Context, selectField interface{}, selectFields ...interface{}) error {
	if err := mds.Verify(); err != nil {
		return err
	}

	if mds.UpdatedAt == 0 {
		mds.UpdatedAt = time.Now().Unix()
	}
	return DB(ctx).Model(mds).Session(&gorm.Session{SkipHooks: true}).Select(selectField, selectFields...).Updates(mds).Error
}

// MiddlewareDatasourceDel 删除中间件数据源
func MiddlewareDatasourceDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(new(MiddlewareDatasource)).Error
}

// MiddlewareDatasourceGet 根据ID获取中间件数据源
func MiddlewareDatasourceGet(ctx *ctx.Context, id int64) (*MiddlewareDatasource, error) {
	var mds *MiddlewareDatasource
	err := DB(ctx).Where("id = ?", id).First(&mds).Error
	if err != nil {
		return nil, err
	}
	return mds, mds.DB2FE()
}

// MiddlewareDatasourceGetByName 根据名称获取中间件数据源
func MiddlewareDatasourceGetByName(ctx *ctx.Context, name string) (*MiddlewareDatasource, error) {
	var mds *MiddlewareDatasource
	err := DB(ctx).Where("name = ?", name).First(&mds).Error
	if err != nil {
		return nil, err
	}
	return mds, mds.DB2FE()
}

// MiddlewareDatasourceExists 检查中间件数据源是否存在
func MiddlewareDatasourceExists(ctx *ctx.Context, name string) (bool, error) {
	return Exists(DB(ctx).Model(&MiddlewareDatasource{}).Where("name = ?", name))
}

// GetMiddlewareDatasources 获取所有中间件数据源
func GetMiddlewareDatasources(ctx *ctx.Context) ([]*MiddlewareDatasource, error) {
	var mds []*MiddlewareDatasource
	err := DB(ctx).Order("order_num asc, id desc").Find(&mds).Error

	for i := 0; i < len(mds); i++ {
		mds[i].DB2FE()
	}

	return mds, err
}

// GetMiddlewareDatasourcesByType 根据类型获取中间件数据源
func GetMiddlewareDatasourcesByType(ctx *ctx.Context, middlewareType string) ([]*MiddlewareDatasource, error) {
	var mds []*MiddlewareDatasource
	err := DB(ctx).Where("type = ?", middlewareType).Order("order_num asc, id desc").Find(&mds).Error

	for i := 0; i < len(mds); i++ {
		mds[i].DB2FE()
	}

	return mds, err
}

// GetMiddlewareDatasourcesBy 根据条件获取中间件数据源
func GetMiddlewareDatasourcesBy(ctx *ctx.Context, middlewareType, status, keyword string, limit, offset int) ([]*MiddlewareDatasource, error) {
	session := DB(ctx)

	if middlewareType != "" {
		session = session.Where("type = ?", middlewareType)
	}

	if status != "" {
		session = session.Where("status = ?", status)
	}

	if keyword != "" {
		keyword = "%" + keyword + "%"
		session = session.Where("name LIKE ? OR description LIKE ?", keyword, keyword)
	}

	var mds []*MiddlewareDatasource
	err := session.Order("order_num asc, id desc").Limit(limit).Offset(offset).Find(&mds).Error
	if err == nil {
		for i := 0; i < len(mds); i++ {
			mds[i].DB2FE()
		}
	}
	return mds, err
}

// GetMiddlewareDatasourcesCount 获取中间件数据源数量
func GetMiddlewareDatasourcesCount(ctx *ctx.Context, middlewareType, status, keyword string) (int64, error) {
	session := DB(ctx).Model(&MiddlewareDatasource{})

	if middlewareType != "" {
		session = session.Where("type = ?", middlewareType)
	}

	if status != "" {
		session = session.Where("status = ?", status)
	}

	if keyword != "" {
		keyword = "%" + keyword + "%"
		session = session.Where("name LIKE ? OR description LIKE ?", keyword, keyword)
	}

	return Count(session)
}

// FE2DB 前端到数据库转换
func (mds *MiddlewareDatasource) FE2DB() error {
	if mds.AuthConfigJson != nil {
		b, err := json.Marshal(mds.AuthConfigJson)
		if err != nil {
			return err
		}
		mds.AuthConfig = string(b)
	}

	if mds.SettingsJson != nil {
		b, err := json.Marshal(mds.SettingsJson)
		if err != nil {
			return err
		}
		mds.Settings = string(b)
	}

	return nil
}

// DB2FE 数据库到前端转换
func (mds *MiddlewareDatasource) DB2FE() error {
	if mds.AuthConfig != "" {
		err := json.Unmarshal([]byte(mds.AuthConfig), &mds.AuthConfigJson)
		if err != nil {
			return err
		}
	}

	if mds.Settings != "" {
		err := json.Unmarshal([]byte(mds.Settings), &mds.SettingsJson)
		if err != nil {
			return err
		}
	}

	return nil
}

// Encrypt 加密敏感信息
func (mds *MiddlewareDatasource) Encrypt(openRsa bool, publicKeyData []byte) error {
	if !openRsa {
		return nil
	}

	if mds.AuthConfig != "" {
		encVal, err := secu.EncryptValue(mds.AuthConfig, publicKeyData)
		if err != nil {
			logger.Errorf("encrypt auth_config failed: datasource=%s err=%v", mds.Name, err)
			return err
		}
		mds.AuthConfigEncoded = encVal
	}

	if mds.Settings != "" {
		encVal, err := secu.EncryptValue(mds.Settings, publicKeyData)
		if err != nil {
			logger.Errorf("encrypt settings failed: datasource=%s err=%v", mds.Name, err)
			return err
		}
		mds.SettingsEncoded = encVal
	}

	mds.ClearPlaintext()
	return nil
}

// Decrypt 解密敏感信息
func (mds *MiddlewareDatasource) Decrypt(privateKeyData []byte, password string) error {
	if mds.AuthConfigEncoded != "" {
		authConfig, err := secu.Decrypt(mds.AuthConfigEncoded, privateKeyData, password)
		if err != nil {
			return err
		}
		mds.AuthConfig = authConfig
		err = json.Unmarshal([]byte(authConfig), &mds.AuthConfigJson)
		if err != nil {
			return err
		}
	}

	if mds.SettingsEncoded != "" {
		settings, err := secu.Decrypt(mds.SettingsEncoded, privateKeyData, password)
		if err != nil {
			return err
		}
		mds.Settings = settings
		err = json.Unmarshal([]byte(settings), &mds.SettingsJson)
		if err != nil {
			return err
		}
	}
	return nil
}

// ClearPlaintext 清除明文敏感信息
func (mds *MiddlewareDatasource) ClearPlaintext() {
	mds.AuthConfig = ""
	mds.AuthConfigJson = nil
	mds.Settings = ""
	mds.SettingsJson = nil
}

// UpdateHealthStatus 更新健康状态
func (mds *MiddlewareDatasource) UpdateHealthStatus(ctx *ctx.Context, status, message string) error {
	now := time.Now().Unix()
	return DB(ctx).Model(mds).Updates(map[string]interface{}{
		"health_status":     status,
		"health_message":    message,
		"last_health_check": now,
		"updated_at":        now,
	}).Error
}

// MiddlewareDatasourceStatistics 统计信息
func MiddlewareDatasourceStatistics(ctx *ctx.Context) (*Statistics, error) {
	session := DB(ctx).Model(&MiddlewareDatasource{}).Select("count(*) as total", "max(updated_at) as last_updated")

	var stats []*Statistics
	err := session.Find(&stats).Error
	if err != nil {
		return nil, err
	}

	return stats[0], nil
}

// GetMiddlewareDatasourceTypes 获取所有中间件类型及数量
func GetMiddlewareDatasourceTypes(ctx *ctx.Context) (map[string]int64, error) {
	type TypeCount struct {
		Type  string
		Count int64
	}

	var results []TypeCount
	err := DB(ctx).Model(&MiddlewareDatasource{}).
		Select("type, count(*) as count").
		Group("type").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	typeMap := make(map[string]int64)
	for _, r := range results {
		typeMap[r.Type] = r.Count
	}

	return typeMap, nil
}

// GetAuthConfigString 获取认证配置中的字符串值
func (mds *MiddlewareDatasource) GetAuthConfigString(key string) string {
	if mds.AuthConfigJson == nil {
		return ""
	}
	if val, ok := mds.AuthConfigJson[key].(string); ok {
		return val
	}
	return ""
}

// GetSettingsString 获取扩展配置中的字符串值
func (mds *MiddlewareDatasource) GetSettingsString(key string) string {
	if mds.SettingsJson == nil {
		return ""
	}
	if val, ok := mds.SettingsJson[key].(string); ok {
		return val
	}
	return ""
}

// GetSettingsBool 获取扩展配置中的布尔值
func (mds *MiddlewareDatasource) GetSettingsBool(key string) bool {
	if mds.SettingsJson == nil {
		return false
	}
	if val, ok := mds.SettingsJson[key].(bool); ok {
		return val
	}
	return false
}

// IsEnabled 检查是否启用
func (mds *MiddlewareDatasource) IsEnabled() bool {
	return mds.Status == MiddlewareStatusEnabled
}

// IsHealthy 检查是否健康
func (mds *MiddlewareDatasource) IsHealthy() bool {
	return mds.HealthStatus == HealthStatusHealthy
}

// GetEnabledMiddlewareDatasourcesByType 获取启用的指定类型中间件数据源
func GetEnabledMiddlewareDatasourcesByType(ctx *ctx.Context, middlewareType string) ([]*MiddlewareDatasource, error) {
	var mds []*MiddlewareDatasource
	err := DB(ctx).Where("type = ? AND status = ?", middlewareType, MiddlewareStatusEnabled).
		Order("order_num asc, id desc").Find(&mds).Error

	for i := 0; i < len(mds); i++ {
		mds[i].DB2FE()
	}

	return mds, err
}

// ValidateAuthConfig 验证认证配置
func (mds *MiddlewareDatasource) ValidateAuthConfig() error {
	if mds.AuthConfigJson == nil {
		return errors.New("auth_config is required")
	}

	switch mds.AuthType {
	case AuthTypeToken:
		token := mds.GetAuthConfigString("token")
		if token == "" {
			return errors.New("token is required for token auth type")
		}
	case AuthTypeBasic:
		username := mds.GetAuthConfigString("username")
		password := mds.GetAuthConfigString("password")
		if username == "" || password == "" {
			return errors.New("username and password are required for basic auth type")
		}
	case AuthTypeSession:
		username := mds.GetAuthConfigString("username")
		password := mds.GetAuthConfigString("password")
		loginUrl := mds.GetAuthConfigString("login_url")
		if username == "" || password == "" || loginUrl == "" {
			return errors.New("username, password and login_url are required for session auth type")
		}
	case AuthTypeOAuth2:
		clientId := mds.GetAuthConfigString("client_id")
		clientSecret := mds.GetAuthConfigString("client_secret")
		tokenUrl := mds.GetAuthConfigString("token_url")
		if clientId == "" || clientSecret == "" || tokenUrl == "" {
			return errors.New("client_id, client_secret and token_url are required for oauth2 auth type")
		}
	case AuthTypeNone:
		// No validation needed
	default:
		return fmt.Errorf("unsupported auth type: %s", mds.AuthType)
	}

	return nil
}
