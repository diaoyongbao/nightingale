// n9e-2kai: 云服务管理模块 - 模块初始化
package cloudmgmt

import (
	"context"

	"github.com/ccfos/nightingale/v6/center/cloudmgmt/huawei"
	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

var (
	// DefaultManager 默认管理器实例
	DefaultManager *Manager
)

// HuaweiAdapter 华为云适配器，实现 CloudProvider 接口
type HuaweiAdapter struct {
	client *huawei.HuaweiClient
}

// NewHuaweiAdapter 创建华为云适配器
func NewHuaweiAdapter(ak, sk string, regions []string) CloudProvider {
	return &HuaweiAdapter{
		client: huawei.NewHuaweiClient(ak, sk, regions),
	}
}

func (a *HuaweiAdapter) GetName() string {
	return a.client.GetName()
}

func (a *HuaweiAdapter) TestConnection(ctx context.Context) error {
	return a.client.TestConnection(ctx)
}

func (a *HuaweiAdapter) GetRegions(ctx context.Context) ([]Region, error) {
	internalRegions := a.client.GetRegionsInternal()
	result := make([]Region, len(internalRegions))
	for i, r := range internalRegions {
		result[i] = Region{
			Region:   r.Region,
			Name:     r.Name,
			Endpoint: r.Endpoint,
		}
	}
	return result, nil
}

func (a *HuaweiAdapter) ListECS(ctx context.Context, region string, opts *ListOptions) ([]models.CloudECS, error) {
	return a.client.ListECSInternal(ctx, region)
}

func (a *HuaweiAdapter) ListRDS(ctx context.Context, region string, opts *ListOptions) ([]models.CloudRDS, error) {
	return a.client.ListRDSInternal(ctx, region)
}

// ListSlowLogStatistics 获取 RDS 慢日志统计
func (a *HuaweiAdapter) ListSlowLogStatistics(ctx context.Context, region string, instanceId string, startTime, endTime string, database string, sqlType string, limit, offset int32) ([]huawei.SlowLogStatistics, int32, error) {
	return a.client.ListSlowLogStatistics(ctx, region, instanceId, startTime, endTime, database, sqlType, limit, offset)
}

// ListSlowLogDetails 获取 RDS 慢日志明细
func (a *HuaweiAdapter) ListSlowLogDetails(ctx context.Context, region string, instanceId string, startTime, endTime string, database string, sqlType string, limit int32, lineNum string) ([]huawei.SlowLogDetail, string, error) {
	return a.client.ListSlowLogDetails(ctx, region, instanceId, startTime, endTime, database, sqlType, limit, lineNum)
}

// Init 初始化云服务管理模块
func Init(c *ctx.Context) {
	DefaultManager = NewManager(c)

	// 注册华为云提供者
	DefaultManager.RegisterProvider("huawei", func(ak, sk string, regions []string) CloudProvider {
		return NewHuaweiAdapter(ak, sk, regions)
	})

	// n9e-2kai: 服务启动时重置所有"同步中"状态的配置
	if affected, err := models.CloudSyncConfigResetRunningStatus(c); err != nil {
		// 使用标准库 log，因为这时 logger 可能还没初始化
		println("reset running sync configs failed:", err.Error())
	} else if affected > 0 {
		println("reset", affected, "running sync configs to failed status")
	}

	// n9e-2kai: 服务启动时重置所有"同步中"状态的同步日志
	if affected, err := models.CloudSyncLogResetRunningStatus(c); err != nil {
		println("reset running sync logs failed:", err.Error())
	} else if affected > 0 {
		println("reset", affected, "running sync logs to failed status")
	}

	// n9e-2kai: 启动慢日志定时同步任务
	DefaultManager.StartSlowLogSyncTask()
}

// GetManager 获取管理器实例
func GetManager() *Manager {
	return DefaultManager
}
