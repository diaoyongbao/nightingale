# 夜莺监控API接口文档

## 1. API概览

### 1.1 API前缀

- **页面API**: `/api/n9e/*`
- **服务API**: `/v1/n9e/*` (需要BasicAuth)
- **Agent API**: `/v1/n9e/heartbeat`

### 1.2 认证方式

#### 1.2.1 JWT Token (页面API)

```http
Authorization: Bearer <token>
```

获取Token:
```http
POST /api/n9e/auth/login
Content-Type: application/json

{
    "username": "root",
    "password": "root.2020"
}

Response:
{
    "dat": {
        "user": {...},
        "access_token": "eyJhbGc...",
        "refresh_token": "eyJhbGc..."
    },
    "error": ""
}
```

#### 1.2.2 Basic Auth (服务API)

```http
Authorization: Basic <base64(username:password)>
```

### 1.3 响应格式

```json
{
    "dat": <数据>,
    "error": "<错误信息>"
}
```

成功时 `error` 为空字符串,失败时 `error` 包含错误信息。

## 2. 用户管理API

### 2.1 获取用户列表

```http
GET /api/n9e/users?query=<搜索关键词>&limit=<每页数量>&offset=<偏移量>
```

**Response**:
```json
{
    "dat": {
        "list": [
            {
                "id": 1,
                "username": "root",
                "nickname": "超级管理员",
                "email": "root@example.com",
                "phone": "13800138000",
                "roles": "Admin",
                "create_at": 1234567890,
                "update_at": 1234567890
            }
        ],
        "total": 100
    },
    "error": ""
}
```

### 2.2 创建用户

```http
POST /api/n9e/users
Content-Type: application/json

{
    "username": "user1",
    "password": "password123",
    "nickname": "用户1",
    "email": "user1@example.com",
    "phone": "13800138001",
    "roles": "Standard"
}
```

### 2.3 更新用户

```http
PUT /api/n9e/user/:id/profile
Content-Type: application/json

{
    "nickname": "新昵称",
    "email": "newemail@example.com",
    "phone": "13800138002"
}
```

### 2.4 删除用户

```http
DELETE /api/n9e/user/:id
```

### 2.5 修改密码

```http
PUT /api/n9e/self/password
Content-Type: application/json

{
    "oldpass": "old_password",
    "newpass": "new_password"
}
```

## 3. 业务组API

### 3.1 获取业务组列表

```http
GET /api/n9e/busi-groups?query=<搜索关键词>
```

**Response**:
```json
{
    "dat": [
        {
            "id": 1,
            "name": "默认业务组",
            "label_enable": 0,
            "label_value": "",
            "create_at": 1234567890,
            "update_at": 1234567890,
            "perm_flag": "rw"
        }
    ],
    "error": ""
}
```

### 3.2 创建业务组

```http
POST /api/n9e/busi-groups
Content-Type: application/json

{
    "name": "新业务组",
    "label_enable": 0,
    "label_value": ""
}
```

### 3.3 更新业务组

```http
PUT /api/n9e/busi-group/:id
Content-Type: application/json

{
    "name": "更新后的名称",
    "label_enable": 1,
    "label_value": "app=myapp"
}
```

### 3.4 删除业务组

```http
DELETE /api/n9e/busi-group/:id
```

### 3.5 添加成员

```http
POST /api/n9e/busi-group/:id/members
Content-Type: application/json

{
    "user_group_ids": [1, 2, 3],
    "perm_flag": "rw"
}
```

## 4. 告警规则API

### 4.1 获取告警规则列表

```http
GET /api/n9e/busi-group/:id/alert-rules?query=<搜索关键词>&disabled=<0|1>
```

**Response**:
```json
{
    "dat": [
        {
            "id": 1,
            "group_id": 1,
            "name": "CPU使用率过高",
            "cate": "prometheus",
            "datasource_ids": [1],
            "prom_ql": "avg(cpu_usage_idle) < 20",
            "severity": 2,
            "disabled": 0,
            "prom_eval_interval": 15,
            "notify_recovered": 1,
            "notify_rule_ids": [1],
            "create_at": 1234567890,
            "update_at": 1234567890
        }
    ],
    "error": ""
}
```

### 4.2 创建告警规则

```http
POST /api/n9e/busi-group/:id/alert-rules
Content-Type: application/json

{
    "name": "CPU使用率过高",
    "cate": "prometheus",
    "datasource_queries": [
        {
            "match_type": 0,
            "op": "in",
            "values": [1]
        }
    ],
    "prom_ql": "avg(cpu_usage_idle) < 20",
    "prom_eval_interval": 15,
    "prom_for_duration": 60,
    "severity": 2,
    "disabled": 0,
    "notify_recovered": 1,
    "notify_rule_ids": [1],
    "notify_repeat_step": 60,
    "recover_duration": 0,
    "rule_config": {
        "queries": [
            {
                "prom_ql": "avg(cpu_usage_idle) < 20",
                "severity": 2
            }
        ],
        "triggers": [
            {
                "severity": 2,
                "exp": "t1 < 20"
            }
        ]
    }
}
```

### 4.3 更新告警规则

```http
PUT /api/n9e/busi-group/:id/alert-rule/:arid
Content-Type: application/json

{
    "name": "更新后的规则名称",
    "disabled": 0,
    ...
}
```

### 4.4 删除告警规则

```http
DELETE /api/n9e/busi-group/:id/alert-rules
Content-Type: application/json

{
    "ids": [1, 2, 3]
}
```

### 4.5 批量更新字段

```http
PUT /api/n9e/busi-group/:id/alert-rules/fields
Content-Type: application/json

{
    "ids": [1, 2, 3],
    "fields": {
        "disabled": 1,
        "notify_recovered": 0
    }
}
```

## 5. 告警事件API

### 5.1 获取当前告警事件

```http
GET /api/n9e/alert-cur-events/list?severity=<级别>&group_ids=<业务组ID>&query=<搜索>
```

**Response**:
```json
{
    "dat": {
        "list": [
            {
                "id": 1,
                "rule_id": 1,
                "rule_name": "CPU使用率过高",
                "severity": 2,
                "status": 0,
                "target_ident": "host-1",
                "trigger_time": 1234567890,
                "trigger_value": "15.5",
                "tags": "region=cn-north,env=prod",
                "notify_cur_number": 2
            }
        ],
        "total": 100
    },
    "error": ""
}
```

### 5.2 获取历史告警事件

```http
GET /api/n9e/alert-his-events/list?stime=<开始时间>&etime=<结束时间>&severity=<级别>
```

### 5.3 获取事件详情

```http
GET /api/n9e/alert-cur-event/:eid
GET /api/n9e/alert-his-event/:eid
```

**Response**:
```json
{
    "dat": {
        "id": 1,
        "rule_id": 1,
        "rule_name": "CPU使用率过高",
        "rule_note": "监控CPU使用率",
        "severity": 2,
        "status": 0,
        "target_ident": "host-1",
        "target_note": "生产服务器1",
        "trigger_time": 1234567890,
        "trigger_value": "15.5",
        "tags": "region=cn-north,env=prod",
        "annotations": {
            "summary": "CPU使用率过高",
            "description": "当前CPU使用率: 15.5%"
        }
    },
    "error": ""
}
```

### 5.4 删除当前告警事件

```http
DELETE /api/n9e/alert-cur-events
Content-Type: application/json

{
    "ids": [1, 2, 3]
}
```

### 5.5 认领告警事件

```http
PUT /api/n9e/alert-cur-event/:eid/claim
```

## 6. 数据源API

### 6.1 获取数据源列表

```http
POST /api/n9e/datasource/list
Content-Type: application/json

{
    "query": "",
    "plugin_type": "prometheus"
}
```

**Response**:
```json
{
    "dat": {
        "list": [
            {
                "id": 1,
                "name": "Prometheus-1",
                "plugin_type": "prometheus",
                "settings": {...},
                "is_default": true,
                "status": "enabled"
            }
        ],
        "total": 10
    },
    "error": ""
}
```

### 6.2 创建数据源

```http
POST /api/n9e/datasource/upsert
Content-Type: application/json

{
    "name": "Prometheus-1",
    "plugin_type": "prometheus",
    "settings": {
        "url": "http://localhost:9090",
        "timeout": 30
    },
    "is_default": true
}
```

### 6.3 更新数据源

```http
POST /api/n9e/datasource/upsert
Content-Type: application/json

{
    "id": 1,
    "name": "Prometheus-1",
    "settings": {...}
}
```

### 6.4 删除数据源

```http
DELETE /api/n9e/datasource/?ids=1,2,3
```

### 6.5 获取数据源简要信息

```http
GET /api/n9e/datasource/brief
```

**Response**:
```json
{
    "dat": [
        {
            "id": 1,
            "name": "Prometheus-1",
            "plugin_type": "prometheus",
            "is_default": true
        }
    ],
    "error": ""
}
```

## 7. 数据查询API

### 7.1 PromQL查询

```http
POST /api/n9e/query-range-batch
Content-Type: application/json

{
    "queries": [
        {
            "ref_id": "A",
            "datasource_id": 1,
            "prom_ql": "up",
            "start": 1234567890,
            "end": 1234567900,
            "step": 15
        }
    ]
}
```

**Response**:
```json
{
    "dat": [
        {
            "ref_id": "A",
            "series": [
                {
                    "metric": {
                        "__name__": "up",
                        "instance": "localhost:9090",
                        "job": "prometheus"
                    },
                    "values": [
                        [1234567890, "1"],
                        [1234567905, "1"]
                    ]
                }
            ]
        }
    ],
    "error": ""
}
```

### 7.2 即时查询

```http
POST /api/n9e/query-instant-batch
Content-Type: application/json

{
    "queries": [
        {
            "ref_id": "A",
            "datasource_id": 1,
            "prom_ql": "up",
            "time": 1234567890
        }
    ]
}
```

### 7.3 日志查询

```http
POST /api/n9e/log-query
Content-Type: application/json

{
    "datasource_id": 2,
    "index": "logs-*",
    "query": "level:error",
    "start": 1234567890000,
    "end": 1234567900000,
    "limit": 100
}
```

## 8. 仪表盘API

### 8.1 获取仪表盘列表

```http
GET /api/n9e/busi-group/:id/boards?query=<搜索关键词>
```

**Response**:
```json
{
    "dat": [
        {
            "id": 1,
            "group_id": 1,
            "name": "系统监控",
            "tags": "system,monitoring",
            "create_at": 1234567890,
            "update_at": 1234567890,
            "public": 0
        }
    ],
    "error": ""
}
```

### 8.2 创建仪表盘

```http
POST /api/n9e/busi-group/:id/boards
Content-Type: application/json

{
    "name": "系统监控",
    "tags": "system,monitoring",
    "configs": {
        "var": [],
        "panels": []
    }
}
```

### 8.3 获取仪表盘详情

```http
GET /api/n9e/board/:bid
```

**Response**:
```json
{
    "dat": {
        "id": 1,
        "name": "系统监控",
        "configs": {
            "var": [
                {
                    "name": "host",
                    "type": "query",
                    "query": "label_values(up, instance)"
                }
            ],
            "panels": [
                {
                    "id": 1,
                    "type": "timeseries",
                    "title": "CPU使用率",
                    "targets": [
                        {
                            "ref_id": "A",
                            "expr": "100 - avg(cpu_usage_idle{instance=~\"$host\"})"
                        }
                    ]
                }
            ]
        }
    },
    "error": ""
}
```

### 8.4 更新仪表盘

```http
PUT /api/n9e/board/:bid
Content-Type: application/json

{
    "name": "更新后的名称",
    "configs": {...}
}
```

### 8.5 删除仪表盘

```http
DELETE /api/n9e/boards
Content-Type: application/json

{
    "ids": [1, 2, 3]
}
```

## 9. 监控对象API

### 9.1 获取监控对象列表

```http
GET /api/n9e/targets?query=<搜索>&group_ids=<业务组ID>
```

**Response**:
```json
{
    "dat": {
        "list": [
            {
                "id": 1,
                "ident": "host-1",
                "note": "生产服务器1",
                "tags": "region=cn-north,env=prod",
                "update_at": 1234567890,
                "group_ids": [1, 2]
            }
        ],
        "total": 100
    },
    "error": ""
}
```

### 9.2 删除监控对象

```http
DELETE /api/n9e/targets
Content-Type: application/json

{
    "ids": [1, 2, 3]
}
```

### 9.3 绑定标签

```http
POST /api/n9e/targets/tags
Content-Type: application/json

{
    "ids": [1, 2, 3],
    "tags": ["env=prod", "region=cn-north"]
}
```

### 9.4 解绑标签

```http
DELETE /api/n9e/targets/tags
Content-Type: application/json

{
    "ids": [1, 2, 3],
    "tags": ["env=prod"]
}
```

### 9.5 更新备注

```http
PUT /api/n9e/targets/note
Content-Type: application/json

{
    "ids": [1, 2, 3],
    "note": "新的备注"
}
```

### 9.6 绑定业务组

```http
PUT /api/n9e/targets/bgids
Content-Type: application/json

{
    "ids": [1, 2, 3],
    "bgids": [1, 2]
}
```

## 10. 通知管理API

### 10.1 获取通知规则列表

```http
GET /api/n9e/notify-rules?query=<搜索>
```

**Response**:
```json
{
    "dat": [
        {
            "id": 1,
            "name": "默认通知规则",
            "disabled": 0,
            "user_group_ids": [1, 2],
            "channel_ids": [1, 2],
            "contact_keys": ["phone", "email"],
            "match_conditions": {
                "severity": [1, 2],
                "tags": ["env=prod"]
            },
            "for_duration": 0
        }
    ],
    "error": ""
}
```

### 10.2 创建通知规则

```http
POST /api/n9e/notify-rules
Content-Type: application/json

{
    "name": "生产环境通知",
    "disabled": 0,
    "user_group_ids": [1],
    "channel_ids": [1, 2],
    "contact_keys": ["phone", "email"],
    "match_conditions": {
        "severity": [1, 2],
        "tags": ["env=prod"]
    },
    "for_duration": 300
}
```

### 10.3 更新通知规则

```http
PUT /api/n9e/notify-rule/:id
Content-Type: application/json

{
    "name": "更新后的名称",
    ...
}
```

### 10.4 删除通知规则

```http
DELETE /api/n9e/notify-rules
Content-Type: application/json

{
    "ids": [1, 2, 3]
}
```

### 10.5 获取通知渠道列表

```http
GET /api/n9e/notify-channel-configs
```

**Response**:
```json
{
    "dat": [
        {
            "id": 1,
            "name": "钉钉机器人",
            "ident": "dingtalk-robot-1",
            "built_in": 0,
            "extra_config": {
                "webhook": "https://oapi.dingtalk.com/robot/send?access_token=xxx"
            }
        }
    ],
    "error": ""
}
```

### 10.6 创建通知渠道

```http
POST /api/n9e/notify-channel-configs
Content-Type: application/json

{
    "name": "钉钉机器人",
    "ident": "dingtalk-robot-1",
    "extra_config": {
        "webhook": "https://oapi.dingtalk.com/robot/send?access_token=xxx"
    }
}
```

## 11. 服务API (BasicAuth)

### 11.1 心跳上报

```http
POST /v1/n9e/heartbeat
Authorization: Basic <credentials>
Content-Type: application/json

{
    "ident": "host-1",
    "clock": 1234567890
}
```

### 11.2 获取告警规则

```http
GET /v1/n9e/alert-rules?gids=<业务组ID>
Authorization: Basic <credentials>
```

### 11.3 获取数据源

```http
GET /v1/n9e/datasources
Authorization: Basic <credentials>
```

### 11.4 添加告警事件

```http
POST /v1/n9e/alert-cur-events
Authorization: Basic <credentials>
Content-Type: application/json

{
    "rule_id": 1,
    "trigger_time": 1234567890,
    "trigger_value": "15.5",
    ...
}
```

## 12. 错误码

| 错误码 | 说明 |
|--------|------|
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 500 | 服务器错误 |

## 13. 限流

- 页面API: 无限流
- 服务API: 根据配置限流
- Agent API: 根据配置限流

## 14. 最佳实践

### 14.1 批量操作

使用批量接口而不是循环调用单个接口:

```javascript
// 好的做法
await deleteAlertRules({ ids: [1, 2, 3] });

// 不好的做法
for (const id of [1, 2, 3]) {
    await deleteAlertRule(id);
}
```

### 14.2 分页查询

对于大量数据,使用分页查询:

```javascript
const limit = 100;
let offset = 0;
let hasMore = true;

while (hasMore) {
    const res = await getAlertRules({ limit, offset });
    // 处理数据
    hasMore = res.dat.length === limit;
    offset += limit;
}
```

### 14.3 错误处理

```javascript
try {
    const res = await createAlertRule(data);
    if (res.error) {
        message.error(res.error);
    } else {
        message.success('创建成功');
    }
} catch (error) {
    message.error('网络错误');
}
```

### 14.4 Token刷新

```javascript
// 使用refresh_token刷新access_token
const refreshToken = async () => {
    const res = await fetch('/api/n9e/auth/refresh', {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${refresh_token}`
        }
    });
    const data = await res.json();
    localStorage.setItem('access_token', data.dat.access_token);
};
```
