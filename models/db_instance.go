package models

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"github.com/toolkits/pkg/logger"
	"github.com/toolkits/pkg/str"
)

// DBInstance 数据库实例配置
type DBInstance struct {
	Id           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	InstanceName string `json:"instance_name" gorm:"type:varchar(128);not null;uniqueIndex"`
	DBType       string `json:"db_type" gorm:"type:varchar(32);not null;default:'mysql';index"` // mysql, redis, mongodb, postgresql

	// 连接信息
	Host           string `json:"host" gorm:"type:varchar(128);not null"`
	Port           int    `json:"port" gorm:"not null"`
	Username       string `json:"username" gorm:"type:varchar(64);not null"`
	Password       string `json:"-" gorm:"type:varchar(256);not null"`          // 加密存储，不返回给前端
	PlainPassword  string `json:"password" gorm:"-"`                            // 接收前端明文密码，不存储到数据库

	// 数据库配置
	Charset string `json:"charset" gorm:"type:varchar(32);default:'utf8mb4'"`

	// 连接池配置
	MaxConnections int `json:"max_connections" gorm:"default:10"` // 最大连接数
	MaxIdleConns   int `json:"max_idle_conns" gorm:"default:5"`   // 最大空闲连接

	// 实例角色
	IsMaster bool `json:"is_master" gorm:"default:true"` // 是否主库

	// 启用状态
	Enabled bool `json:"enabled" gorm:"default:true"`

	// 健康状态
	HealthStatus   int    `json:"health_status" gorm:"default:0"` // 0:未知 1:健康 2:异常
	LastCheckTime  int64  `json:"last_check_time" gorm:"default:0"`
	LastCheckError string `json:"last_check_error" gorm:"type:text"`

	// 元数据
	Description string `json:"description" gorm:"type:varchar(500)"`
	CreateAt    int64  `json:"create_at" gorm:"not null"`
	CreateBy    string `json:"create_by" gorm:"type:varchar(64);not null"`
	UpdateAt    int64  `json:"update_at" gorm:"not null"`
	UpdateBy    string `json:"update_by" gorm:"type:varchar(64);not null"`
}

func (DBInstance) TableName() string {
	return "db_instance"
}

// 健康状态常量（DBInstance 专用）
const (
	DBHealthStatusUnknown = 0
	DBHealthStatusHealthy = 1
	DBHealthStatusFailed  = 2
)

// 数据库类型常量
const (
	DBTypeMySQL      = "mysql"
	DBTypeRedis      = "redis"
	DBTypeMongoDB    = "mongodb"
	DBTypePostgreSQL = "postgresql"
)

// 简单混淆密钥（实际部署时应从环境变量或配置文件读取）
var encryptionKey = []byte("n9e-dbm-aes256-encryption-key!!!") // 必须正好32字节用于AES-256

// EncryptPassword 加密密码
func EncryptPassword(plaintext string) (string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64 编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPassword 解密密码
func DecryptPassword(ciphertext string) (string, error) {
	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// SetPassword 设置加密密码
func (d *DBInstance) SetPassword(password string) error {
	encrypted, err := EncryptPassword(password)
	if err != nil {
		return err
	}
	d.Password = encrypted
	return nil
}

// GetPassword 获取解密后的密码
func (d *DBInstance) GetPassword() (string, error) {
	password, err := DecryptPassword(d.Password)
	if err != nil {
		logger.Errorf("DecryptPassword failed for instance %s: %v, encrypted: %s", d.InstanceName, err, d.Password)
		return "", err
	}
	logger.Debugf("DecryptPassword success for instance %s, password length: %d", d.InstanceName, len(password))
	return password, nil
}

// Verify 验证实例配置
func (d *DBInstance) Verify() error {
	if str.Dangerous(d.InstanceName) {
		return errors.New("InstanceName has invalid characters")
	}

	if d.InstanceName == "" {
		return errors.New("InstanceName is required")
	}

	if d.DBType == "" {
		d.DBType = DBTypeMySQL
	}

	// 验证数据库类型
	validTypes := []string{DBTypeMySQL, DBTypeRedis, DBTypeMongoDB, DBTypePostgreSQL}
	valid := false
	for _, t := range validTypes {
		if d.DBType == t {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("Invalid db_type, must be one of: %v", validTypes)
	}

	if d.Host == "" {
		return errors.New("Host is required")
	}

	if d.Port <= 0 || d.Port > 65535 {
		return errors.New("Invalid port number")
	}

	if d.Username == "" {
		return errors.New("Username is required")
	}

	if d.Password == "" {
		return errors.New("Password is required")
	}

	// 设置默认值
	if d.MaxConnections <= 0 {
		d.MaxConnections = 10
	}

	if d.MaxIdleConns <= 0 {
		d.MaxIdleConns = 5
	}

	if d.MaxIdleConns > d.MaxConnections {
		d.MaxIdleConns = d.MaxConnections
	}

	if d.Charset == "" {
		d.Charset = "utf8mb4"
	}

	return nil
}

// Add 添加实例
func (d *DBInstance) Add(ctx *ctx.Context) error {
	if err := d.Verify(); err != nil {
		return err
	}

	now := time.Now().Unix()
	d.CreateAt = now
	d.UpdateAt = now
	return Insert(ctx, d)
}

// Update 更新实例
func (d *DBInstance) Update(ctx *ctx.Context, selectFields ...string) error {
	if err := d.Verify(); err != nil {
		return err
	}

	d.UpdateAt = time.Now().Unix()
	return DB(ctx).Model(d).Select(selectFields).Updates(d).Error
}

// DBInstanceGet 根据 ID 获取实例
func DBInstanceGet(ctx *ctx.Context, id int64) (*DBInstance, error) {
	var instance DBInstance
	err := DB(ctx).Where("id = ?", id).First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// DBInstanceGetByName 根据名称获取实例
func DBInstanceGetByName(ctx *ctx.Context, name string) (*DBInstance, error) {
	var instance DBInstance
	err := DB(ctx).Where("instance_name = ?", name).First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// DBInstanceGets 获取实例列表
func DBInstanceGets(ctx *ctx.Context, dbType, query string, limit, offset int) ([]DBInstance, error) {
	var instances []DBInstance
	session := DB(ctx).Order("id desc")

	if dbType != "" && dbType != "all" {
		session = session.Where("db_type = ?", dbType)
	}

	if query != "" {
		session = session.Where("instance_name like ? or host like ? or description like ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	if limit > 0 {
		session = session.Limit(limit).Offset(offset)
	}

	err := session.Find(&instances).Error
	return instances, err
}

// DBInstanceCount 获取实例总数
func DBInstanceCount(ctx *ctx.Context, dbType, query string) (int64, error) {
	session := DB(ctx).Model(&DBInstance{})

	if dbType != "" && dbType != "all" {
		session = session.Where("db_type = ?", dbType)
	}

	if query != "" {
		session = session.Where("instance_name like ? or host like ? or description like ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%")
	}

	var count int64
	err := session.Count(&count).Error
	return count, err
}

// DBInstanceGetsEnabled 获取所有启用的实例
func DBInstanceGetsEnabled(ctx *ctx.Context) ([]DBInstance, error) {
	var instances []DBInstance
	err := DB(ctx).Where("enabled = ?", true).Find(&instances).Error
	return instances, err
}

// DBInstanceGetsByType 根据数据库类型获取实例
func DBInstanceGetsByType(ctx *ctx.Context, dbType string) ([]DBInstance, error) {
	var instances []DBInstance
	err := DB(ctx).Where("db_type = ? AND enabled = ?", dbType, true).Find(&instances).Error
	return instances, err
}

// DBInstanceDel 删除实例
func DBInstanceDel(ctx *ctx.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return DB(ctx).Where("id in ?", ids).Delete(new(DBInstance)).Error
}

// UpdateHealthStatus 更新健康状态
func (d *DBInstance) UpdateHealthStatus(ctx *ctx.Context, status int, errMsg string) error {
	d.HealthStatus = status
	d.LastCheckTime = time.Now().Unix()
	d.LastCheckError = errMsg
	return DB(ctx).Model(d).Select("health_status", "last_check_time", "last_check_error").Updates(d).Error
}

// UpdateEnabled 更新启用状态
func (d *DBInstance) UpdateEnabled(ctx *ctx.Context, enabled bool) error {
	d.Enabled = enabled
	d.UpdateAt = time.Now().Unix()
	return DB(ctx).Model(d).Select("enabled", "update_at").Updates(d).Error
}

// GetDSN 获取数据源名称（用于连接数据库）
func (d *DBInstance) GetDSN(database string) (string, error) {
	password, err := d.GetPassword()
	if err != nil {
		return "", err
	}

	switch d.DBType {
	case DBTypeMySQL:
		// user:password@tcp(host:port)/database?charset=utf8mb4&parseTime=true&loc=Local
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local&timeout=5s",
			d.Username, password, d.Host, d.Port, database, d.Charset), nil

	case DBTypePostgreSQL:
		// host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			d.Host, d.Port, d.Username, password, database), nil

	default:
		return "", fmt.Errorf("unsupported database type: %s", d.DBType)
	}
}
