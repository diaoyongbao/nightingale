// n9e-2kai: 云服务管理模块 - 接口定义
package cloudmgmt

import (
	"context"

	"github.com/ccfos/nightingale/v6/models"
)

// CloudProvider 云服务提供者接口
type CloudProvider interface {
	// GetName 获取云厂商标识
	GetName() string

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error

	// GetRegions 获取区域列表
	GetRegions(ctx context.Context) ([]Region, error)

	// ECS 相关
	ListECS(ctx context.Context, region string, opts *ListOptions) ([]models.CloudECS, error)

	// RDS 相关
	ListRDS(ctx context.Context, region string, opts *ListOptions) ([]models.CloudRDS, error)
}

// Region 区域信息
type Region struct {
	Region   string `json:"region"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

// ListOptions 列表查询选项
type ListOptions struct {
	Limit  int32
	Marker string // 分页标记
}

// SyncResult 同步结果
type SyncResult struct {
	Total   int
	Added   int
	Updated int
	Deleted int
	Errors  []error
}

// ProviderInfo 云厂商信息
type ProviderInfo struct {
	Key                string   `json:"key"`
	Name               string   `json:"name"`
	Enabled            bool     `json:"enabled"`
	SupportedResources []string `json:"supported_resources"`
}

// GetHuaweiRegions 获取华为云区域列表
func GetHuaweiRegions() []Region {
	return []Region{
		// 中国大陆
		{Region: "cn-north-1", Name: "华北-北京一", Endpoint: "ecs.cn-north-1.myhuaweicloud.com"},
		{Region: "cn-north-4", Name: "华北-北京四", Endpoint: "ecs.cn-north-4.myhuaweicloud.com"},
		{Region: "cn-north-9", Name: "华北-乌兰察布一", Endpoint: "ecs.cn-north-9.myhuaweicloud.com"},
		{Region: "cn-east-2", Name: "华东-上海二", Endpoint: "ecs.cn-east-2.myhuaweicloud.com"},
		{Region: "cn-east-3", Name: "华东-上海一", Endpoint: "ecs.cn-east-3.myhuaweicloud.com"},
		{Region: "cn-south-1", Name: "华南-广州", Endpoint: "ecs.cn-south-1.myhuaweicloud.com"},
		{Region: "cn-south-2", Name: "华南-深圳", Endpoint: "ecs.cn-south-2.myhuaweicloud.com"},
		{Region: "cn-southwest-2", Name: "西南-贵阳一", Endpoint: "ecs.cn-southwest-2.myhuaweicloud.com"},

		// 中国香港/亚太
		{Region: "ap-southeast-1", Name: "中国-香港", Endpoint: "ecs.ap-southeast-1.myhuaweicloud.com"},
		{Region: "ap-southeast-2", Name: "亚太-曼谷", Endpoint: "ecs.ap-southeast-2.myhuaweicloud.com"},
		{Region: "ap-southeast-3", Name: "亚太-新加坡", Endpoint: "ecs.ap-southeast-3.myhuaweicloud.com"},

		// 其他国际区域
		{Region: "af-south-1", Name: "非洲-约翰内斯堡", Endpoint: "ecs.af-south-1.myhuaweicloud.com"},
		{Region: "la-north-2", Name: "拉美-墨西哥城一", Endpoint: "ecs.la-north-2.myhuaweicloud.com"},
		{Region: "la-south-2", Name: "拉美-圣地亚哥", Endpoint: "ecs.la-south-2.myhuaweicloud.com"},
		{Region: "na-mexico-1", Name: "拉美-墨西哥城二", Endpoint: "ecs.na-mexico-1.myhuaweicloud.com"},
		{Region: "sa-brazil-1", Name: "拉美-圣保罗一", Endpoint: "ecs.sa-brazil-1.myhuaweicloud.com"},
	}
}

// GetSupportedProviders 获取支持的云厂商列表
func GetSupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Key:                "huawei",
			Name:               "华为云",
			Enabled:            true,
			SupportedResources: []string{"ecs", "rds"},
		},
		{
			Key:                "aliyun",
			Name:               "阿里云",
			Enabled:            false,
			SupportedResources: []string{},
		},
		{
			Key:                "tencent",
			Name:               "腾讯云",
			Enabled:            false,
			SupportedResources: []string{},
		},
		{
			Key:                "volcengine",
			Name:               "火山云",
			Enabled:            false,
			SupportedResources: []string{},
		},
	}
}
