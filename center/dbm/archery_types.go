package dbm

import "time"

// ArcheryInstance Archery 实例信息
type ArcheryInstance struct {
	ID           int64  `json:"id"`
	InstanceName string `json:"instance_name"`
	Type         string `json:"type"`    // master, slave
	DBType       string `json:"db_type"` // mysql, redis, mongodb等
	Mode         string `json:"mode"`    // standalone, cluster
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Charset      string `json:"charset"`
	CreateTime   string `json:"create_time"`
	UpdateTime   string `json:"update_time"`
}

// ArcheryPaginatedResponse Archery 分页响应格式
type ArcheryPaginatedResponse struct {
	Count    int         `json:"count"`
	Next     *string     `json:"next"`
	Previous *string     `json:"previous"`
	Results  interface{} `json:"results"`
}

// ArcheryInstanceListResponse 实例列表响应 (分页格式)
type ArcheryInstanceListResponse struct {
	Count    int               `json:"count"`
	Next     *string           `json:"next"`
	Previous *string           `json:"previous"`
	Results  []ArcheryInstance `json:"results"`
}

// ArcheryResponse Archery API 响应格式 (用于某些接口)
// ArcheryResponse Archery API 响应格式
type ArcheryResponse struct {
	Status int         `json:"status"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
}

// ArcheryHealthResponse 健康检查响应
type ArcheryHealthResponse struct {
	Status  string    `json:"status"`
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
}
