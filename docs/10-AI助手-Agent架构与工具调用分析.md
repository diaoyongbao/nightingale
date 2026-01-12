# AI助手 - Agent架构与工具调用分析

> **文档目的**: 基于 `09-AI助手-AIChat与MCP设计.md` 中的父子 Agent 架构设计，整理当前实现方式及每个 Sub Agent 的 Function 调用情况，并分析需要优化的方向。

---

## 1. 架构概览

### 1.1 设计愿景 vs 当前实现

| 维度 | 设计文档愿景 | 当前实现状态 | 差距分析 |
|------|--------------|--------------|----------|
| **架构模式** | 父子 Agent 架构（Coordinator + Expert Agents） | ✅ 已实现基础框架 | 框架已搭建，但专家 Agent 调用能力有限 |
| **路由决策** | 关键词 + LLM 混合路由 | ⚠️ 仅关键词路由 | 缺少 LLM 路由降级机制 |
| **专家 Agent** | K8s/数据库/告警/云服务 | ⚠️ 结构定义，工具未对接 | 专家 Agent 的工具调用未实现 |
| **工具执行** | MCP 工具 + 内置工具 | ⚠️ 仅知识库工具 | MCP/DBM/K8s 工具未集成 |
| **Function Calling** | OpenAI 标准格式 | ✅ 已实现 | 支持 Function Calling 架构 |

### 1.2 当前架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            用户请求                                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Chat Handler (统一入口)                               │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  handleWithFunctionCalling()                                          │  │
│  │  - 构建 System Prompt                                                 │  │
│  │  - 加载工具定义 (知识库工具)                                           │  │
│  │  - 调用 LLM (OpenAI Function Calling)                                │  │
│  │  - 处理工具调用结果                                                    │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                     ┌──────────────┼──────────────┐
                     │              │              │
                     ▼              ▼              ▼
            ┌───────────────┐ ┌──────────┐ ┌───────────────┐
            │ 知识库工具     │ │ MCP 工具 │ │ 内置工具       │
            │ (已实现)       │ │ (未对接) │ │ (未实现)       │
            ├───────────────┤ ├──────────┤ ├───────────────┤
            │ Cloudflare RAG│ │ K8s MCP  │ │ DBM SQL       │
            │ Coze Provider │ │          │ │ Alert Mute    │
            └───────────────┘ └──────────┘ └───────────────┘
```

---

## 2. 父 Agent (CoordinatorAgent) 分析

### 2.1 代码位置

```
center/aiassistant/agent/
├── coordinator.go   # 父 Agent 协调器
├── experts.go       # 专家 Agent 定义
└── types.go         # 类型定义
```

### 2.2 CoordinatorAgent 结构

```go
// center/aiassistant/agent/coordinator.go
type CoordinatorAgent struct {
    aiClient     ai.AIClient
    expertAgents map[AgentType]*ExpertAgent
    routeCache   map[string]AgentType  // 路由缓存
}
```

### 2.3 核心方法

| 方法 | 功能 | 当前状态 |
|------|------|----------|
| `NewCoordinatorAgent()` | 创建协调器，注册专家 Agent | ✅ 已实现 |
| `Route(ctx, message)` | 路由决策（关键词匹配） | ⚠️ 仅关键词，缺少 LLM 路由 |
| `fastRoute(message)` | 快速路由（零成本） | ✅ 已实现 |
| `DelegateToExpert()` | 委派给专家 Agent | ⚠️ 仅调用 LLM，未执行工具 |
| `handleGeneral()` | 通用任务处理 | ✅ 已实现 |

### 2.4 路由关键词配置

```go
// K8s 相关
k8sKeywords := []string{"pod", "deployment", "service", "k8s", "kubernetes", 
                        "容器", "镜像", "namespace", "node", "ingress"}

// 数据库相关
dbKeywords := []string{"sql", "数据库", "慢查询", "query", "table", "索引", 
                       "mysql", "postgresql", "redis", "mongodb", "select", 
                       "insert", "update", "delete"}

// 告警相关
alertKeywords := []string{"告警", "屏蔽", "mute", "alert", "通知", "规则", "订阅"}
```

### 2.5 问题分析

| 问题 | 描述 | 优化建议 |
|------|------|----------|
| **无 LLM 路由** | 复杂意图无法通过关键词识别 | 添加 LLM 路由降级机制 |
| **CoordinatorAgent 未集成到 ChatHandler** | Handler 直接使用 Function Calling，绕过 Coordinator | 需要决定是继续 Function Calling 还是切换到父子 Agent |
| **专家 Agent 工具未执行** | DelegateToExpert 只调用 LLM，不执行工具 | 需要集成工具调用逻辑 |

---

## 3. 专家 Agent (ExpertAgent) 分析

### 3.1 专家 Agent 定义

```go
// center/aiassistant/agent/types.go
type ExpertAgent struct {
    Name         string   `json:"name"`
    SystemPrompt string   `json:"system_prompt"`
    Tools        []Tool   `json:"tools"`
    Model        string   `json:"model"`
    Temperature  float64  `json:"temperature"`
    Keywords     []string `json:"keywords"`  // 路由关键词
}
```

### 3.2 已注册的专家 Agent

#### 3.2.1 K8s 专家 Agent

| 属性 | 值 |
|------|-----|
| **名称** | K8s专家 |
| **模型** | gpt-4 |
| **温度** | 0.3 |
| **关键词** | pod, deployment, service, k8s, kubernetes, 容器, 镜像 |

**System Prompt**:
```
你是 Kubernetes 运维专家，擅长：
- Pod 故障诊断（CrashLoopBackOff、ImagePullBackOff、OOMKilled 等）
- 资源调度分析（CPU/内存超限、节点亲和性）
- 配置优化建议（副本数、资源限制、探针配置）

诊断步骤：
1. 先查看 Pod 状态和事件
2. 检查日志（最近 100 行）
3. 分析资源使用情况
4. 给出可操作的修复建议

遵循 Kubernetes 最佳实践。
```

**工具列表**:

| 工具名称 | 描述 | 实现状态 |
|----------|------|----------|
| `k8s_list_pods` | 列出 Pod | ❌ 未实现 |
| `k8s_get_logs` | 获取 Pod 日志 | ❌ 未实现 |
| `k8s_describe` | 查看资源详情 | ❌ 未实现 |
| `k8s_get_events` | 获取事件 | ❌ 未实现 |

---

#### 3.2.2 数据库专家 Agent

| 属性 | 值 |
|------|-----|
| **名称** | 数据库专家 |
| **模型** | gpt-3.5-turbo |
| **温度** | 0.2 |
| **关键词** | sql, 数据库, 慢查询, query, table, 索引 |

**System Prompt**:
```
你是数据库运维专家，擅长：
- SQL 查询优化（慢查询分析、索引建议）
- 执行计划分析
- 数据库性能诊断

分析步骤：
1. 先检查 SQL 语法
2. 分析执行计划（EXPLAIN）
3. 提出索引优化建议
4. 评估数据量和查询成本

注意事项：
- 所有写操作（INSERT/UPDATE/DELETE）必须二次确认
- SELECT 未加 LIMIT 时提醒用户
- 多语句查询需要额外审核
```

**工具列表**:

| 工具名称 | 描述 | 实现状态 |
|----------|------|----------|
| `dbm_sql_check` | SQL 语法检查 | ❌ 未实现 (DBM 模块已有 API) |
| `dbm_sql_query` | 执行 SQL 查询 | ❌ 未实现 (DBM 模块已有 API) |
| `dbm_explain` | 查看执行计划 | ❌ 未实现 |

---

#### 3.2.3 告警专家 Agent

| 属性 | 值 |
|------|-----|
| **名称** | 告警专家 |
| **模型** | gpt-3.5-turbo |
| **温度** | 0.1 |
| **关键词** | 告警, 屏蔽, mute, alert, 通知 |

**System Prompt**:
```
你是告警管理专家，擅长：
- 告警规则配置
- 告警屏蔽策略设计
- 告警降噪优化

操作步骤：
1. 理解用户屏蔽需求
2. 设计精准的屏蔽规则（tags 过滤）
3. 使用 preview 评估影响范围
4. 超过 50 条匹配时要求用户确认

原则：
- 避免过宽的屏蔽规则（tags 为空、正则 .*）
- 时间范围应明确（避免永久屏蔽）
- 优先使用 tags 而非 datasource 全量
```

**工具列表**:

| 工具名称 | 描述 | 实现状态 |
|----------|------|----------|
| `alert_mute_preview` | 预览屏蔽影响 | ❌ 未实现 (夜莺已有 API) |
| `alert_mute_create` | 创建屏蔽规则 | ❌ 未实现 (夜莺已有 API) |
| `alert_mute_list` | 列出屏蔽规则 | ❌ 未实现 (夜莺已有 API) |

---

## 4. 实际工作的工具系统

### 4.1 当前已实现的工具调用流程

```
用户请求
    │
    ▼
ChatHandler.HandleChat()
    │
    ▼
handleWithFunctionCalling()
    │
    ├─→ buildToolDefinitions() ─→ 从 KnowledgeToolRegistry 获取工具定义
    │
    ├─→ aiClient.ChatCompletion() ─→ 调用 LLM (Function Calling)
    │
    └─→ 处理 ToolCalls
            │
            ├─→ 单个工具调用: handleToolCall()
            │       │
            │       └─→ 知识库工具: executeKnowledgeTool()
            │
            └─→ 多个工具调用: handleMultipleToolCalls()
                    │
                    └─→ optManager.ExecuteToolsConcurrently()
```

### 4.2 知识库工具注册表

**代码位置**: `center/aiassistant/knowledge/registry.go`

```go
type KnowledgeToolRegistry struct {
    tools     map[string]*RegisteredTool  // 注册的工具
    providers map[int64]Provider          // Provider 实例
    mu        sync.RWMutex
    c         *ctx.Context
}
```

**核心方法**:

| 方法 | 功能 |
|------|------|
| `LoadFromConfig()` | 从数据库加载 Provider 和工具配置 |
| `GetToolDefinitions()` | 返回 OpenAI Function Calling 格式的工具定义 |
| `IsKnowledgeTool()` | 判断是否为知识库工具 |
| `ExecuteTool()` | 执行知识库查询 |

### 4.3 已实现的知识库 Provider

| Provider | 类型 | 状态 |
|----------|------|------|
| **Cloudflare AutoRAG** | 向量检索 | ✅ 已实现 |
| **Coze** | Bot 对话 | ⚠️ 代码存在，未完全测试 |

---

## 5. 辅助模块分析

### 5.1 并发执行器 (executor)

**代码位置**: `center/aiassistant/executor/concurrent.go`

```go
type ConcurrentExecutor struct {
    maxConcurrency int
    config         *models.ConcurrentConfig
    mu             sync.RWMutex
    appCtx         *ctx.Context
}
```

**核心方法**:

| 方法 | 功能 |
|------|------|
| `ExecuteAll()` | 并发执行多个工具调用 |
| `ExecuteOne()` | 执行单个工具调用 |
| `AggregateResults()` | 聚合执行结果 |

### 5.2 风险检查器 (risk)

**代码位置**: `center/aiassistant/risk/checker.go`

```go
type Checker struct {
    config *Config
}

type Level string  // low/medium/high
```

**风险检查方法**:

| 方法 | 功能 | 触发条件 |
|------|------|----------|
| `CheckSQL()` | SQL 风险判定 | 写操作/多语句/无 LIMIT |
| `CheckAlertMute()` | 告警屏蔽风险 | tags 空/匹配数过多 |
| `CheckK8sOperation()` | K8s 操作风险 | exec/scale/delete 等 |
| `CheckOperation()` | 通用操作风险 | 批量操作/目标不明确 |

### 5.3 确认管理器 (confirmation)

**代码位置**: `center/aiassistant/confirmation/`

支持高风险操作的二次确认机制。

---

## 6. 架构优化建议

### 6.1 问题一：父子 Agent 架构未真正启用

**现状**: 
- `CoordinatorAgent` 已定义，但 `ChatHandler` 直接使用 Function Calling，绕过了父子 Agent 架构
- 专家 Agent 的工具列表仅是定义，未实际执行

**优化方案 A - 启用父子 Agent 架构**:

```go
// ChatHandler 改为调用 CoordinatorAgent
func (h *Handler) HandleChat(ctx context.Context, req *ChatRequest, userID int64) (*ChatResponse, error) {
    // 1. 路由决策
    agentType := h.coordinator.Route(ctx, req.Message)
    
    // 2. 如果是通用任务，使用 Function Calling
    if agentType == agent.AgentTypeGeneral {
        return h.handleWithFunctionCalling(ctx, req, traceID, sessionID, userID)
    }
    
    // 3. 否则委派给专家 Agent
    return h.delegateToExpert(ctx, agentType, req, traceID, sessionID, userID)
}
```

**优化方案 B - 继续 Function Calling，增强工具集**:

继续使用 Function Calling 架构，但丰富工具定义，让 LLM 自行选择工具。

### 6.2 问题二：专家 Agent 工具未实现

**需要实现的工具**:

#### K8s 工具 (需对接 MCP Server)

```go
// 工具注册
coordinator.registerTool("k8s_list_pods", &K8sListPodsHandler{})
coordinator.registerTool("k8s_get_logs", &K8sGetLogsHandler{})
coordinator.registerTool("k8s_describe", &K8sDescribeHandler{})
coordinator.registerTool("k8s_get_events", &K8sGetEventsHandler{})
```

#### DBM 工具 (复用现有 API)

```go
// 复用 center/dbm 模块
coordinator.registerTool("dbm_sql_check", &DBMSQLCheckHandler{
    client: dbm.MySQLClient,
})
coordinator.registerTool("dbm_sql_query", &DBMSQLQueryHandler{
    client: dbm.MySQLClient,
})
```

#### 告警工具 (复用现有 API)

```go
// 复用 models/alert_mute.go
coordinator.registerTool("alert_mute_preview", &AlertMutePreviewHandler{})
coordinator.registerTool("alert_mute_create", &AlertMuteCreateHandler{})
```

### 6.3 问题三：MCP 客户端未集成

**现状**: MCP 客户端代码存在，但未与工具执行流程打通

**建议**:

```go
// center/aiassistant/mcp/manager.go
type MCPManager struct {
    clients map[int64]MCPClientInterface
    tools   map[string]*MCPToolBinding  // 工具名 -> MCP Server 映射
}

func (m *MCPManager) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResponse, error) {
    binding := m.tools[toolName]
    client := m.clients[binding.ServerID]
    return client.CallTool(ctx, &ToolRequest{
        Name:      toolName,
        Arguments: args,
    })
}
```

### 6.4 建议的工作优先级

| 优先级 | 任务 | 说明 |
|--------|------|------|
| **P0** | 明确架构选择 | 决定是启用父子 Agent 还是继续 Function Calling |
| **P1** | 实现 DBM 工具对接 | 复用现有 DBM API 实现 SQL 工具 |
| **P1** | 实现 Alert Mute 工具 | 复用现有告警屏蔽 API |
| **P2** | 启用 MCP Manager | 对接外部 MCP Server |
| **P2** | 实现 K8s 工具 | 需要部署 K8s MCP Server |
| **P3** | 添加 LLM 路由 | 增强路由决策能力 |

---

## 7. 工具调用矩阵

### 7.1 设计文档中的工具 vs 实现状态

| 类别 | 工具名称 | 设计用途 | 实现状态 | 备注 |
|------|----------|----------|----------|------|
| **知识库** | search_* | 知识库检索 | ✅ 已实现 | Cloudflare RAG |
| **K8s** | k8s_list_pods | 列出 Pod | ❌ 未实现 | 需 MCP Server |
| **K8s** | k8s_get_logs | 获取日志 | ❌ 未实现 | 需 MCP Server |
| **K8s** | k8s_describe | 资源详情 | ❌ 未实现 | 需 MCP Server |
| **K8s** | k8s_get_events | 获取事件 | ❌ 未实现 | 需 MCP Server |
| **DBM** | dbm_sql_check | SQL 语法检查 | ❌ 未实现 | API 已存在 |
| **DBM** | dbm_sql_query | SQL 查询 | ❌ 未实现 | API 已存在 |
| **DBM** | dbm_explain | 执行计划 | ❌ 未实现 | API 已存在 |
| **告警** | alert_mute_preview | 预览影响 | ❌ 未实现 | API 已存在 |
| **告警** | alert_mute_create | 创建屏蔽 | ❌ 未实现 | API 已存在 |
| **告警** | alert_mute_list | 列表查询 | ❌ 未实现 | API 已存在 |

### 7.2 当前可用的工具调用路径

```
请求 → ChatHandler → LLM Function Calling → 知识库工具 → Provider → 返回结果
                              ↓
                         其他工具 → 返回"工具暂未实现"
```

---

## 8. 结论

### 8.1 当前实现总结

1. **框架已搭建**: 父子 Agent 架构、Function Calling 机制均已实现
2. **知识库可用**: Cloudflare RAG 知识库工具可正常使用
3. **工具缺失**: K8s/DBM/告警专家 Agent 的工具均未实现
4. **MCP 未启用**: MCP 客户端代码存在但未集成

### 8.2 需要优化的关键点

1. **选择架构方向**: 父子 Agent vs 纯 Function Calling
2. **补充工具实现**: 对接 DBM/告警现有 API 作为工具
3. **启用 MCP**: 实现外部 MCP Server 的工具调用
4. **增强路由**: 添加 LLM 路由提升意图识别准确率

---

## 附录：代码文件索引

| 文件路径 | 功能说明 |
|----------|----------|
| `center/aiassistant/agent/coordinator.go` | 父 Agent 协调器 |
| `center/aiassistant/agent/experts.go` | 专家 Agent 定义 |
| `center/aiassistant/agent/types.go` | Agent 类型定义 |
| `center/aiassistant/chat/handler.go` | 对话处理器 |
| `center/aiassistant/chat/types.go` | 请求/响应类型 |
| `center/aiassistant/knowledge/registry.go` | 知识库工具注册表 |
| `center/aiassistant/knowledge/cloudflare_rag.go` | Cloudflare RAG Provider |
| `center/aiassistant/knowledge/coze_provider.go` | Coze Provider |
| `center/aiassistant/mcp/interface.go` | MCP 客户端接口 |
| `center/aiassistant/mcp/http_client.go` | HTTP MCP 客户端 |
| `center/aiassistant/mcp/manager.go` | MCP 管理器 |
| `center/aiassistant/executor/concurrent.go` | 并发执行器 |
| `center/aiassistant/risk/checker.go` | 风险检查器 |
| `center/aiassistant/confirmation/` | 确认管理器 |
| `models/mcp_server.go` | MCP Server 配置模型 |
| `models/mcp_template.go` | MCP 模板模型 |
