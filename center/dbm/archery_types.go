package dbm

import "time"

// ArcheryInstance Archery 实例信息
type ArcheryInstance struct {
	ID           int64  `json:"id"`
	InstanceName string `json:"instance_name"`
	Type         string `json:"type"`    // mysql, redis, mongodb等
	DBType       string `json:"db_type"` // master, slave
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Charset      string `json:"charset"`
	CreateTime   string `json:"create_time"`
	UpdateTime   string `json:"update_time"`
}

// ArcheryResponse Archery API 响应格式
type ArcheryResponse struct {
	Status int         `json:"status"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
}

// ArcheryInstanceListResponse 实例列表响应
type ArcheryInstanceListResponse struct {
	Status int               `json:"status"`
	Msg    string            `json:"msg"`
	Data   []ArcheryInstance `json:"data"`
}

// ArcheryHealthResponse 健康检查响应
type ArcheryHealthResponse struct {
	Status  string    `json:"status"`
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
}
