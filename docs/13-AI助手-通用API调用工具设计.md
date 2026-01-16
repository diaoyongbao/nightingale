# AI助手 - 通用 API 调用工具设计

> **文档目的**: 设计一套让 AI 自主决定调用什么 API、如何调用的通用工具框架，实现系统功能的 Agent 化。

---

## 1. 设计理念

### 1.1 核心问题

传统的 Agent 工具设计需要为每个功能硬编码一个工具函数，存在以下问题：

| 问题 | 影响 |
|------|------|
| 每新增功能需要写代码 | 扩展慢，需要开发-编译-部署 |
| 工具数量爆炸 | 系统有 200+ API，难以全部映射 |
| 维护成本高 | API 变更需同步修改工具代码 |
| AI 上下文浪费 | 大量工具描述占用 Token |

### 1.2 设计目标

```
用户说: "帮我屏蔽 host-01 的 CPU 告警 1 小时"

AI 自动完成:
1. 理解意图 -> 需要创建告警屏蔽
2. 发现工具 -> 找到 alert_mute_create 工具
3. 提取参数 -> { targets: ["host-01"], duration: 3600, ... }
4. 执行调用 -> POST /api/n9e/busi-group/:id/alert-mutes
5. 返回结果 -> "已成功创建屏蔽规则，ID: 123"
```

### 1.3 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                      AI Agent (LLM)                         │
│  "用户想屏蔽告警" -> 选择工具 -> 生成参数 -> 解释结果        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Tool Router (工具路由)                    │
│  1. 工具发现: 根据意图从 ai_tool 表检索候选工具              │
│  2. 工具选择: LLM 从候选中选择最合适的工具                   │
│  3. 参数映射: 将自然语言转换为 API 参数                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Generic API Executor (通用执行器)               │
│  1. 读取工具配置 (method, url, schema)                       │
│  2. 构建 HTTP 请求 (注入 Auth, 替换路径参数)                 │
│  3. 执行请求并处理响应                                       │
│  4. 格式化结果返回给 LLM                                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Nightingale API Layer                      │
│  /api/n9e/alert-mutes, /api/n9e/targets, ...                │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 工具定义规范 (Tool Definition Schema)

### 2.1 数据库表设计

```sql
CREATE TABLE `ai_tool` (
    `id` bigint PRIMARY KEY AUTO_INCREMENT,
    `name` varchar(128) NOT NULL UNIQUE COMMENT '工具唯一标识，如: alert_mute_create',
    `display_name` varchar(128) NOT NULL COMMENT '显示名称，如: 创建告警屏蔽',
    `description` text NOT NULL COMMENT 'LLM 用于理解何时调用的描述',
    `category` varchar(64) NOT NULL COMMENT '分类: alert, target, dashboard, user, system',
    
    -- 工具类型
    `type` varchar(32) NOT NULL DEFAULT 'api' COMMENT 'api=内部API, mcp=外部MCP, script=脚本',
    
    -- API 配置 (type='api' 时使用)
    `method` varchar(10) COMMENT 'HTTP 方法: GET, POST, PUT, DELETE',
    `url_pattern` varchar(256) COMMENT 'URL 模板，支持占位符: /api/n9e/busi-group/{group_id}/alert-mutes',
    `request_body_schema` text COMMENT 'JSON Schema: 请求体参数定义',
    `query_params_schema` text COMMENT 'JSON Schema: 查询参数定义',
    `path_params_schema` text COMMENT 'JSON Schema: 路径参数定义',
    `response_template` text COMMENT '响应格式化模板 (可选)',
    
    -- MCP 配置 (type='mcp' 时使用)
    `mcp_server_id` bigint COMMENT '关联 mcp_server 表',
    `mcp_tool_name` varchar(128) COMMENT 'MCP Server 暴露的工具名',
    
    -- 权限与状态
    `required_permission` varchar(128) COMMENT '所需权限点，如: /alert-mutes/add',
    `risk_level` tinyint DEFAULT 1 COMMENT '风险等级: 1=低(查询), 2=中(修改), 3=高(删除)',
    `enabled` tinyint(1) DEFAULT 1,
    `keywords` text COMMENT 'JSON 数组: 触发关键词 ["屏蔽", "mute", "静默"]',
    
    -- 示例与文档
    `example_prompt` text COMMENT '示例用户输入',
    `example_params` text COMMENT '示例参数 JSON',
    
    `created_at` bigint,
    `updated_at` bigint,
    
    INDEX `idx_category` (`category`),
    INDEX `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 工具定义表';
```

### 2.2 JSON Schema 规范

工具参数使用标准 JSON Schema 定义，LLM 可以理解并生成符合规范的参数：

```json
{
  "type": "object",
  "properties": {
    "group_id": {
      "type": "integer",
      "description": "业务组 ID，用户未指定时使用默认业务组"
    },
    "targets": {
      "type": "array",
      "items": { "type": "string" },
      "description": "要屏蔽的目标标识列表，如 ['host-01', 'host-02']"
    },
    "cause": {
      "type": "string",
      "description": "屏蔽原因"
    },
    "btime": {
      "type": "integer",
      "description": "开始时间戳，默认为当前时间"
    },
    "etime": {
      "type": "integer",
      "description": "结束时间戳"
    },
    "duration": {
      "type": "integer",
      "description": "持续时长(秒)，与 etime 二选一，如 3600 表示 1 小时"
    }
  },
  "required": ["targets"]
}
```

### 2.3 工具定义示例

```json
{
  "name": "alert_mute_create",
  "display_name": "创建告警屏蔽",
  "description": "当用户需要临时屏蔽某些主机、服务或规则的告警时使用此工具。可以指定屏蔽时长或结束时间。",
  "category": "alert",
  "type": "api",
  "method": "POST",
  "url_pattern": "/api/n9e/busi-group/{group_id}/alert-mutes",
  "path_params_schema": {
    "type": "object",
    "properties": {
      "group_id": { "type": "integer", "description": "业务组ID" }
    },
    "required": ["group_id"]
  },
  "request_body_schema": {
    "type": "object",
    "properties": {
      "prod": { "type": "string", "default": "host", "description": "产品类型" },
      "note": { "type": "string", "description": "备注说明" },
      "cate": { "type": "integer", "default": 0, "description": "屏蔽类型" },
      "btime": { "type": "integer", "description": "开始时间戳" },
      "etime": { "type": "integer", "description": "结束时间戳" },
      "disabled": { "type": "integer", "default": 0 },
      "mute_time_type": { "type": "integer", "default": 0 },
      "severities": { "type": "array", "items": { "type": "integer" } },
      "tags": { "type": "array", "items": { "type": "object" } }
    },
    "required": ["btime", "etime"]
  },
  "keywords": ["屏蔽", "静默", "mute", "不告警", "暂停告警"],
  "required_permission": "/alert-mutes/add",
  "risk_level": 2,
  "example_prompt": "帮我屏蔽 host-01 的告警 2 小时",
  "example_params": {
    "group_id": 1,
    "targets": ["host-01"],
    "duration": 7200
  }
}
```

---

## 3. 通用 API 执行器设计

### 3.1 Go 代码结构

```go
// center/aiassistant/tools/executor.go

package tools

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "regexp"
    "strings"
    "time"
)

// ToolExecutor 通用工具执行器
type ToolExecutor struct {
    baseURL    string
    httpClient *http.Client
}

// ExecuteRequest 执行工具请求的输入
type ExecuteRequest struct {
    ToolName   string                 `json:"tool_name"`
    Parameters map[string]interface{} `json:"parameters"`
    UserToken  string                 `json:"-"` // 用户认证 Token
}

// ExecuteResult 执行结果
type ExecuteResult struct {
    Success    bool        `json:"success"`
    Data       interface{} `json:"data,omitempty"`
    Error      string      `json:"error,omitempty"`
    StatusCode int         `json:"status_code"`
}

// Execute 执行工具调用
func (e *ToolExecutor) Execute(ctx context.Context, tool *AITool, req *ExecuteRequest) (*ExecuteResult, error) {
    switch tool.Type {
    case "api":
        return e.executeAPITool(ctx, tool, req)
    case "mcp":
        return e.executeMCPTool(ctx, tool, req)
    default:
        return nil, fmt.Errorf("unsupported tool type: %s", tool.Type)
    }
}

// executeAPITool 执行 API 类型工具
func (e *ToolExecutor) executeAPITool(ctx context.Context, tool *AITool, req *ExecuteRequest) (*ExecuteResult, error) {
    // 1. 构建 URL (替换路径参数)
    url := e.buildURL(tool.URLPattern, req.Parameters)
    
    // 2. 分离请求体参数和查询参数
    bodyParams := e.extractBodyParams(tool, req.Parameters)
    queryParams := e.extractQueryParams(tool, req.Parameters)
    
    // 3. 添加查询参数到 URL
    if len(queryParams) > 0 {
        url = e.appendQueryParams(url, queryParams)
    }
    
    // 4. 构建 HTTP 请求
    var bodyReader io.Reader
    if len(bodyParams) > 0 && (tool.Method == "POST" || tool.Method == "PUT") {
        bodyBytes, _ := json.Marshal(bodyParams)
        bodyReader = bytes.NewReader(bodyBytes)
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, tool.Method, url, bodyReader)
    if err != nil {
        return nil, err
    }
    
    // 5. 设置请求头
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+req.UserToken)
    
    // 6. 执行请求
    resp, err := e.httpClient.Do(httpReq)
    if err != nil {
        return &ExecuteResult{Success: false, Error: err.Error()}, nil
    }
    defer resp.Body.Close()
    
    // 7. 解析响应
    respBody, _ := io.ReadAll(resp.Body)
    
    var result ExecuteResult
    result.StatusCode = resp.StatusCode
    
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        var respData struct {
            Dat   interface{} `json:"dat"`
            Error string      `json:"error"`
        }
        json.Unmarshal(respBody, &respData)
        
        if respData.Error != "" {
            result.Success = false
            result.Error = respData.Error
        } else {
            result.Success = true
            result.Data = respData.Dat
        }
    } else {
        result.Success = false
        result.Error = string(respBody)
    }
    
    return &result, nil
}

// buildURL 替换 URL 中的路径参数
func (e *ToolExecutor) buildURL(pattern string, params map[string]interface{}) string {
    url := e.baseURL + pattern
    
    // 替换 {param} 格式的占位符
    re := regexp.MustCompile(`\{(\w+)\}`)
    url = re.ReplaceAllStringFunc(url, func(match string) string {
        key := match[1 : len(match)-1]
        if val, ok := params[key]; ok {
            return fmt.Sprintf("%v", val)
        }
        return match
    })
    
    return url
}
```

### 3.2 工具发现与选择流程

```go
// center/aiassistant/tools/discovery.go

package tools

import (
    "context"
    "strings"
)

// ToolDiscovery 工具发现服务
type ToolDiscovery struct {
    toolCache map[string]*AITool // 工具缓存
}

// FindRelevantTools 根据用户意图查找相关工具
func (d *ToolDiscovery) FindRelevantTools(ctx context.Context, userIntent string, category string) ([]*AITool, error) {
    var candidates []*AITool
    
    for _, tool := range d.toolCache {
        if !tool.Enabled {
            continue
        }
        
        // 按分类过滤
        if category != "" && tool.Category != category {
            continue
        }
        
        // 关键词匹配
        if d.matchKeywords(userIntent, tool.Keywords) {
            candidates = append(candidates, tool)
            continue
        }
        
        // 描述相似度匹配 (可扩展为向量检索)
        if d.matchDescription(userIntent, tool.Description) {
            candidates = append(candidates, tool)
        }
    }
    
    return candidates, nil
}

// matchKeywords 检查是否匹配关键词
func (d *ToolDiscovery) matchKeywords(text string, keywords []string) bool {
    textLower := strings.ToLower(text)
    for _, kw := range keywords {
        if strings.Contains(textLower, strings.ToLower(kw)) {
            return true
        }
    }
    return false
}

// GenerateToolPrompt 生成工具选择的 Prompt
func (d *ToolDiscovery) GenerateToolPrompt(tools []*AITool) string {
    var sb strings.Builder
    sb.WriteString("你可以使用以下工具来完成用户的请求：\n\n")
    
    for i, tool := range tools {
        sb.WriteString(fmt.Sprintf("%d. **%s** (%s)\n", i+1, tool.DisplayName, tool.Name))
        sb.WriteString(fmt.Sprintf("   描述: %s\n", tool.Description))
        sb.WriteString(fmt.Sprintf("   风险等级: %s\n", riskLevelToString(tool.RiskLevel)))
        if tool.ExamplePrompt != "" {
            sb.WriteString(fmt.Sprintf("   示例: \"%s\"\n", tool.ExamplePrompt))
        }
        sb.WriteString("\n")
    }
    
    sb.WriteString("\n请根据用户意图选择最合适的工具，并提取必要的参数。")
    return sb.String()
}
```

---

## 4. LLM 工具调用协议

### 4.1 Function Calling 格式

当 LLM 决定调用工具时，返回标准格式：

```json
{
  "tool_calls": [
    {
      "id": "call_001",
      "type": "function",
      "function": {
        "name": "alert_mute_create",
        "arguments": {
          "group_id": 1,
          "targets": ["host-01"],
          "cause": "系统维护",
          "duration": 7200
        }
      }
    }
  ]
}
```

### 4.2 多步骤工具调用

对于复杂任务，AI 可以规划多个工具调用：

```json
{
  "plan": "为了完成用户请求，我需要：1) 查询目标主机信息 2) 创建告警屏蔽",
  "tool_calls": [
    {
      "step": 1,
      "function": {
        "name": "target_query",
        "arguments": { "query": "host-01" }
      },
      "reason": "先确认主机存在"
    },
    {
      "step": 2,
      "depends_on": [1],
      "function": {
        "name": "alert_mute_create",
        "arguments": { "targets": ["$step1.result.ident"], "duration": 7200 }
      },
      "reason": "为该主机创建屏蔽规则"
    }
  ]
}
```

### 4.3 参数智能补全

AI 需要智能处理用户未明确指定的参数：

| 参数场景 | AI 行为 |
|---------|--------|
| 时间相对表达 | "2小时" -> 计算为 Unix 时间戳 |
| 默认值 | group_id 未指定 -> 使用用户默认业务组 |
| 模糊匹配 | "生产服务器" -> 查询匹配的 targets |
| 确认高风险操作 | 删除操作前先询问确认 |

---

## 5. 系统 API 分类与工具映射

### 5.1 告警管理类 (Category: alert)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `alert_rule_list` | `/api/n9e/busi-group/:id/alert-rules` | GET | 查询告警规则列表 | 低 |
| `alert_rule_get` | `/api/n9e/alert-rule/:arid` | GET | 获取告警规则详情 | 低 |
| `alert_rule_create` | `/api/n9e/busi-group/:id/alert-rules` | POST | 创建告警规则 | 中 |
| `alert_rule_update` | `/api/n9e/busi-group/:id/alert-rule/:arid` | PUT | 更新告警规则 | 中 |
| `alert_rule_delete` | `/api/n9e/busi-group/:id/alert-rules` | DELETE | 删除告警规则 | 高 |
| `alert_rule_enable` | `/api/n9e/busi-group/:id/alert-rules/fields` | PUT | 启用/禁用规则 | 中 |
| `alert_mute_list` | `/api/n9e/busi-groups/alert-mutes` | GET | 查询屏蔽规则 | 低 |
| `alert_mute_create` | `/api/n9e/busi-group/:id/alert-mutes` | POST | 创建告警屏蔽 | 中 |
| `alert_mute_delete` | `/api/n9e/busi-group/:id/alert-mutes` | DELETE | 删除屏蔽规则 | 中 |
| `alert_event_list` | `/api/n9e/alert-cur-events/list` | GET | 查询当前告警 | 低 |
| `alert_event_history` | `/api/n9e/alert-his-events/list` | GET | 查询历史告警 | 低 |
| `alert_event_claim` | `/api/n9e/alert-cur-event/:eid/claim` | PUT | 认领告警 | 低 |

### 5.2 监控对象类 (Category: target)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `target_list` | `/api/n9e/targets` | GET | 查询监控对象列表 | 低 |
| `target_get` | `/api/n9e/target/extra-meta` | GET | 获取对象详情 | 低 |
| `target_bind_tags` | `/api/n9e/targets/tags` | POST | 绑定标签 | 中 |
| `target_unbind_tags` | `/api/n9e/targets/tags` | DELETE | 解绑标签 | 中 |
| `target_update_note` | `/api/n9e/targets/note` | PUT | 更新备注 | 低 |
| `target_bind_group` | `/api/n9e/targets/bgids` | PUT | 绑定业务组 | 中 |
| `target_delete` | `/api/n9e/targets` | DELETE | 删除监控对象 | 高 |

### 5.3 仪表盘类 (Category: dashboard)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `dashboard_list` | `/api/n9e/busi-group/:id/boards` | GET | 查询仪表盘列表 | 低 |
| `dashboard_get` | `/api/n9e/board/:bid` | GET | 获取仪表盘详情 | 低 |
| `dashboard_create` | `/api/n9e/busi-group/:id/boards` | POST | 创建仪表盘 | 中 |
| `dashboard_update` | `/api/n9e/board/:bid` | PUT | 更新仪表盘 | 中 |
| `dashboard_delete` | `/api/n9e/boards` | DELETE | 删除仪表盘 | 高 |
| `dashboard_clone` | `/api/n9e/busi-group/:id/board/:bid/clone` | POST | 克隆仪表盘 | 低 |

### 5.4 数据查询类 (Category: query)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `metrics_query_range` | `/api/n9e/query-range-batch` | POST | 时序数据范围查询 | 低 |
| `metrics_query_instant` | `/api/n9e/query-instant-batch` | POST | 时序数据即时查询 | 低 |
| `logs_query` | `/api/n9e/log-query` | POST | 日志查询 | 低 |
| `datasource_list` | `/api/n9e/datasource/brief` | GET | 数据源列表 | 低 |

### 5.5 用户管理类 (Category: user)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `user_list` | `/api/n9e/users` | GET | 查询用户列表 | 低 |
| `user_create` | `/api/n9e/users` | POST | 创建用户 | 高 |
| `user_update` | `/api/n9e/user/:id/profile` | PUT | 更新用户信息 | 中 |
| `user_delete` | `/api/n9e/user/:id` | DELETE | 删除用户 | 高 |
| `user_group_list` | `/api/n9e/user-groups` | GET | 查询用户组 | 低 |

### 5.6 业务组类 (Category: busi_group)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `busi_group_list` | `/api/n9e/busi-groups` | GET | 查询业务组列表 | 低 |
| `busi_group_get` | `/api/n9e/busi-group/:id` | GET | 获取业务组详情 | 低 |
| `busi_group_create` | `/api/n9e/busi-groups` | POST | 创建业务组 | 中 |
| `busi_group_update` | `/api/n9e/busi-group/:id` | PUT | 更新业务组 | 中 |
| `busi_group_delete` | `/api/n9e/busi-group/:id` | DELETE | 删除业务组 | 高 |
| `busi_group_add_member` | `/api/n9e/busi-group/:id/members` | POST | 添加成员 | 中 |

### 5.7 通知管理类 (Category: notify)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `notify_rule_list` | `/api/n9e/notify-rules` | GET | 查询通知规则 | 低 |
| `notify_rule_create` | `/api/n9e/notify-rules` | POST | 创建通知规则 | 中 |
| `notify_channel_list` | `/api/n9e/notify-channel-configs` | GET | 查询通知渠道 | 低 |
| `notify_test` | `/api/n9e/notify-rule/test` | POST | 测试通知 | 低 |

### 5.8 DBM 数据库管理类 (Category: dbm)

| 工具名 | API 路径 | 方法 | 描述 | 风险 |
|-------|---------|------|------|------|
| `dbm_instance_list` | `/api/n9e/dbm/instances` | GET | 数据库实例列表 | 低 |
| `dbm_instance_health` | `/api/n9e/dbm/instance/:id/health` | GET | 实例健康检查 | 低 |
| `dbm_sessions` | `/api/n9e/dbm/sessions` | POST | 查询会话 | 低 |
| `dbm_kill_sessions` | `/api/n9e/dbm/sessions/kill` | POST | 终止会话 | 高 |
| `dbm_slow_queries` | `/api/n9e/dbm/slow-queries` | POST | 慢查询分析 | 低 |
| `dbm_sql_execute` | `/api/n9e/dbm/sql/query` | POST | 执行 SQL | 高 |
| `dbm_lock_waits` | `/api/n9e/dbm/lock-waits` | POST | 锁等待查询 | 低 |

---

## 6. 安全与权限控制

### 6.1 权限验证流程

```
用户请求 -> AI 选择工具 -> 权限检查 -> 执行/拒绝
                              │
                              ▼
                    ┌─────────────────┐
                    │ 检查用户角色    │
                    │ 检查权限点      │
                    │ 检查资源归属    │
                    └─────────────────┘
```

### 6.2 风险等级控制

| 风险等级 | 处理方式 |
|---------|---------|
| 1 (低) | 直接执行查询类操作 |
| 2 (中) | 执行前告知用户操作内容 |
| 3 (高) | 必须用户明确确认才执行 |

### 6.3 操作审计

所有通过 AI 工具执行的操作都记录到 `ai_tool_execution_log` 表：

```sql
CREATE TABLE `ai_tool_execution_log` (
    `id` bigint PRIMARY KEY AUTO_INCREMENT,
    `session_id` varchar(64) NOT NULL,
    `user_id` bigint NOT NULL,
    `tool_name` varchar(128) NOT NULL,
    `parameters` text,
    `result_success` tinyint(1),
    `result_data` text,
    `error_message` text,
    `execution_time_ms` int,
    `created_at` bigint,
    
    INDEX `idx_session` (`session_id`),
    INDEX `idx_user` (`user_id`),
    INDEX `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## 7. 工具使用示例

### 7.1 查询告警

**用户**: "最近有什么告警？"

**AI 调用**:
```json
{
  "tool": "alert_event_list",
  "parameters": {
    "limit": 20,
    "stime": 1704067200,
    "etime": 1704153600
  }
}
```

**API 请求**:
```http
GET /api/n9e/alert-cur-events/list?limit=20&stime=1704067200&etime=1704153600
Authorization: Bearer <user_token>
```

### 7.2 创建告警屏蔽

**用户**: "帮我屏蔽 db-master-01 的告警 3 小时，原因是数据库维护"

**AI 调用**:
```json
{
  "tool": "alert_mute_create",
  "parameters": {
    "group_id": 1,
    "tags": [{"key": "ident", "func": "==", "value": "db-master-01"}],
    "cause": "数据库维护",
    "btime": 1704153600,
    "etime": 1704164400
  }
}
```

**API 请求**:
```http
POST /api/n9e/busi-group/1/alert-mutes
Authorization: Bearer <user_token>
Content-Type: application/json

{
  "prod": "host",
  "note": "数据库维护",
  "cate": 0,
  "btime": 1704153600,
  "etime": 1704164400,
  "tags": [{"key": "ident", "func": "==", "value": "db-master-01"}],
  "disabled": 0,
  "mute_time_type": 0
}
```

### 7.3 查询仪表盘数据

**用户**: "查看 web-server-01 最近 1 小时的 CPU 使用率"

**AI 调用**:
```json
{
  "tool": "metrics_query_range",
  "parameters": {
    "datasource_id": 1,
    "prom_ql": "100 - avg(rate(node_cpu_seconds_total{mode=\"idle\",ident=\"web-server-01\"}[5m])) * 100",
    "start": 1704150000,
    "end": 1704153600,
    "step": 60
  }
}
```

### 7.4 终止数据库会话 (高风险)

**用户**: "kill 掉 MySQL 里面那个运行了 2 小时的查询"

**AI 响应**: 
> 这是一个高风险操作，我检测到有以下长时间运行的会话：
> - 会话 ID: 12345, 运行时间: 2小时15分, SQL: SELECT * FROM large_table...
> 
> 确认要终止这个会话吗？请回复"确认"继续。

**用户**: "确认"

**AI 调用**:
```json
{
  "tool": "dbm_kill_sessions",
  "parameters": {
    "instance_id": 3,
    "session_ids": [12345]
  }
}
```

---

## 8. 扩展与最佳实践

### 8.1 新增工具的流程

1. **分析 API**: 确定要映射的 API 端点
2. **定义 Schema**: 编写请求参数的 JSON Schema
3. **配置工具**: 在管理界面或数据库中创建 `ai_tool` 记录
4. **测试验证**: 通过 AI 对话测试工具调用
5. **优化描述**: 根据 LLM 理解情况调整 description 和 keywords

### 8.2 工具描述编写规范

好的工具描述应该：

- **明确场景**: 说明何时应该使用这个工具
- **包含动词**: 使用"创建"、"查询"、"删除"等明确动词
- **列出约束**: 说明必要的前置条件
- **给出示例**: 包含典型的使用场景

**好的描述**:
> 当用户需要临时屏蔽某些主机或服务的告警时使用。支持按主机标识、标签、告警规则等条件屏蔽。用户需要指定屏蔽时长或结束时间。

**差的描述**:
> 创建告警屏蔽规则。

### 8.3 与 MCP 工具的统一

系统同时支持内部 API 工具和外部 MCP 工具，统一的工具接口使 LLM 无需区分工具来源：

```go
// 统一的工具接口
type Tool interface {
    GetName() string
    GetDescription() string
    GetParameterSchema() *JSONSchema
    Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
}

// API 工具实现
type APITool struct { ... }

// MCP 工具实现  
type MCPTool struct { ... }
```

---

## 9. 未来规划

### 9.1 向量化工具检索

当工具数量超过 50 个时，采用 Embedding + 向量检索替代关键词匹配：

1. 预计算所有工具描述的 Embedding
2. 用户意图 -> Embedding -> 向量相似度搜索
3. 返回 Top-K 候选工具给 LLM 选择

### 9.2 工具链编排

支持定义工具组合模式：

```yaml
name: "complete_maintenance_workflow"
description: "完整的维护工作流：屏蔽告警 -> 执行操作 -> 取消屏蔽"
steps:
  - tool: alert_mute_create
    save_result_as: mute_id
  - tool: $user_selected_operation
    wait_for_confirmation: true
  - tool: alert_mute_delete
    parameters:
      ids: ["$mute_id"]
```

### 9.3 自动化工具生成

基于 OpenAPI/Swagger 规范自动生成工具定义：

```bash
# 从 Swagger 文档导入工具
n9e-cli tools import --swagger ./swagger.json --category alert
```
