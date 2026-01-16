// n9e-2kai: 华为云区域配置
package huawei

// Region 华为云区域信息(内部使用)
type Region struct {
	Region   string
	Name     string
	Endpoint string
}

// 华为云区域列表
var huaweiRegions = []Region{
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

// GetAvailableRegions 获取可用区域列表
func GetAvailableRegions() []Region {
	return huaweiRegions
}

// GetRegionByCode 根据区域代码获取区域信息
func GetRegionByCode(code string) *Region {
	for _, r := range huaweiRegions {
		if r.Region == code {
			return &r
		}
	}
	return nil
}
