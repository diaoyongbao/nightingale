// Package config 配置加载器
// n9e-2kai: AI 助手模块 - 配置加载器
package config

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// Loader 配置加载器（从数据库读取 + 缓存）
type Loader struct {
	cache      sync.Map // 配置缓存
	lastReload time.Time
	ctx        *ctx.Context
}

// NewLoader 创建配置加载器
func NewLoader(c *ctx.Context) *Loader {
	loader := &Loader{
		ctx: c,
	}

	// 初始加载
	loader.ReloadAll()

	// 启动定期刷新（每 60 秒）
	go loader.startAutoReload()

	return loader
}

// GetAIModelConfig 获取 AI 模型配置
func (l *Loader) GetAIModelConfig(key string) (*AIModelConfig, error) {
	// 先从缓存获取
	if cached, ok := l.cache.Load(key); ok {
		return cached.(*AIModelConfig), nil
	}

	// 从数据库读取
	configValue, err := l.getConfigFromDB(key)
	// 如果出错或配置为空，返回 nil
	if err != nil || configValue == "" {
		return nil, nil
	}

	// 展开环境变量
	configValue = expandEnvVarsInJSON(configValue)

	var config AIModelConfig
	if err := json.Unmarshal([]byte(configValue), &config); err != nil {
		return nil, err
	}

	// 存入缓存
	l.cache.Store(key, &config)

	return &config, nil
}

// GetSessionConfig 获取会话管理配置
func (l *Loader) GetSessionConfig() (*SessionConfig, error) {
	key := models.AIConfigKeySession
	if cached, ok := l.cache.Load(key); ok {
		return cached.(*SessionConfig), nil
	}

	configValue, err := l.getConfigFromDB(key)
	// 如果出错或配置为空，返回默认配置
	if err != nil || configValue == "" {
		return &SessionConfig{
			TTL:                   604800, // 7 天
			MaxMessagesPerSession: 2000,
			MaxSessionsPerUser:    50,
		}, nil
	}

	var config SessionConfig
	if err := json.Unmarshal([]byte(configValue), &config); err != nil {
		// 解析失败也返回默认配置
		return &SessionConfig{
			TTL:                   604800,
			MaxMessagesPerSession: 2000,
			MaxSessionsPerUser:    50,
		}, nil
	}

	l.cache.Store(key, &config)
	return &config, nil
}

// GetKnowledgeConfig 获取知识库配置
func (l *Loader) GetKnowledgeConfig() (*KnowledgeConfig, error) {
	key := models.AIConfigKeyKnowledge
	if cached, ok := l.cache.Load(key); ok {
		return cached.(*KnowledgeConfig), nil
	}

	configValue, err := l.getConfigFromDB(key)
	// 如果出错或配置为空，返回 nil（知识库是可选的）
	if err != nil || configValue == "" {
		return nil, nil
	}

	// 展开环境变量
	configValue = expandEnvVarsInJSON(configValue)

	var config KnowledgeConfig
	if err := json.Unmarshal([]byte(configValue), &config); err != nil {
		return nil, err
	}

	l.cache.Store(key, &config)
	return &config, nil
}

// ReloadAll 重新加载所有配置
func (l *Loader) ReloadAll() error {
	// 清空缓存
	l.cache.Range(func(key, value interface{}) bool {
		l.cache.Delete(key)
		return true
	})

	l.lastReload = time.Now()
	return nil
}

// getConfigFromDB 从数据库读取配置
func (l *Loader) getConfigFromDB(key string) (string, error) {
	config, err := models.AIConfigGetByKey(l.ctx, key)
	if err != nil {
		return "", err
	}
	if config == nil {
		return "", nil
	}

	return config.ConfigValue, nil
}

// startAutoReload 自动刷新配置（每 60 秒）
func (l *Loader) startAutoReload() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 检查数据库中的配置是否有更新
		configs, err := models.AIConfigGets(l.ctx, "enabled = 1")
		if err != nil {
			logger.Errorf("failed to check AI config updates: %v", err)
			continue
		}

		var latestUpdate int64
		for _, c := range configs {
			if c.UpdateAt > latestUpdate {
				latestUpdate = c.UpdateAt
			}
		}

		if latestUpdate > l.lastReload.Unix() {
			l.ReloadAll()
			logger.Infof("AI config reloaded, latest update: %d", latestUpdate)
		}
	}
}

// expandEnvVar 环境变量展开（支持 ${VAR_NAME}）
func expandEnvVar(s string) string {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		envName := s[2 : len(s)-1]
		if envValue := os.Getenv(envName); envValue != "" {
			return envValue
		}
	}
	return s
}

// expandEnvVarsInJSON 展开JSON字符串中的环境变量
func expandEnvVarsInJSON(jsonStr string) string {
	// 使用正则表达式匹配 ${VAR_NAME} 模式
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		envName := match[2 : len(match)-1] // 去掉 ${ 和 }
		if envValue := os.Getenv(envName); envValue != "" {
			return envValue
		}
		return match // 如果环境变量不存在，保持原样
	})
}
// GetFileConfig 获取文件管理配置
func (l *Loader) GetFileConfig() (*FileConfig, error) {
	key := models.AIConfigKeyFile
	if cached, ok := l.cache.Load(key); ok {
		return cached.(*FileConfig), nil
	}

	// 默认配置
	defaultConfig := &FileConfig{
		MaxSize:        104857600, // 100MB
		StorageBackend: "local",
		StoragePath:    "/tmp/ai_assistant_files",
		TTL:            3600, // 1小时
	}

	configValue, err := l.getConfigFromDB(key)
	// 如果出错或配置为空，返回默认配置
	if err != nil || configValue == "" {
		return defaultConfig, nil
	}

	// 展开环境变量
	configValue = expandEnvVarsInJSON(configValue)

	var config FileConfig
	if err := json.Unmarshal([]byte(configValue), &config); err != nil {
		// 解析失败返回默认配置
		return defaultConfig, nil
	}

	l.cache.Store(key, &config)
	return &config, nil
}