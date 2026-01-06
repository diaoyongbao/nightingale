package migrate

import (
	"github.com/ccfos/nightingale/v6/models"
	"github.com/toolkits/pkg/logger"
	"gorm.io/gorm"
)

// MigrateMiddlewareDatasource 中间件数据源表迁移
func MigrateMiddlewareDatasource(db *gorm.DB) error {
	logger.Info("migrate middleware_datasource table")

	// 检查表是否存在
	if db.Migrator().HasTable(&models.MiddlewareDatasource{}) {
		logger.Info("middleware_datasource table already exists, checking for schema updates")

		// 检查并添加缺失的列
		if !db.Migrator().HasColumn(&models.MiddlewareDatasource{}, "health_check_url") {
			logger.Info("adding health_check_url column to middleware_datasource")
			db.Migrator().AddColumn(&models.MiddlewareDatasource{}, "health_check_url")
		}

		if !db.Migrator().HasColumn(&models.MiddlewareDatasource{}, "health_check_interval") {
			logger.Info("adding health_check_interval column to middleware_datasource")
			db.Migrator().AddColumn(&models.MiddlewareDatasource{}, "health_check_interval")
		}

		if !db.Migrator().HasColumn(&models.MiddlewareDatasource{}, "last_health_check") {
			logger.Info("adding last_health_check column to middleware_datasource")
			db.Migrator().AddColumn(&models.MiddlewareDatasource{}, "last_health_check")
		}

		if !db.Migrator().HasColumn(&models.MiddlewareDatasource{}, "health_status") {
			logger.Info("adding health_status column to middleware_datasource")
			db.Migrator().AddColumn(&models.MiddlewareDatasource{}, "health_status")
		}

		if !db.Migrator().HasColumn(&models.MiddlewareDatasource{}, "health_message") {
			logger.Info("adding health_message column to middleware_datasource")
			db.Migrator().AddColumn(&models.MiddlewareDatasource{}, "health_message")
		}

		return nil
	}

	// 创建表
	err := db.AutoMigrate(&models.MiddlewareDatasource{})
	if err != nil {
		logger.Errorf("failed to migrate middleware_datasource table: %v", err)
		return err
	}

	logger.Info("middleware_datasource table created successfully")
	return nil
}
