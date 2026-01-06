package models

import (
	"encoding/json"
	"time"

	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/toolkits/pkg/logger"
)

// MigrateArcheryConfigToDB 从配置文件迁移 Archery 配置到数据库
func MigrateArcheryConfigToDB(ctx *ctx.Context, archeryConf dbm.ArcheryConfig) error {
	if !archeryConf.Enable {
		logger.Info("archery is disabled, skip migration")
		return nil
	}

	// 检查是否已存在
	exists, err := MiddlewareDatasourceExists(ctx, "archery-default")
	if err != nil {
		return err
	}
	if exists {
		logger.Info("archery-default already exists in database, skip migration")
		return nil
	}

	// 构建认证配置
	authConfig := make(map[string]interface{})
	authType := archeryConf.AuthType
	if authType == "" {
		authType = AuthTypeToken // 默认使用 token 认证
	}

	switch authType {
	case AuthTypeToken:
		authConfig["token"] = archeryConf.AuthToken
		authConfig["header_name"] = "Authorization"
		authConfig["header_prefix"] = "Bearer"
	case AuthTypeBasic:
		authConfig["username"] = archeryConf.Username
		authConfig["password"] = archeryConf.Password
	}

	// 构建扩展配置
	settings := make(map[string]interface{})
	settings["api_version"] = "v1"
	settings["features"] = map[string]bool{
		"sql_query":          true,
		"sql_check":          true,
		"slow_query":         true,
		"session_management": true,
	}
	settings["default_db_type"] = "mysql"

	// 创建数据源记录
	ds := &MiddlewareDatasource{
		Name:                "archery-default",
		Type:                MiddlewareTypeArchery,
		Description:         "从配置文件自动迁移的 Archery 实例",
		Address:             archeryConf.Address,
		Status:              MiddlewareStatusEnabled,
		Timeout:             archeryConf.Timeout,
		ConnectTimeout:      archeryConf.ConnectTimeout,
		InsecureSkipVerify:  archeryConf.InsecureSkipVerify,
		AuthType:            authType,
		AuthConfigJson:      authConfig,
		SettingsJson:        settings,
		HealthCheckUrl:      "/api/health/",
		HealthCheckInterval: 60,
		CreatedAt:           time.Now().Unix(),
		CreatedBy:           "system",
		UpdatedAt:           time.Now().Unix(),
		UpdatedBy:           "system",
	}

	err = ds.Add(ctx)
	if err != nil {
		logger.Errorf("failed to migrate archery config to database: %v", err)
		return err
	}

	logger.Info("successfully migrated archery config to database")
	return nil
}

// GetArcheryClientFromDB 从数据库获取 Archery 客户端配置
func GetArcheryClientFromDB(ctx *ctx.Context, name string) (*dbm.ArcheryConfig, error) {
	var mds *MiddlewareDatasource
	var err error

	if name != "" {
		mds, err = MiddlewareDatasourceGetByName(ctx, name)
	} else {
		// 获取第一个启用的 Archery 实例
		list, err := GetEnabledMiddlewareDatasourcesByType(ctx, MiddlewareTypeArchery)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, nil
		}
		mds = list[0]
	}

	if err != nil {
		return nil, err
	}

	if mds == nil {
		return nil, nil
	}

	// 转换为 ArcheryConfig
	config := &dbm.ArcheryConfig{
		Enable:             mds.Status == MiddlewareStatusEnabled,
		Address:            mds.Address,
		AuthType:           mds.AuthType,
		Timeout:            mds.Timeout,
		ConnectTimeout:     mds.ConnectTimeout,
		InsecureSkipVerify: mds.InsecureSkipVerify,
	}

	// 提取认证信息
	switch mds.AuthType {
	case AuthTypeToken:
		config.AuthToken = mds.GetAuthConfigString("token")
	case AuthTypeBasic:
		config.Username = mds.GetAuthConfigString("username")
		config.Password = mds.GetAuthConfigString("password")
	}

	return config, nil
}

// ListArcheryClients 列出所有 Archery 客户端
func ListArcheryClients(ctx *ctx.Context) ([]*dbm.ArcheryConfig, error) {
	list, err := GetEnabledMiddlewareDatasourcesByType(ctx, MiddlewareTypeArchery)
	if err != nil {
		return nil, err
	}

	configs := make([]*dbm.ArcheryConfig, 0, len(list))
	for _, mds := range list {
		config := &dbm.ArcheryConfig{
			Enable:             mds.Status == MiddlewareStatusEnabled,
			Address:            mds.Address,
			AuthType:           mds.AuthType,
			Timeout:            mds.Timeout,
			ConnectTimeout:     mds.ConnectTimeout,
			InsecureSkipVerify: mds.InsecureSkipVerify,
		}

		// 提取认证信息
		switch mds.AuthType {
		case AuthTypeToken:
			config.AuthToken = mds.GetAuthConfigString("token")
		case AuthTypeBasic:
			config.Username = mds.GetAuthConfigString("username")
			config.Password = mds.GetAuthConfigString("password")
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// GetMiddlewareDatasourceAsArcheryConfig 获取中间件数据源并转换为 ArcheryConfig (辅助函数)
func GetMiddlewareDatasourceAsArcheryConfig(ctx *ctx.Context, id int64) (*dbm.ArcheryConfig, error) {
	mds, err := MiddlewareDatasourceGet(ctx, id)
	if err != nil {
		return nil, err
	}

	if mds.Type != MiddlewareTypeArchery {
		return nil, nil
	}

	config := &dbm.ArcheryConfig{
		Enable:             mds.Status == MiddlewareStatusEnabled,
		Address:            mds.Address,
		AuthType:           mds.AuthType,
		Timeout:            mds.Timeout,
		ConnectTimeout:     mds.ConnectTimeout,
		InsecureSkipVerify: mds.InsecureSkipVerify,
	}

	// 提取认证信息
	switch mds.AuthType {
	case AuthTypeToken:
		config.AuthToken = mds.GetAuthConfigString("token")
	case AuthTypeBasic:
		config.Username = mds.GetAuthConfigString("username")
		config.Password = mds.GetAuthConfigString("password")
	}

	return config, nil
}

// ArcheryConfigToMiddlewareDatasource 将 ArcheryConfig 转换为 MiddlewareDatasource
func ArcheryConfigToMiddlewareDatasource(name string, config *dbm.ArcheryConfig, createdBy string) (*MiddlewareDatasource, error) {
	authConfig := make(map[string]interface{})

	switch config.AuthType {
	case AuthTypeToken, "": // 空值默认为 token
		authConfig["token"] = config.AuthToken
		authConfig["header_name"] = "Authorization"
		authConfig["header_prefix"] = "Bearer"
	case AuthTypeBasic:
		authConfig["username"] = config.Username
		authConfig["password"] = config.Password
	}

	authConfigBytes, err := json.Marshal(authConfig)
	if err != nil {
		return nil, err
	}

	settings := make(map[string]interface{})
	settings["api_version"] = "v1"
	settings["features"] = map[string]bool{
		"sql_query":          true,
		"sql_check":          true,
		"slow_query":         true,
		"session_management": true,
	}

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	authType := config.AuthType
	if authType == "" {
		authType = AuthTypeToken
	}

	status := MiddlewareStatusDisabled
	if config.Enable {
		status = MiddlewareStatusEnabled
	}

	now := time.Now().Unix()
	return &MiddlewareDatasource{
		Name:                name,
		Type:                MiddlewareTypeArchery,
		Description:         "Archery SQL 审核平台",
		Address:             config.Address,
		Status:              status,
		Timeout:             config.Timeout,
		ConnectTimeout:      config.ConnectTimeout,
		InsecureSkipVerify:  config.InsecureSkipVerify,
		AuthType:            authType,
		AuthConfig:          string(authConfigBytes),
		Settings:            string(settingsBytes),
		HealthCheckUrl:      "/api/health/",
		HealthCheckInterval: 60,
		CreatedAt:           now,
		CreatedBy:           createdBy,
		UpdatedAt:           now,
		UpdatedBy:           createdBy,
	}, nil
}
