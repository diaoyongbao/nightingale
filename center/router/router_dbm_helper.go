package router

import (
	"github.com/ccfos/nightingale/v6/center/dbm"
	"github.com/ccfos/nightingale/v6/models"
)

// getDBClient 辅助函数：从实例创建数据库客户端
func getDBClient(instance *models.DBInstance) (*dbm.MySQLClient, error) {
	dsn, err := instance.GetDSN("")
	if err != nil {
		return nil, err
	}

	return dbm.NewMySQLClient(instance.Id, dsn, instance.MaxConnections, instance.MaxIdleConns)
}
