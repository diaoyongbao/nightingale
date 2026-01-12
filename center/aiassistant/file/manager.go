// Package file 文件管理器
// n9e-2kai: AI 助手模块 - 文件管理器
package file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/center/aiassistant"
	"github.com/ccfos/nightingale/v6/storage"
	"github.com/google/uuid"
)

// Manager 文件管理器
type Manager struct {
	redis  storage.Redis
	config *Config
}

// Config 文件管理配置
type Config struct {
	MaxSize      int64         // 最大文件大小（字节）
	StoragePath  string        // 存储路径
	TTL          time.Duration // 文件过期时间
	RedisPrefix  string        // Redis key 前缀
	AllowedTypes []string      // 允许的 MIME 类型
	TokenTTL     time.Duration // 下载 token 过期时间
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxSize:     10 * 1024 * 1024, // 10MB
		StoragePath: "./data/ai_files",
		TTL:         24 * time.Hour,
		RedisPrefix: "ai_assistant:",
		AllowedTypes: []string{
			"image/png", "image/jpeg", "image/gif", "image/webp",
			"text/plain", "text/csv",
			"application/json", "application/pdf",
		},
		TokenTTL: 1 * time.Hour,
	}
}

// FileInfo 文件信息
type FileInfo struct {
	FileID    string `json:"file_id"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
	UserID    int64  `json:"user_id"`
}

// DownloadToken 下载令牌
type DownloadToken struct {
	Token     string `json:"token"`
	FileID    string `json:"file_id"`
	ExpiresAt int64  `json:"expires_at"`
}

// NewManager 创建文件管理器
func NewManager(redis storage.Redis, config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 确保存储目录存在
	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &Manager{
		redis:  redis,
		config: config,
	}, nil
}

// Upload 上传文件
func (m *Manager) Upload(ctx context.Context, userID int64, fileName string, mimeType string, reader io.Reader) (*FileInfo, error) {
	// 检查 MIME 类型
	if !m.isAllowedType(mimeType) {
		return nil, aiassistant.NewError(aiassistant.ErrCodeInvalidFileType, fmt.Sprintf("不支持的文件类型: %s", mimeType))
	}

	// 生成文件 ID
	fileID := fmt.Sprintf("file_%s", uuid.New().String())

	// 创建临时文件
	tempPath := filepath.Join(m.config.StoragePath, fileID+".tmp")
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempPath)
	defer tempFile.Close()

	// 计算 SHA256 并写入文件
	hash := sha256.New()
	limitReader := io.LimitReader(reader, m.config.MaxSize+1)
	multiWriter := io.MultiWriter(tempFile, hash)

	size, err := io.Copy(multiWriter, limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// 检查文件大小
	if size > m.config.MaxSize {
		return nil, aiassistant.NewError(aiassistant.ErrCodeFileTooLarge, fmt.Sprintf("文件大小超过限制: %d > %d", size, m.config.MaxSize))
	}

	// 计算最终哈希
	sha256Hash := hex.EncodeToString(hash.Sum(nil))

	// 移动到最终位置
	finalPath := filepath.Join(m.config.StoragePath, fileID)
	tempFile.Close()
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// 创建文件信息
	now := time.Now()
	fileInfo := &FileInfo{
		FileID:    fileID,
		FileName:  fileName,
		MimeType:  mimeType,
		Size:      size,
		SHA256:    sha256Hash,
		ExpiresAt: now.Add(m.config.TTL).Unix(),
		CreatedAt: now.Unix(),
		UserID:    userID,
	}

	// 存储文件元数据到 Redis
	if err := m.saveFileInfo(ctx, fileInfo); err != nil {
		os.Remove(finalPath)
		return nil, err
	}

	return fileInfo, nil
}

// GetFileInfo 获取文件信息
func (m *Manager) GetFileInfo(ctx context.Context, fileID string) (*FileInfo, error) {
	key := m.fileInfoKey(fileID)
	data, err := m.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, aiassistant.NewError(aiassistant.ErrCodeFileNotFound, "文件不存在或已过期")
	}

	var fileInfo FileInfo
	if err := json.Unmarshal([]byte(data), &fileInfo); err != nil {
		return nil, fmt.Errorf("failed to parse file info: %w", err)
	}

	return &fileInfo, nil
}

// GenerateDownloadToken 生成下载令牌
func (m *Manager) GenerateDownloadToken(ctx context.Context, fileID string) (*DownloadToken, error) {
	// 验证文件存在
	_, err := m.GetFileInfo(ctx, fileID)
	if err != nil {
		return nil, err
	}

	// 生成令牌
	token := fmt.Sprintf("dl_%s", uuid.New().String())
	expiresAt := time.Now().Add(m.config.TokenTTL).Unix()

	downloadToken := &DownloadToken{
		Token:     token,
		FileID:    fileID,
		ExpiresAt: expiresAt,
	}

	// 存储令牌
	key := m.downloadTokenKey(token)
	data, _ := json.Marshal(downloadToken)
	m.redis.Set(ctx, key, string(data), m.config.TokenTTL)

	return downloadToken, nil
}

// ValidateDownloadToken 验证下载令牌
func (m *Manager) ValidateDownloadToken(ctx context.Context, token string) (*FileInfo, error) {
	key := m.downloadTokenKey(token)
	data, err := m.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, aiassistant.NewError(aiassistant.ErrCodeFileNotFound, "下载链接无效或已过期")
	}

	var downloadToken DownloadToken
	if err := json.Unmarshal([]byte(data), &downloadToken); err != nil {
		return nil, fmt.Errorf("failed to parse download token: %w", err)
	}

	// 检查过期
	if time.Now().Unix() > downloadToken.ExpiresAt {
		m.redis.Del(ctx, key)
		return nil, aiassistant.NewError(aiassistant.ErrCodeFileNotFound, "下载链接已过期")
	}

	// 获取文件信息
	return m.GetFileInfo(ctx, downloadToken.FileID)
}

// GetFilePath 获取文件路径（验证后）
func (m *Manager) GetFilePath(ctx context.Context, fileID string) (string, error) {
	// 验证文件存在
	_, err := m.GetFileInfo(ctx, fileID)
	if err != nil {
		return "", err
	}

	filePath := filepath.Join(m.config.StoragePath, fileID)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", aiassistant.NewError(aiassistant.ErrCodeFileNotFound, "文件不存在")
	}

	return filePath, nil
}

// ValidatePath 验证路径安全性（防止路径穿越）
func (m *Manager) ValidatePath(path string) error {
	// 检查路径穿越
	if strings.Contains(path, "..") {
		return aiassistant.NewError(aiassistant.ErrCodeInvalidRequest, "检测到路径穿越攻击")
	}

	// 检查绝对路径
	if filepath.IsAbs(path) {
		return aiassistant.NewError(aiassistant.ErrCodeInvalidRequest, "不允许使用绝对路径")
	}

	// 检查 Windows 盘符
	if len(path) >= 2 && path[1] == ':' {
		return aiassistant.NewError(aiassistant.ErrCodeInvalidRequest, "不允许使用盘符路径")
	}

	// 清理路径并验证
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") {
		return aiassistant.NewError(aiassistant.ErrCodeInvalidRequest, "检测到路径穿越攻击")
	}

	return nil
}

// DeleteFile 删除文件
func (m *Manager) DeleteFile(ctx context.Context, fileID string) error {
	// 删除文件
	filePath := filepath.Join(m.config.StoragePath, fileID)
	os.Remove(filePath)

	// 删除元数据
	key := m.fileInfoKey(fileID)
	m.redis.Del(ctx, key)

	return nil
}

// CleanupExpiredFiles 清理过期文件
func (m *Manager) CleanupExpiredFiles(ctx context.Context) (int, error) {
	// 扫描存储目录
	entries, err := os.ReadDir(m.config.StoragePath)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileID := entry.Name()
		if strings.HasSuffix(fileID, ".tmp") {
			// 删除临时文件
			os.Remove(filepath.Join(m.config.StoragePath, fileID))
			cleaned++
			continue
		}

		// 检查文件是否过期
		fileInfo, err := m.GetFileInfo(ctx, fileID)
		if err != nil || time.Now().Unix() > fileInfo.ExpiresAt {
			m.DeleteFile(ctx, fileID)
			cleaned++
		}
	}

	return cleaned, nil
}

// isAllowedType 检查是否是允许的类型
func (m *Manager) isAllowedType(mimeType string) bool {
	for _, allowed := range m.config.AllowedTypes {
		if allowed == mimeType {
			return true
		}
	}
	return false
}

// saveFileInfo 保存文件信息
func (m *Manager) saveFileInfo(ctx context.Context, fileInfo *FileInfo) error {
	key := m.fileInfoKey(fileInfo.FileID)
	data, _ := json.Marshal(fileInfo)
	return m.redis.Set(ctx, key, string(data), m.config.TTL).Err()
}

// fileInfoKey 生成文件信息 key
func (m *Manager) fileInfoKey(fileID string) string {
	return fmt.Sprintf("%sfile:%s:info", m.config.RedisPrefix, fileID)
}

// downloadTokenKey 生成下载令牌 key
func (m *Manager) downloadTokenKey(token string) string {
	return fmt.Sprintf("%sdownload_token:%s", m.config.RedisPrefix, token)
}
