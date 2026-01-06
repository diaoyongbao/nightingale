package router

import (
	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/models"
)

// createArcheryClient 从数据库配置创建 Archery 客户端
func (rt *Router) createArcheryClient(config *dbm.ArcheryConfig) (*dbm.ArcheryClient, error) {
	return dbm.NewArcheryClient(config)
}

// getArcheryClientFromDB 从数据库获取 Archery 客户端
func (rt *Router) getArcheryClientFromDB(name string) (*dbm.ArcheryClient, error) {
	config, err := models.GetArcheryClientFromDB(rt.Ctx, name)
	if err != nil {
		return nil, err
	}

	if config == nil {
		// 如果数据库中没有,尝试从配置文件读取
		config = rt.getArcheryConfigFromEnv()
		if config == nil {
			return nil, nil
		}
	}

	return rt.createArcheryClient(config)
}

// getArcheryConfigFromEnv 从环境配置获取 Archery 配置
func (rt *Router) getArcheryConfigFromEnv() *dbm.ArcheryConfig {
	// 这里需要从全局配置中获取 Archery 配置
	// 由于当前 Router 结构中没有直接的配置引用,返回 nil
	// 在实际集成时需要添加配置引用
	return nil
}

// initArcheryClientFromDB 初始化 Archery 客户端 (从数据库)
func (rt *Router) initArcheryClientFromDB() {
	// 获取默认的 Archery 实例
	client, err := rt.getArcheryClientFromDB("")
	if err != nil {
		return
	}

	if client != nil {
		rt.ArcheryClient = client
	}
}
