 09-AI助手-AIChat与MCP设计
 1. 背景与目标
 1.1 目标
在 Nightingale 管理系统新增侧边栏菜单 **AI 助手**，用户通过对话完成查询与运维操作，并支持：
- **AI 对话（Chat）**：采用 OpenAI/GPT 兼容协议 + MCP 工具，多轮对话；支持文件上传（图片转 base64、多模态）、会话 ID；返回 `pending_confirmation` 供高风险操作二次确认；**单轮仅调用一个相关工具**；**透传完整错误（结构化透传，不丢细节）**。
- **工具安全**：环境/前缀/IP 精准匹配；禁止无关/批量/订阅类工具；危险操作执行两步“检查-执行”并需用户确认；严禁幻觉结果（仅基于真实工具返回）。
- **会话与归档**：消息入 Redis；活跃会话统计与删除；定时/手动归档任务。
- **MCP 管理**：MCP Server 配置 CRUD，工具注册/分类统计，模板商店 CRUD（含默认模板 SQL），支持向 MCP Server 下发远程配置。
- **知识库**：`/knowledge/query` 调 Coze Bot（可替换为抽象 Provider），支持 `conversation_id` 与自定义 `bot_id`。
- **文件**：`/chat/upload` 返回 `file_id`；`/files/download/{path}` 提供下载链接；`file_transfer` 工具复用。
- **前端契约**：模式切换（知识库/MCP/预留任务）、Markdown 渲染、下载链接解析、文件上传返回字段保持兼容。
- **K8s MCP Server**：多集群前缀工具（list_pods/get_logs/exec/scale 等），环境变量绑定 kubeconfig/cluster/storage。
- **观测与日志**：`trace_id`/`session_id` 贯穿，记录工具调用/确认审计；`/health` 健康检查。
 1.2 非目标（明确约束）
- 不做“模型自行推断结果”：**严禁幻觉**，任何“执行结果/资源状态/数据”必须来自真实工具返回或真实系统 API 返回。
- 不做复杂的多工具链式编排：**单轮最多调用一个工具**（或一个检查工具，见二次确认机制）。
- 不开放订阅/长连接/持续运行型工具（watch/subscribe/stream）。
- 不允许无关工具、批量工具、以及默认危险工具（删除/破坏性操作）——除非明确允许且满足二次确认。
 2. 与夜莺现有前后端交互一致性（必须遵守）
 2.1 API 分层与前缀（沿用夜莺规划）
夜莺现有 API 规划（详见 `nightingale/docs/06-API接口文档.md`）：
- 页面 API：`/api/n9e/*`（JWT Bearer，前端页面使用）
- 服务 API：`/v1/n9e/*`（BasicAuth，服务间调用）
- Agent API：`/v1/n9e/heartbeat`
本模块属于“页面 API”，必须挂在 `/api/n9e/*`。
 2.2 前端请求封装约束（沿用 `fe/src/utils/request.tsx`）
前端所有页面请求默认通过 `request` 发送，并自动：
- 添加 `Authorization: Bearer <access_token>`
- 添加 `X-Language: <i18next.language>`
- 添加部署前缀 `basePrefix + url`
前端调用路径：`/api/n9e/...`
 2.3 返回结构与错误透传策略（顺应 `request` 拦截器）
前端 `request` 对错误会抛异常并可能触发全局通知；为了“工具错误完整透传 + UI 自主渲染错误”，本模块规定：
- **请求级错误**（鉴权、权限、参数不合法）：
  - 使用 HTTP 状态码（401/403/400），或返回 `{ error: '...' }`（会被前端拦截器当作失败）
- **工具级错误**（SQL 执行失败、MCP 工具失败、Coze 调用失败）：
  - **HTTP 200**
  - **顶层 `error` 必须为空字符串**：`error: ""`
  - 将错误写入 `dat.status = "error"` 且提供 `dat.tool.error`（结构化），确保前端能拿到完整错误细节
> 这样既"沿用夜莺常见返回结构 `{error, dat}`"，又避免前端拦截器丢失结构化错误。
 3. 命名与路径统一（前后端完全一致）
 3.1 前端路由（UI）
侧边栏菜单：**AI 助手**，统一路由前缀：`/ai-assistant`
建议子路由：
- `/ai-assistant`（默认进入对话）
- `/ai-assistant/chat`
- `/ai-assistant/knowledge`
- `/ai-assistant/mcp/servers`
- `/ai-assistant/mcp/tools`
- `/ai-assistant/mcp/templates`
- `/ai-assistant/sessions`
 3.2 后端 API BasePath（页面 API）
统一 API 前缀：`/api/n9e/ai-assistant/*`

基础接口：
- `GET  /api/n9e/ai-assistant/health`
- `POST /api/n9e/ai-assistant/chat`
- `POST /api/n9e/ai-assistant/chat/upload`
- `POST /api/n9e/ai-assistant/knowledge/query`
- `GET  /api/n9e/ai-assistant/files/download/{path}`

会话与归档：
- `GET    /api/n9e/ai-assistant/sessions/stats`
- `GET    /api/n9e/ai-assistant/sessions/{session_id}`
- `DELETE /api/n9e/ai-assistant/sessions/{session_id}`
- `POST   /api/n9e/ai-assistant/sessions/archive`

 4. 沿用与复用（避免重复造轮子）
 4.1 复用 DBM SQL 查询能力
夜莺已存在 DBM SQL 能力（页面 API，RBAC 已实现）：
- `POST /api/n9e/dbm/sql/check`（语法检查）
- `POST /api/n9e/dbm/sql/query`（执行 SQL）
AI 助手不重新实现 SQL 执行，只做：
- 意图识别、参数提取、风险判定
- 高风险二次确认（见第 7 章）
- 调用既有 DBM 执行逻辑（内部复用或转调均可）
 4.2 复用告警屏蔽（Alert Mute）
夜莺已存在完整告警屏蔽 CRUD + preview/tryrun：
- `/api/n9e/busi-group/:id/alert-mutes*`
AI 助手不重复实现屏蔽业务模型，只做：
- 生成屏蔽规则草案
- 调用 preview 评估影响面
- 超阈值/高风险触发二次确认
- 最终调用既有创建/更新/删除接口或内部复用模型逻辑
 4.3 复用“下载型链接”的交互模式
夜莺存在“任务下载/日志下载”一类场景：前端通过 `<a href=...>` 直接下载。
AI 助手文件下载也必须提供“可直接点击的下载链接”（见第 9 章），不能依赖 `Authorization` Header。
 5. 工具治理与安全模型
 5.1 工具来源（白名单）
AI 助手只能调用：
1) **系统内工具（Nightingale 内建）**：DBM SQL、Alert Mute、文件传输等（由后端封装成 tool）
2) **MCP 工具**：来源于已登记 MCP Server 且通过 env/prefix/ip 校验
 5.2 禁止的工具类型（硬禁止）
- 无关工具
- 批量工具（除非明确允许且仍需二次确认）
- 订阅类工具（watch/subscribe/stream）
- 跨环境工具（env/prefix 不匹配）
- 默认破坏性工具（delete/cleanup/drain 等）除非显式开放
 5.3 RBAC 与权限不可绕过
AI 助手必须遵守夜莺既有：
- JWT 鉴权
- 权限点（RBAC）校验
- 业务组读写权限校验（bgro/bgrw）
- MCP Server 管理默认仅 Admin（建议）

---

## 5A. Agent 工作模式（父子 Agent 架构）

### 5A.1 架构设计：父 Agent + 专家子 Agent

**本系统采用父子 Agent 架构**，参考 Claude Code 设计，针对运维场景优化。

```
┌─────────────────────────────────────────────────────────────┐
│           父 Agent：运维助手（Coordinator Agent）            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  职责：                                                │  │
│  │  1. 理解用户意图（路由决策）                           │  │
│  │  2. 简单任务直接处理（如查询文档、通用对话）           │  │
│  │  3. 复杂任务委派给专家子 Agent                         │  │
│  │  4. 汇总子 Agent 结果并格式化输出                      │  │
│  └───────────────────────────────────────────────────────┘  │
│                           │                                  │
│         路由决策（关键词 + LLM 混合）                        │
│                           │                                  │
└───────────────────────────┼──────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│ K8s 专家      │   │ 数据库专家     │   │ 告警专家       │
│ Agent         │   │ Agent         │   │ Agent         │
├───────────────┤   ├───────────────┤   ├───────────────┤
│ System Prompt │   │ System Prompt │   │ System Prompt │
│ K8s Tools     │   │ DBM Tools     │   │ Alert Tools   │
│ GPT-4         │   │ GPT-3.5       │   │ GPT-3.5       │
└───────────────┘   └───────────────┘   └───────────────┘
        │                   │                   │
        └───────────────────┴───────────────────┘
                            │
                  将结果返回给父 Agent
```

**核心优势**：
- ✅ **模块化**：每个子 Agent 专注特定领域
- ✅ **可扩展**：新增专家 Agent 不影响现有架构（如云服务专家、监控专家）
- ✅ **成本可控**：简单任务不调子 Agent，子 Agent 可用便宜模型
- ✅ **并行能力**：未来可支持多个子 Agent 并发执行（DAG 编排）
- ✅ **职责清晰**：父 Agent 协调，子 Agent 执行

### 5A.2 父 Agent 实现（运维助手）

```go
// center/aiassistant/agent/coordinator.go
package agent

import (
    "context"
    "github.com/ccfos/nightingale/v6/pkg/ai"
)

type CoordinatorAgent struct {
    aiClient      ai.AIClient
    expertAgents  map[AgentType]*ExpertAgent
    routeCache    *RouteCache  // 缓存常见路由决策
}

type AgentType string

const (
    AgentTypeGeneral   AgentType = "general"    // 父 Agent 自己处理
    AgentTypeK8s       AgentType = "k8s"        // K8s 专家
    AgentTypeDatabase  AgentType = "database"   // 数据库专家
    AgentTypeAlert     AgentType = "alert"      // 告警专家
    AgentTypeCloud     AgentType = "cloud"      // 云服务专家（可扩展）
)

func NewCoordinatorAgent(aiClient ai.AIClient) *CoordinatorAgent {
    coordinator := &CoordinatorAgent{
        aiClient:     aiClient,
        expertAgents: make(map[AgentType]*ExpertAgent),
        routeCache:   NewRouteCache(),
    }
    
    // 注册子 Agent
    coordinator.registerExpert(AgentTypeK8s, NewK8sExpertAgent())
    coordinator.registerExpert(AgentTypeDatabase, NewDatabaseExpertAgent())
    coordinator.registerExpert(AgentTypeAlert, NewAlertExpertAgent())
    
    return coordinator
}

func (c *CoordinatorAgent) HandleChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 路由决策（决定由谁处理）
    agentType, err := c.route(ctx, req.Message)
    if err != nil {
        return nil, err
    }
    
    // 2. 如果是通用任务，父 Agent 自己处理
    if agentType == AgentTypeGeneral {
        return c.handleGeneral(ctx, req)
    }
    
    // 3. 委派给专家子 Agent
    return c.delegateToExpert(ctx, agentType, req)
}

// route 路由决策（关键词 + LLM 混合策略）
func (c *CoordinatorAgent) route(ctx context.Context, message string) (AgentType, error) {
    // 优先使用缓存（零成本）
    if cached := c.routeCache.Get(message); cached != "" {
        return cached, nil
    }
    
    // 快速路由（关键词匹配，零成本）
    if agentType := c.fastRoute(message); agentType != "" {
        c.routeCache.Set(message, agentType)
        return agentType, nil
    }
    
    // 复杂意图用 LLM 判断（仅对模糊请求，成本可控）
    agentType, err := c.llmRoute(ctx, message)
    if err != nil {
        return AgentTypeGeneral, nil  // 降级到父 Agent
    }
    
    c.routeCache.Set(message, agentType)
    return agentType, nil
}

// fastRoute 快速路由（关键词匹配）
func (c *CoordinatorAgent) fastRoute(message string) AgentType {
    // K8s 相关
    k8sKeywords := []string{"pod", "deployment", "service", "k8s", "kubernetes", "容器", "镜像"}
    if containsAnyKeyword(message, k8sKeywords) {
        return AgentTypeK8s
    }
    
    // 数据库相关
    dbKeywords := []string{"sql", "数据库", "慢查询", "query", "table", "索引"}
    if containsAnyKeyword(message, dbKeywords) {
        return AgentTypeDatabase
    }
    
    // 告警相关
    alertKeywords := []string{"告警", "屏蔽", "mute", "alert", "通知"}
    if containsAnyKeyword(message, alertKeywords) {
        return AgentTypeAlert
    }
    
    return ""  // 无法快速判断
}

// llmRoute LLM 路由（仅对模糊请求）
func (c *CoordinatorAgent) llmRoute(ctx context.Context, message string) (AgentType, error) {
    prompt := `分析以下用户消息，判断应该由哪个专家 Agent 处理：
- k8s: Kubernetes 容器相关问题
- database: 数据库查询、慢查询分析
- alert: 告警规则、告警屏蔽
- general: 通用问题、文档查询

用户消息：` + message + `

只返回一个词：k8s/database/alert/general`

    resp, err := c.aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
        Messages: []ai.Message{{Role: "user", Content: prompt}},
        Model:    "gpt-3.5-turbo",  // 路由用便宜模型
    })
    
    if err != nil {
        return "", err
    }
    
    return AgentType(strings.TrimSpace(resp.Choices[0].Message.Content)), nil
}

// handleGeneral 父 Agent 处理通用任务
func (c *CoordinatorAgent) handleGeneral(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 使用父 Agent 的 System Prompt（通用运维助手）
    systemPrompt := `你是夜莺监控系统的运维助手，可以回答：
- 系统使用方法
- 配置说明
- 最佳实践建议

如果用户问题涉及具体的技术操作（K8s、数据库、告警），请引导用户提供更多信息。`

    resp, err := c.aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
        Model:        "gpt-4",
        SystemPrompt: systemPrompt,
        Messages:     req.ConversationHistory,
    })
    
    return c.buildTextResponse(resp.Choices[0].Message.Content), nil
}

// delegateToExpert 委派给专家子 Agent
func (c *CoordinatorAgent) delegateToExpert(ctx context.Context, agentType AgentType, req *ChatRequest) (*ChatResponse, error) {
    expert, exists := c.expertAgents[agentType]
    if !exists {
        return c.handleGeneral(ctx, req)  // 降级
    }
    
    // 构建子 Agent 上下文（传递关键上下文）
    expertReq := &ExpertRequest{
        SessionID: req.SessionID + "_" + string(agentType),
        Message:   req.Message,
        Context:   c.buildExpertContext(req),
        Tools:     expert.Tools,
    }
    
    // 调用子 Agent
    return expert.Execute(ctx, c.aiClient, expertReq)
}

func (c *CoordinatorAgent) buildExpertContext(req *ChatRequest) map[string]interface{} {
    return map[string]interface{}{
        "user_id":       req.UserID,
        "busi_group_id": req.ClientContext.BusiGroupID,
        "timezone":      req.ClientContext.UserTimezone,
        // 只传递必要的上下文，不传整个对话历史（节省 token）
    }
}
```

### 5A.3 子 Agent 定义（专家 Agent）

```go
// center/aiassistant/agent/expert.go
package agent

type ExpertAgent struct {
    Name         string
    SystemPrompt string
    Tools        []Tool
    Model        string
    Temperature  float64
}

type ExpertRequest struct {
    SessionID string
    Message   string
    Context   map[string]interface{}
    Tools     []Tool
}

func (e *ExpertAgent) Execute(ctx context.Context, aiClient ai.AIClient, req *ExpertRequest) (*ChatResponse, error) {
    // 使用专家的 System Prompt 和工具
    resp, err := aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
        Model:        e.Model,
        SystemPrompt: e.SystemPrompt,
        Messages: []ai.Message{
            {Role: "user", Content: req.Message},
        },
        Tools:       e.Tools,
        Temperature: e.Temperature,
    })
    
    if err != nil {
        return nil, err
    }
    
    // 如果有工具调用，处理工具调用
    if resp.ToolCalls != nil && len(resp.ToolCalls) > 0 {
        return handleToolCall(ctx, resp.ToolCalls[0], req.Tools)
    }
    
    return &ChatResponse{
        Status:  "completed",
        Message: resp.Choices[0].Message.Content,
    }, nil
}

// K8s 专家 Agent
func NewK8sExpertAgent() *ExpertAgent {
    return &ExpertAgent{
        Name: "K8s专家",
        SystemPrompt: `你是 Kubernetes 运维专家，擅长：
- Pod 故障诊断（CrashLoopBackOff、ImagePullBackOff、OOMKilled 等）
- 资源调度分析（CPU/内存超限、节点亲和性）
- 配置优化建议（副本数、资源限制、探针配置）

诊断步骤：
1. 先查看 Pod 状态和事件
2. 检查日志（最近 100 行）
3. 分析资源使用情况
4. 给出可操作的修复建议

遵循 Kubernetes 最佳实践。`,
        Tools: []Tool{
            {Name: "k8s_list_pods", Description: "列出 Pod"},
            {Name: "k8s_get_logs", Description: "获取 Pod 日志"},
            {Name: "k8s_describe", Description: "查看资源详情"},
            {Name: "k8s_get_events", Description: "获取事件"},
        },
        Model:       "gpt-4",  // K8s 诊断用 GPT-4
        Temperature: 0.3,      // 更确定性的输出
    }
}

// 数据库专家 Agent
func NewDatabaseExpertAgent() *ExpertAgent {
    return &ExpertAgent{
        Name: "数据库专家",
        SystemPrompt: `你是数据库运维专家，擅长：
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
- 多语句查询需要额外审核`,
        Tools: []Tool{
            {Name: "dbm_sql_check", Description: "SQL 语法检查"},
            {Name: "dbm_sql_query", Description: "执行 SQL 查询"},
            {Name: "dbm_explain", Description: "查看执行计划"},
        },
        Model:       "gpt-3.5-turbo",  // 数据库分析用便宜模型
        Temperature: 0.2,
    }
}

// 告警专家 Agent
func NewAlertExpertAgent() *ExpertAgent {
    return &ExpertAgent{
        Name: "告警专家",
        SystemPrompt: `你是告警管理专家，擅长：
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
- 优先使用 tags 而非 datasource 全量`,
        Tools: []Tool{
            {Name: "alert_mute_preview", Description: "预览屏蔽影响"},
            {Name: "alert_mute_create", Description: "创建屏蔽规则"},
            {Name: "alert_mute_list", Description: "列出屏蔽规则"},
        },
        Model:       "gpt-3.5-turbo",
        Temperature: 0.1,
    }
}
```

### 5A.4 成本优化策略

**1. 路由决策成本（最小化 LLM 调用）**
```go
// 三层路由策略
路由成本：
1. 缓存命中      → 0 成本
2. 关键词匹配    → 0 成本
3. LLM 路由      → GPT-3.5（便宜）

预计 80% 的请求通过前两层，仅 20% 需要 LLM 路由。
```

**2. 子 Agent 模型选择（差异化）**
```go
K8s 专家       → GPT-4（需要复杂推理）
数据库专家     → GPT-3.5（大部分是模式识别）
告警专家       → GPT-3.5（规则生成）
```

**3. 上下文传递优化**
```go
// 只传必要信息，不传整个对话历史
expertContext := map[string]interface{}{
    "user_id":       req.UserID,
    "busi_group_id": req.ClientContext.BusiGroupID,
    // 不包含 conversation_history（节省大量 token）
}
```

### 5A.5 扩展性设计

**新增专家 Agent 极其简单**：

```go
// 新增云服务专家（阿里云/华为云/腾讯云）
func NewCloudExpertAgent() *ExpertAgent {
    return &ExpertAgent{
        Name: "云服务专家",
        SystemPrompt: `你是云服务运维专家，擅长：
- ECS/VM 实例管理
- 负载均衡配置
- 安全组规则优化`,
        Tools: []Tool{
            {Name: "cloud_list_instances", Description: "列出云主机"},
            {Name: "cloud_modify_sg", Description: "修改安全组"},
        },
        Model:       "gpt-3.5-turbo",
        Temperature: 0.2,
    }
}

// 在父 Agent 初始化时注册
coordinator.registerExpert(AgentTypeCloud, NewCloudExpertAgent())

// 在路由函数中添加关键词
cloudKeywords := []string{"ecs", "云主机", "安全组", "负载均衡"}
```

### 5A.6 未来演进：并行子 Agent（DAG 编排）

**当前阶段**：父 Agent 顺序调用单个子 Agent

**未来扩展**（结合第 18.2 节 DAG 编排）：
```yaml
# 复杂巡检任务：并行调用多个子 Agent
workflow:
  name: "全链路巡检"
  tasks:
    - id: k8s_check
      agent: k8s
      parallel: true
      
    - id: db_check
      agent: database
      parallel: true
      
    - id: alert_check
      agent: alert
      parallel: true
      
    - id: summary
      agent: coordinator
      depends_on: [k8s_check, db_check, alert_check]
      action: "汇总所有巡检结果"
```

### 5A.7 与单 Agent 的对比

| 维度 | 单 Agent | 父子 Agent |
|------|----------|-----------|
| 复杂度 | 简单 | 中等 |
| 扩展性 | 差（所有逻辑耦合） | 优（模块化） |
| 成本控制 | 难（统一模型） | 易（按需选择模型） |
| 专业性 | 中（通用 Prompt） | 高（专家 Prompt） |
| 并行能力 | 无 | 可扩展 |
| 维护性 | 差（代码耦合） | 优（职责分离） |

**推荐理由**：父子 Agent 架构在可扩展性、专业性和成本控制上都显著优于单 Agent，适合夜莺这种多领域运维场景。

---



```
┌───────────────────────────────────────────────────────────┐
│                   AI Chat Handler                          │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  LLM Client (统一接口 - pkg/ai)                      │  │
│  │  - OpenAI API                                        │  │
│  │  - Gemini API (via OpenAI 兼容层)                   │  │
│  │  - Azure OpenAI                                      │  │
│  └──────────────────┬──────────────────────────────────┘  │
│                     │                                       │
│                     ▼                                       │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Tool Registry (工具注册表)                          │  │
│  │  - 内置工具 (DBM SQL, Alert Mute, 文件传输)         │  │
│  │  - MCP 工具 (从外部 MCP Server HTTP 获取)           │  │
│  └──────────────────┬──────────────────────────────────┘  │
│                     │                                       │
│                     ▼                                       │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Single Turn Executor                                │  │
│  │  1. 解析意图                                         │  │
│  │  2. 选择工具（单轮最多一个）                         │  │
│  │  3. 参数提取                                         │  │
│  │  4. 风险判定                                         │  │
│  │  5. 执行工具 (或返回 pending_confirmation)           │  │
│  │  6. 格式化响应                                       │  │
│  └──────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────┘
```

**为什么不使用多 Agent**：
- 成本高（多次 LLM 调用）
- 延迟高（Agent 间通信）
- 一致性难保证
- 调试复杂

### 5A.2 实现代码

```go
// center/aiassistant/chat/handler.go
package chat

import (
    "github.com/ccfos/nightingale/v6/pkg/ai"
)

type ChatHandler struct {
    aiClient     ai.AIClient        // 统一的 AI 客户端（pkg/ai）
    toolRegistry *ToolRegistry      // 工具注册表
    riskChecker  *RiskChecker       // 风险判定器
    config       *AIChatConfig
}

func (h *ChatHandler) HandleChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 构建 System Prompt
    systemPrompt := h.buildSystemPrompt(req.ClientContext)
    
    // 2. 准备工具列表（OpenAI Function Calling 格式）
    tools := h.toolRegistry.GetAvailableTools(req.ClientContext)
    
    // 3. 调用 LLM
    llmResp, err := h.aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
        Model: h.config.Model,
        Messages: append(req.ConversationHistory, ai.Message{
            Role:    "user",
            Content: req.Message,
        }),
        Tools:       tools,
        SystemPrompt: systemPrompt,
    })
    
    // 4. 处理 LLM 响应
    if llmResp.ToolCall != nil {
        // 4a. 风险判定
        risk := h.riskChecker.CheckRisk(llmResp.ToolCall)
        if risk == RiskHigh {
            return h.buildPendingConfirmation(llmResp.ToolCall), nil
        }
        
        // 4b. 执行工具（单轮只执行一个）
        toolResult, err := h.toolRegistry.ExecuteTool(ctx, llmResp.ToolCall)
        if err != nil {
            return h.buildErrorResponse(err), nil
        }
        
        // 4c. 格式化返回
        return h.buildCompletedResponse(toolResult), nil
    }
    
    // 5. 纯对话回复（无工具调用）
    return h.buildTextResponse(llmResp.Content), nil
}
```

### 5A.3 Agent 与 MCP Tool 的关系

**明确概念**：
- **Agent**：LLM 驱动的推理层，负责理解意图、选择工具、构造参数
- **MCP Tool**：无状态的功能执行单元，由 Agent 调用

**交互流程**：

```
用户: "检查 prod 环境所有 CrashLoopBackOff 的 pod"
  ↓
Agent (LLM 推理):
  1. 识别环境: prod
  2. 识别目标: pods with status=CrashLoopBackOff
  3. 选择工具: k8s.list_pods
  4. 构造参数: {namespace: "prod", status_filter: "CrashLoopBackOff"}
  ↓
MCP Tool Execution (外部 MCP Server):
  - 调用 K8s API
  - 返回 Pod 列表
  ↓
Agent (格式化输出):
  "在 prod 环境找到 3 个 CrashLoopBackOff 的 pod: ..."
```

---

## 5B. MCP Server 管理（外部服务模式）

### 5B.1 部署模式：统一使用外部服务

**本系统所有 MCP Server 必须独立部署，通过 HTTP/SSE 调用**。

**不使用 stdio 内嵌模式的原因**：
- 进程管理复杂（需监控、重启）
- 资源隔离差（与主进程共享资源）
- 扩展性差（无法水平扩展）
- 运维困难（日志混杂、难以独立升级）

### 5B.2 MCP Server 配置

**数据库表结构**（仅支持 http/sse 类型）：

```sql
CREATE TABLE `mcp_server` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `name` varchar(128) NOT NULL UNIQUE COMMENT 'MCP Server 名称',
  `description` varchar(500) COMMENT '描述',
  
  -- 连接配置（仅 http/sse）
  `server_type` varchar(32) NOT NULL DEFAULT 'http' COMMENT 'http 或 sse',
  `endpoint` varchar(256) NOT NULL COMMENT 'HTTP 端点 URL',
  
  -- 健康检查
  `health_check_url` varchar(256) COMMENT '健康检查 URL（可选，默认 endpoint/health）',
  `health_check_interval` int DEFAULT 60 COMMENT '健康检查间隔（秒）',
  
  -- 安全配置
  `allowed_envs` text COMMENT 'JSON 数组，允许的环境标识',
  `allowed_prefixes` text COMMENT 'JSON 数组，允许的工具前缀',
  
  -- 状态
  `enabled` tinyint(1) DEFAULT 1,
  `health_status` int DEFAULT 0 COMMENT '0=未知 1=健康 2=异常',
  `last_check_time` bigint DEFAULT 0,
  `last_check_error` text,
  
  -- 审计
  `create_at` bigint NOT NULL,
  `create_by` varchar(64) NOT NULL,
  `update_at` bigint NOT NULL,
  `update_by` varchar(64) NOT NULL,
  
  KEY `idx_server_type` (`server_type`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP Server 配置表（仅外部服务）';
```

**配置示例**：

```json
{
  "id": 1,
  "name": "k8s-prod-cluster",
  "server_type": "http",
  "endpoint": "http://k8s-mcp-server.internal:8080",
  "health_check_url": "http://k8s-mcp-server.internal:8080/health",
  "health_check_interval": 60,
  "allowed_envs": ["prod", "staging"],
  "allowed_prefixes": ["k8s_"],
  "enabled": true
}
```

### 5B.3 MCP Server 独立部署

**K8s MCP Server 部署示例**：

```bash
# Docker 部署
docker run -d \
  --name k8s-mcp-server \
  --restart always \
  -p 8080:8080 \
  -v /etc/k8s:/etc/k8s:ro \
  -e KUBECONFIG=/etc/k8s/prod-kubeconfig.yaml \
  -e CLUSTER_NAME=prod-cluster-01 \
  k8s-mcp-server:latest

# 或 K8s Deployment
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-mcp-server
spec:
  replicas: 2
  selector:
    matchLabels:
      app: k8s-mcp-server
  template:
    metadata:
      labels:
        app: k8s-mcp-server
    spec:
      containers:
      - name: server
        image: k8s-mcp-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: KUBECONFIG
          value: /etc/k8s/kubeconfig.yaml
        volumeMounts:
        - name: kubeconfig
          mountPath: /etc/k8s
          readOnly: true
      volumes:
      - name: kubeconfig
        secret:
          secretName: k8s-kubeconfig
---
apiVersion: v1
kind: Service
metadata:
  name: k8s-mcp-server
spec:
  selector:
    app: k8s-mcp-server
  ports:
  - port: 8080
    targetPort: 8080
EOF
```

### 5B.4 夜莺中的 MCP 客户端管理

```go
// center/aiassistant/mcp/manager.go
package mcp

type MCPManager struct {
    clients map[int64]MCPClientInterface
    mu      sync.RWMutex
    ctx     *ctx.Context
}

// 初始化所有外部 MCP Server 连接
func (m *MCPManager) InitMCPClients(ctx context.Context) error {
    servers, err := models.MCPServerGets(m.ctx, "enabled = 1")
    if err != nil {
        return err
    }
    
    for _, server := range servers {
        if server.ServerType != "http" && server.ServerType != "sse" {
            logger.Warnf("unsupported server type: %s, skipped", server.ServerType)
            continue
        }
        
        // 创建 HTTP 客户端
        client := NewHTTPMCPClient(HTTPMCPClientConfig{
            Endpoint:           server.Endpoint,
            HealthCheckURL:     server.HealthCheckURL,
            HealthCheckInterval: server.HealthCheckInterval,
        })
        
        // 健康检查
        if err := client.Health(ctx); err != nil {
            logger.Errorf("MCP server %s health check failed: %v", server.Name, err)
            m.updateHealthStatus(server.ID, 2, err.Error())
            continue
        }
        
        m.mu.Lock()
        m.clients[server.ID] = client
        m.mu.Unlock()
        
        m.updateHealthStatus(server.ID, 1, "")
        
        // 启动定期健康检查
        go m.startHealthCheck(ctx, server.ID, client, server.HealthCheckInterval)
    }
    
    return nil
}

// 定期健康检查
func (m *MCPManager) startHealthCheck(ctx context.Context, serverID int64, client MCPClientInterface, interval int) {
    ticker := time.NewTicker(time.Duration(interval) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := client.Health(ctx); err != nil {
                logger.Errorf("MCP server %d health check failed: %v", serverID, err)
                m.updateHealthStatus(serverID, 2, err.Error())
            } else {
                m.updateHealthStatus(serverID, 1, "")
            }
        }
    }
}

func (m *MCPManager) updateHealthStatus(serverID int64, status int, errMsg string) {
    models.DB(m.ctx).Model(&models.MCPServer{}).
        Where("id = ?", serverID).
        Updates(map[string]interface{}{
            "health_status":    status,
            "last_check_time":  time.Now().Unix(),
            "last_check_error": errMsg,
        })
}
```

### 5B.5 生产部署架构

```
┌──────────────────────────────────────────────────────┐
│               Nightingale 主进程                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  MCP Manager                                   │  │
│  │  - HTTP 客户端连接池                           │  │
│  │  - 定期健康检查                                │  │
│  │  - 工具注册表                                  │  │
│  └────────────────┬───────────────────────────────┘  │
└───────────────────┼──────────────────────────────────┘
                    │ HTTP/SSE
                    ▼
┌────────────────────────────────────────────────────┐
│          外部 MCP Server 集群                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │ K8s MCP      │  │ DB MCP       │  │ 其他 MCP │ │
│  │ - Deployment │  │ - Deployment │  │          │ │
│  │ - Service    │  │ - Service    │  │          │ │
│  └──────────────┘  └──────────────┘  └──────────┘ │
└────────────────────────────────────────────────────┘
```

---

**架构设计**：

```
┌───────────────────────────────────────────────────────────┐
│                   AI Chat Handler                          │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  LLM Client (统一接口)                               │  │
│  │  - OpenAI API                                        │  │
│  │  - Gemini API (via OpenAI 兼容层)                   │  │
│  │  - Azure OpenAI                                      │  │
│  │  - 自托管模型 (Ollama/LocalAI)                       │  │
│  └──────────────────┬──────────────────────────────────┘  │
│                     │                                       │
│                     ▼                                       │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Tool Registry (工具注册表)                          │  │
│  │  - 内置工具 (DBM SQL, Alert Mute, 文件传输)         │  │
│  │  - MCP 工具 (从 MCP Server 动态获取)                │  │
│  └──────────────────┬──────────────────────────────────┘  │
│                     │                                       │
│                     ▼                                       │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Single Turn Executor                                │  │
│  │  1. 解析意图                                         │  │
│  │  2. 选择工具                                         │  │
│  │  3. 参数提取                                         │  │
│  │  4. 风险判定                                         │  │
│  │  5. 执行工具 (或返回 pending_confirmation)           │  │
│  │  6. 格式化响应                                       │  │
│  └──────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────┘
```

**推荐原因**：
- 简单可控，易于调试
- 符合"单轮单工具"约束
- 避免 Agent 之间的协调开销
- 用户体验一致（不会出现"Agent A 说一套，Agent B 说另一套"）

**实现示例**：

```go
// center/aiassistant/chat/handler.go
package chat

type ChatHandler struct {
    llmClient    LLMClient          // 统一的 LLM 客户端
    toolRegistry *ToolRegistry      // 工具注册表
    riskChecker  *RiskChecker       // 风险判定器
    config       *AIChatConfig
}

func (h *ChatHandler) HandleChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 构建 System Prompt
    systemPrompt := h.buildSystemPrompt(req.ClientContext)
    
    // 2. 准备工具列表（OpenAI Function Calling 格式）
    tools := h.toolRegistry.GetAvailableTools(req.ClientContext)
    
    // 3. 调用 LLM
    llmResp, err := h.llmClient.ChatCompletion(ctx, &LLMRequest{
        Model: h.config.Model,
        Messages: append(req.ConversationHistory, Message{
            Role:    "user",
            Content: req.Message,
        }),
        Tools:       tools,
        SystemPrompt: systemPrompt,
    })
    
    // 4. 处理 LLM 响应
    if llmResp.ToolCall != nil {
        // 4a. 风险判定
        risk := h.riskChecker.CheckRisk(llmResp.ToolCall)
        if risk == RiskHigh {
            return h.buildPendingConfirmation(llmResp.ToolCall), nil
        }
        
        // 4b. 执行工具
        toolResult, err := h.toolRegistry.ExecuteTool(ctx, llmResp.ToolCall)
        if err != nil {
            return h.buildErrorResponse(err), nil
        }
        
        // 4c. 格式化返回
        return h.buildCompletedResponse(toolResult), nil
    }
    
    // 5. 纯对话回复
    return h.buildTextResponse(llmResp.Content), nil
}
```

### 5A.2 多 Agent 模式（未来扩展）

**适用场景**：
- **知识库专家** + **运维专家** 分工协作
- **决策 Agent** + **执行 Agent** 两阶段模式
- **主 Agent** + **验证 Agent** 双层检查

**架构草案**（参考 AutoGen/CrewAI 模式）：

```yaml
agents:
  - name: knowledge_expert
    role: "知识库查询专家"
    capabilities: [knowledge_search, document_retrieval]
    llm_config:
      model: gpt-4
      temperature: 0.3
      
  - name: ops_expert
    role: "运维操作专家"
    capabilities: [k8s_tools, dbm_tools, alert_mute]
    llm_config:
      model: gpt-4
      temperature: 0.1
      
  - name: coordinator
    role: "协调者"
    responsibilities:
      - 理解用户意图
      - 分发任务给专家 Agent
      - 汇总结果
```

**问题与挑战**：
1. **成本高**：多次 LLM 调用
2. **延迟高**：Agent 间通信增加响应时间
3. **一致性难保证**：不同 Agent 可能给出矛盾建议
4. **调试复杂**：难以追踪决策链

**推荐策略**：
- **阶段 1-2**：保持单 Agent
- **阶段 3**：仅在复杂场景（如跨系统巡检）引入多 Agent

### 5A.3 Agent 与 MCP Tool 的关系

**关键区别**：
- **Agent**：具有推理能力的 LLM，负责理解意图、规划步骤
- **MCP Tool**：无状态的功能调用，由 Agent 驱动

**交互流程**：

```
用户: "检查 prod 环境所有 CrashLoopBackOff 的 pod"
  ↓
Agent (LLM 推理):
  1. 识别环境: prod
  2. 识别目标: pods with status=CrashLoopBackOff
  3. 选择工具: k8s.list_pods
  4. 构造参数: {namespace: "prod", status_filter: "CrashLoopBackOff"}
  ↓
MCP Tool Execution:
  - 调用 K8s API
  - 返回 Pod 列表
  ↓
Agent (格式化输出):
  "在 prod 环境找到 3 个 CrashLoopBackOff 的 pod: ..."
```

**不要混淆概念**：
- ❌ 错误：把 MCP Server 当作"子 Agent"
- ✅ 正确：MCP Server 提供工具，Agent 调用工具

---

## 5B. MCP Server 管理与启动模式（关键架构决策）

### 5B.1 MCP Server 部署模式对比

| 模式 | 描述 | 优势 | 劣势 | 适用场景 |
|------|------|------|------|----------|
| **模式 A：内嵌启动（npx/uvx）** | 夜莺进程内通过 npx/uvx 启动 MCP Server 子进程 | 部署简单、无外部依赖 | 资源占用高、进程管理复杂 | 开发/测试、小规模部署 |
| **模式 B：外部服务（推荐）** | MCP Server 独立部署，夜莺通过 HTTP/SSE 调用 | 解耦、可扩展、支持多语言 | 需要额外部署 | 生产环境、大规模部署 |
| **模式 C：K8s Sidecar** | MCP Server 作为 Pod 的 Sidecar 容器 | 资源隔离、K8s 原生 | 仅适用于 K8s 环境 | 云原生场景 |

### 5B.2 推荐方案：模式 B（外部服务）+ 模式 A（可选）

**理由**：
1. **K8s MCP Server** 应该独立部署
   - 需要 kubeconfig、集群访问权限
   - 可能需要连接多个集群
   - 资源消耗较高（Go/Python 进程常驻）

2. **轻量级 MCP Server** 可以内嵌
   - 纯计算型工具（如文件格式转换）
   - 无外部依赖
   - 启动快、资源占用低

### 5B.3 夜莺中的 MCP Server 配置

**配置表结构**（第 12 章已定义）：

```sql
CREATE TABLE `mcp_server` (
  `server_type` varchar(32) NOT NULL DEFAULT 'stdio' COMMENT 'stdio/sse/http',
  `command` varchar(256) COMMENT 'stdio 模式启动命令',
  `endpoint` varchar(256) COMMENT 'SSE/HTTP 模式的端点',
  `env` text COMMENT 'JSON 对象，环境变量',
  ...
);
```

**配置示例**：

##### 示例 1：K8s MCP Server（外部服务模式）

```json
{
  "id": 1,
  "name": "k8s-prod-cluster",
  "server_type": "http",
  "endpoint": "http://k8s-mcp-server.internal:8080",
  "env": {
    "KUBECONFIG": "/etc/k8s/prod-kubeconfig.yaml",
    "CLUSTER_NAME": "prod-cluster-01"
  },
  "health_check_url": "http://k8s-mcp-server.internal:8080/health",
  "enabled": true
}
```

**部署方式**：
```bash
# 独立部署 K8s MCP Server
docker run -d \
  --name k8s-mcp-server \
  -p 8080:8080 \
  -v /etc/k8s:/etc/k8s:ro \
  -e KUBECONFIG=/etc/k8s/prod-kubeconfig.yaml \
  k8s-mcp-server:latest
```

##### 示例 2：轻量级工具（内嵌 npx 模式）

```json
{
  "id": 2,
  "name": "json-formatter",
  "server_type": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-json-formatter"],
  "env": {},
  "enabled": true
}
```

**夜莺启动时行为**：
```go
// center/aiassistant/mcp/manager.go
func (m *MCPManager) StartMCPServers(ctx context.Context) error {
    servers, _ := models.MCPServerGets(m.ctx, "enabled = 1")
    
    for _, server := range servers {
        switch server.ServerType {
        case "stdio":
            // 通过 exec.Command 启动子进程
            client := NewStdioMCPClient(server.Command, server.Args, server.Env)
            m.clients[server.ID] = client
            
        case "http", "sse":
            // 不启动进程，仅创建 HTTP 客户端
            client := NewHTTPMCPClient(server.Endpoint)
            m.clients[server.ID] = client
        }
    }
}
```

### 5B.4 MCP Server 生命周期管理

```go
// center/aiassistant/mcp/lifecycle.go
package mcp

type MCPLifecycleManager struct {
    processes map[int64]*exec.Cmd  // stdio 模式的子进程
    clients   map[int64]MCPClientInterface
    mu        sync.RWMutex
}

// 启动所有 MCP Server
func (m *MCPLifecycleManager) StartAll(ctx context.Context) error {
    servers, _ := models.MCPServerGets(ctx, "enabled = 1")
    for _, server := range servers {
        if err := m.Start(ctx, server); err != nil {
            logger.Errorf("failed to start MCP server %s: %v", server.Name, err)
            continue
        }
    }
}

// 启动单个 MCP Server
func (m *MCPLifecycleManager) Start(ctx context.Context, server *models.MCPServer) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    switch server.ServerType {
    case "stdio":
        // 启动子进程
        cmd := exec.CommandContext(ctx, server.Command, parseArgs(server.Args)...)
        cmd.Env = buildEnv(server.Env)
        
        stdin, _ := cmd.StdinPipe()
        stdout, _ := cmd.StdoutPipe()
        
        if err := cmd.Start(); err != nil {
            return err
        }
        
        m.processes[server.ID] = cmd
        m.clients[server.ID] = NewStdioMCPClient(stdin, stdout)
        
        // 监控进程状态
        go m.monitorProcess(server.ID, cmd)
        
    case "http", "sse":
        // 不启动进程，仅健康检查
        client := NewHTTPMCPClient(server.Endpoint)
        if err := client.Health(ctx); err != nil {
            return fmt.Errorf("health check failed: %w", err)
        }
        m.clients[server.ID] = client
    }
    
    return nil
}

// 停止 MCP Server
func (m *MCPLifecycleManager) Stop(serverID int64) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if cmd, exists := m.processes[serverID]; exists {
        // 优雅关闭
        cmd.Process.Signal(syscall.SIGTERM)
        
        // 等待 5 秒
        done := make(chan error)
        go func() { done <- cmd.Wait() }()
        
        select {
        case <-done:
            // 正常退出
        case <-time.After(5 * time.Second):
            // 强制 kill
            cmd.Process.Kill()
        }
        
        delete(m.processes, serverID)
        delete(m.clients, serverID)
    }
    
    return nil
}

// 重启 MCP Server
func (m *MCPLifecycleManager) Restart(ctx context.Context, serverID int64) error {
    server, _ := models.MCPServerGetById(ctx, serverID)
    m.Stop(serverID)
    return m.Start(ctx, server)
}

// 监控子进程状态
func (m *MCPLifecycleManager) monitorProcess(serverID int64, cmd *exec.Cmd) {
    if err := cmd.Wait(); err != nil {
        logger.Errorf("MCP server %d exited: %v", serverID, err)
        
        // 标记为不健康
        models.DB(m.ctx).Model(&models.MCPServer{}).
            Where("id = ?", serverID).
            Updates(map[string]interface{}{
                "health_status":    2,
                "last_check_error": err.Error(),
            })
        
        // 可选：自动重启
        if m.config.AutoRestart {
            time.Sleep(5 * time.Second)
            server, _ := models.MCPServerGetById(m.ctx, serverID)
            m.Start(m.ctx, server)
        }
    }
}
```

### 5B.5 推荐的 MCP Server 部署架构

**生产环境推荐**：

```
┌──────────────────────────────────────────────────────┐
│               Nightingale 主进程                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  MCP Lifecycle Manager                         │  │
│  │  - 管理轻量级 stdio MCP (npx/uvx)              │  │
│  │  - HTTP 客户端连接外部 MCP Server               │  │
│  └────────────────────────────────────────────────┘  │
└────────────┬─────────────────────┬───────────────────┘
             │                     │
             │ HTTP/SSE            │ stdio (可选)
             ▼                     ▼
┌────────────────────────┐  ┌──────────────────┐
│  K8s MCP Server Pod    │  │ 轻量级 MCP 子进程 │
│  - Deployment 部署     │  │ - JSON 格式化     │
│  - Service 暴露        │  │ - 文本处理        │
│  - 多集群 kubeconfig   │  └──────────────────┘
└────────────────────────┘
```

**推荐策略**：
1. **重量级 MCP**（K8s、DB、监控系统）→ 外部服务
2. **轻量级 MCP**（工具类）→ stdio 内嵌启动（可选）
3. **默认建议**：全部使用外部服务，便于运维和扩展

---

## 5C. AI 模型调用整合方案（复用 aisummary 基础设施）

### 5C.1 现有 aisummary 模块架构回顾

根据 `08-AI处理模块详解.md`，已有组件：

```
alert/pipeline/processor/aisummary/
├── ai_summary.go           # AI 摘要处理器
├── callback/callback.go    # HTTP 配置基类
└── common/common.go        # 通用初始化逻辑
```

**核心能力**：
- ✅ HTTP 客户端管理（SSL、代理、超时）
- ✅ OpenAI 兼容 API 调用
- ✅ 模板引擎集成（`tplx.TemplateFuncMap`）
- ✅ 参数类型转换（`convertCustomParam`）

### 5C.2 整合架构设计

**方案：提取共享层 + 各模块独立调用**

```
┌─────────────────────────────────────────────────────────┐
│           共享层：pkg/ai/client.go (新建)                │
│  ┌───────────────────────────────────────────────────┐  │
│  │  type AIClient interface {                        │  │
│  │    ChatCompletion(req) (*ChatResponse, error)     │  │
│  │    StreamCompletion(req) (chan Delta, error)      │  │
│  │  }                                                 │  │
│  │                                                    │  │
│  │  type OpenAIClient struct { ... }  // 实现类      │  │
│  │  - 复用 callback.HTTPConfig                       │  │
│  │  - 复用 convertCustomParam 逻辑                   │  │
│  └───────────────────────────────────────────────────┘  │
└────────────┬────────────────────────────┬───────────────┘
             │                            │
             ▼                            ▼
┌────────────────────────┐    ┌──────────────────────────┐
│  alert/pipeline/       │    │  center/aiassistant/     │
│    processor/aisummary │    │    chat/handler.go       │
│  - 继续使用原逻辑       │    │  - 调用共享 AIClient     │
│  - 零改动               │    │  - 新增工具调用能力      │
└────────────────────────┘    └──────────────────────────┘
```

### 5C.3 共享 AI 客户端实现

```go
// pkg/ai/client.go (新建)
package ai

import (
    "github.com/ccfos/nightingale/v6/alert/pipeline/processor/callback"
)

// AIClient 统一的 AI 模型调用接口
type AIClient interface {
    ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
    StreamCompletion(ctx context.Context, req *ChatCompletionRequest) (chan StreamDelta, error)
}

// OpenAIClient 实现（复用 aisummary 的 HTTP 配置）
type OpenAIClient struct {
    callback.HTTPConfig               // 复用现有 HTTP 客户端
    Model      string                `json:"model"`
    APIKey     string                `json:"api_key"`
    BaseURL    string                `json:"base_url"`
    CustomParams map[string]interface{} `json:"custom_params,omitempty"`
}

func NewOpenAIClient(config OpenAIClientConfig) (*OpenAIClient, error) {
    client := &OpenAIClient{
        HTTPConfig: callback.HTTPConfig{
            URL:           config.BaseURL,
            Timeout:       config.Timeout,
            SkipSSLVerify: config.SkipSSLVerify,
            Proxy:         config.Proxy,
        },
        Model:        config.Model,
        APIKey:       config.APIKey,
        BaseURL:      config.BaseURL,
        CustomParams: config.CustomParams,
    }
    
    // 初始化 HTTP 客户端（复用 aisummary 逻辑）
    client.HTTPConfig.InitClient()
    
    return client, nil
}

func (c *OpenAIClient) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
    // 构建请求体
    body := map[string]interface{}{
        "model":    c.Model,
        "messages": req.Messages,
    }
    
    // 合并自定义参数（复用 aisummary 的参数转换）
    for k, v := range c.CustomParams {
        body[k] = convertCustomParam(v)
    }
    
    // 添加工具定义（如果有）
    if len(req.Tools) > 0 {
        body["tools"] = req.Tools
        body["tool_choice"] = "auto"
    }
    
    // 发送请求（复用 HTTP 客户端）
    jsonData, _ := json.Marshal(body)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
    
    resp, err := c.Client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // 解析响应
    var chatResp ChatCompletionResponse
    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        return nil, err
    }
    
    return &chatResp, nil
}

// convertCustomParam 从 aisummary 复用
func convertCustomParam(v interface{}) interface{} {
    // ... 与 aisummary/ai_summary.go 中相同的实现
}
```

### 5C.4 AI 助手调用共享客户端

```go
// center/aiassistant/chat/handler.go
package chat

import (
    "github.com/ccfos/nightingale/v6/pkg/ai"
)

type ChatHandler struct {
    aiClient ai.AIClient  // 使用共享 AI 客户端
    // ...
}

func NewChatHandler(config *Config) (*ChatHandler, error) {
    // 使用共享的 AI 客户端工厂
    aiClient, err := ai.NewOpenAIClient(ai.OpenAIClientConfig{
        Model:         config.AI.Model,
        APIKey:        config.AI.APIKey,
        BaseURL:       config.AI.BaseURL,
        Timeout:       config.AI.Timeout,
        SkipSSLVerify: config.AI.SkipSSLVerify,
        CustomParams:  config.AI.CustomParams,
    })
    
    return &ChatHandler{
        aiClient: aiClient,
    }, nil
}

func (h *ChatHandler) HandleChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 构建工具定义（OpenAI Function Calling 格式）
    tools := h.buildToolDefinitions()
    
    // 调用共享 AI 客户端
    resp, err := h.aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
        Messages: req.Messages,
        Tools:    tools,
    })
    
    // 处理响应
    if resp.ToolCalls != nil {
        return h.handleToolCall(ctx, resp.ToolCalls[0])
    }
    
    return h.buildTextResponse(resp.Choices[0].Message.Content), nil
}
```

### 5C.5 aisummary 模块保持不变

```go
// alert/pipeline/processor/aisummary/ai_summary.go
// 无需修改，继续使用原有逻辑

type AISummaryConfig struct {
    callback.HTTPConfig                    // 继续复用
    ModelName      string                 
    APIKey         string                 
    PromptTemplate string                 
    CustomParams   map[string]interface{} 
}

// 原有方法不变
func (c *AISummaryConfig) Process(ctx *ctx.Context, event *models.AlertCurEvent) (*models.AlertCurEvent, string, error) {
    // ... 保持原有实现
}
```

### 5C.6 整合方案总结

| 模块 | AI 调用方式 | 是否改动 | 说明 |
|------|------------|----------|------|
| `aisummary` | 直接使用 callback.HTTPConfig | ❌ 零改动 | 继续独立运行 |
| `ai-assistant` | 使用共享 `pkg/ai.AIClient` | ✅ 新模块 | 新增工具调用能力 |
| 共享层 `pkg/ai` | 提取公共逻辑 | ✅ 新建 | 复用 HTTP 客户端、参数转换 |

**优势**：
- aisummary 模块零改动，不影响现有告警流程
- 新模块复用成熟的 HTTP 客户端和参数处理
- 未来可以让 aisummary 也迁移到共享客户端（可选）

 6. 通用返回结构与链路约定
 6.1 Headers（前端自动附带）
- `Authorization: Bearer <token>`
- `X-Language: zh_CN/en_US/...`
 6.2 统一响应结构（对齐前端 `request`）
页面 API 推荐统一为：
{ "dat": {}, "error": "" }
6.3 trace_id / session_id
- 每次请求必须返回 trace_id
- chat/knowledge/mcp 交互必须贯穿 session_id
7. 高风险判定（Deterministic）与二次确认
7.1 风险等级
- low：只读、范围明确、影响可控
- medium：只读但范围大/可能敏感/成本高；可配置是否需要确认
- high：改变系统状态/影响范围大/不可逆风险；必须二次确认
7.2 通用高风险规则（任何工具）
满足任一条 => high：
- 写操作/状态变更（create/update/delete/exec/scale/put_fields/push-config）
- 目标范围无法收敛（缺 namespace、缺资源名、tags 为空、all 语义）
- 批量语义（多 ids、多目标、通配符）
- 单轮尝试多个工具调用（违反“单轮单工具”）
- env/prefix/ip 不匹配（直接拒绝更合适）
7.3 SQL 风险判定（DBM）
确定性分类（不依赖模型“理解”）：
- 只读：SELECT/SHOW/EXPLAIN/DESC/WITH...SELECT
- 写/危险：INSERT/UPDATE/DELETE/REPLACE/TRUNCATE/ALTER/DROP/CREATE/GRANT/REVOKE/SET/CALL/LOAD DATA/...
- 多语句：包含多个非空语句（; 分割后多段）
判定建议：
- 写/危险关键字 => high（必须二次确认）
- 多语句 => high
- SELECT 无 LIMIT 且未给 limit_num => medium（可自动补默认 limit 或要求确认）
- EXPLAIN/DESC/SHOW => low
执行策略（必须落实）：
- high：check -> pending_confirmation -> execute
- 工具错误：HTTP 200 + dat.status="error" + dat.tool.error，顶层 err=""
7.4 告警屏蔽（Alert Mute）风险判定
范围过宽 => high（必确认）：
- tags 为空/约束不足/正则过宽（如 .*）
- datasource 全量（含 0 或等价 all）
- severities 全覆盖且 tags 约束不足
- prod/cate 不明确
- 时间跨度超阈值（例如 > 24h，可配置）
- 周期性静音“全周+全天”近似全量
操作类型 => high：
- 删除规则
- 批量 fields 更新
- 批量启停
建议策略：
- 创建/更新前调用 preview，匹配事件数过阈值（如 >50）可升级 pending_confirmation
7.5 K8s MCP 风险判定
- exec、scale：high（必确认，必须指定 cluster/namespace/name）
- get_logs：medium（需限制行数/大小，可能敏感）
- list/get/describe：low（namespace=all 或跨多 namespace 可升 medium）
- 删除类工具：默认禁用
7.6 MCP 管理风险判定
以下一律 high 且建议仅 Admin：
- MCP Server 配置 CRUD
- push-config 远程配置下发
- 模板商店修改/删除（新增可按策略放宽，但仍建议确认）
7.7 二次确认协议（前后端统一）
当风险为 high，后端返回：
- dat.status = "pending_confirmation"
- dat.pending_confirmation 固定结构（前端按此渲染确认卡片）
- 确认通过后，前端在下一次 POST /chat 携带：
  - confirmation.confirm_id
  - confirmation.action = "approve"
8. Chat API：POST /api/${N9E_PATHNAME}/ai-assistant/chat
8.1 功能要求（硬约束）
- 多轮对话：支持 session_id
- 文件：支持 file_id；图片可转 base64 多模态输入（模型支持时）
- 单轮最多一个工具调用
- 高风险：返回 pending_confirmation，确认后执行
- 工具错误：结构化完整透传（不丢细节）
8.2 Request（建议）
{
  session_id: ses_xxx,
  mode: chat,
  message: 帮我查询实例 3 的 db1：select * from t limit 10,
  attachments: [
    { type: image, file_id: file_xxx, mime_type: image/png }
  ],
  client_context: {
    busi_group_id: 1,
    user_timezone: Asia/Shanghai,
    ui_language: zh_CN,
    env: prod
  },
  confirmation: { confirm_id: confirm_xxx, action: approve }
}
8.3 Response（注意：工具错误不走顶层 err）
completed
{
  dat: {
    trace_id: t_xxx,
    session_id: ses_xxx,
    status: completed,
    assistant_message: { format: markdown, content: ... },
    tool: { called: true, name: dbm.sql_query, request: {}, result: {} }
  },
  error: ""
}
pending_confirmation
{
  dat: {
    trace_id: t_xxx,
    session_id: ses_xxx,
    status: pending_confirmation,
    assistant_message: { format: markdown, content: 检测到高风险操作，请确认... },
    pending_confirmation: {
      confirm_id: confirm_xxx,
      risk_level: high,
      summary: 将执行高风险操作,
      proposed_tool: { name: dbm.sql_query, request: {} },
      check_result: { name: dbm.sql_check, result: {} },
      expires_at: 1730000000
    }
  },
  error: ""
}
error（工具级错误，依旧 HTTP200 且 err=""）
{
  dat: {
    trace_id: t_xxx,
    session_id: ses_xxx,
    status: error,
    assistant_message: { format: markdown, content: 执行失败，原因如下... },
    tool: {
      called: true,
      name: dbm.sql_query,
      request: {},
      error: {
        code: UPSTREAM_ERROR,
        message: Access denied ...,
        raw: 原始错误（必要时脱敏）
      }
    }
  },
  error: ""
}
9. 文件：上传与下载（对齐夜莺“可直接下载链接”模式）
9.1 上传：POST /api/${N9E_PATHNAME}/ai-assistant/chat/upload
- multipart/form-data
- 响应字段必须稳定（前端依赖兼容）：
{
  dat: {
    file_id: file_xxx,
    file_name: a.png,
    mime_type: image/png,
    size: 12345,
    sha256: ....,
    expires_at: 1730000000
  },
  error: ""
}
9.2 下载：GET /api/${N9E_PATHNAME}/ai-assistant/files/download/{path}
由于浏览器直接下载不会带 Authorization Header，本模块规定下载必须返回可直接点击的链接：
- 后端在工具输出或上传响应中提供：
  - download_url（相对路径即可，前端直接作为 href）
  - download_token（短期有效，一次性或短 TTL）
示例：
{
  download_url: /api/n9e/ai-assistant/files/download/obj/abc123?token=dl_xxx,
  expires_at: 1730000000
}
强制安全要求：
- {path} 必须是受控对象 key（不能是任意文件系统路径）
- 防路径穿越：拒绝 ..、绝对路径、盘符等
- 下载审计：user/trace/session/ip/资源摘要
9.3 file_transfer 工具复用
文件相关能力统一通过 file_transfer 语义复用，避免重复实现与多套兼容字段。

10. 会话、统计、删除与归档
   10.1 Redis 存储（建议）
   Key 设计示例：

- ai_assistant:session:{session_id}:messages（List/ZSet）
- ai_assistant:session:{session_id}:meta（Hash）
- ai_assistant:active_sessions（ZSet，score=last_active_at）
- ai_assistant:confirm:{confirm_id}（Hash + TTL）
  策略建议：
- 活跃会话 TTL：7~30 天（可配置）
- 消息条数上限：如 2000（超出滚动裁剪）
- 大输出落地：对象存储/受控文件，并在消息中引用下载链接
  10.2 活跃会话统计：GET /api/n9e/ai-assistant/sessions/stats
  建议字段：
- active_count
- per_mode_count（chat/knowledge/mcp）
- last24h_created / last24h_active
- storage_estimate（可选）
  10.3 删除会话：DELETE /api/n9e/ai-assistant/sessions/{session_id}
- 校验：仅 Admin 或会话所有者可删（建议）
- 删除会话相关 Redis keys
- 写审计：谁删的、何时、原因（可选）
  10.4 归档
- 定时归档任务：按不活跃阈值归档（如 30 天）
- 手动归档：POST /api/n9e/ai-assistant/sessions/archive
- 归档内容：meta + message（脱敏版）+ tool call 记录 + trace 信息

## 11. RBAC 权限点定义（必须与 `center/cconf/ops.go` 同步）

### 11.1 权限点规划

参考夜莺既有 RBAC 模式与 DBM 权限设计，AI 助手模块需新增以下权限点：

```go
// center/cconf/ops.go 中需要添加
var aiAssistantOps = []Operation{
    {Name: "/ai-assistant", Ops: []string{"GET"}},                    // 只读权限（查看会话、统计）
    {Name: "/ai-assistant/chat", Ops: []string{"POST"}},              // 对话权限
    {Name: "/ai-assistant/knowledge", Ops: []string{"POST"}},         // 知识库查询权限
    {Name: "/ai-assistant/files", Ops: []string{"GET", "POST"}},      // 文件上传下载
    {Name: "/ai-assistant/sessions/write", Ops: []string{"DELETE"}},  // 会话删除权限
    {Name: "/ai-assistant/mcp/admin", Ops: []string{"POST", "PUT", "DELETE"}}, // MCP 管理（仅 Admin）
}
```

### 11.2 权限分级说明

| 权限点 | 角色 | 说明 |
|--------|------|------|
| `/ai-assistant` (GET) | Standard, Admin | 查看会话列表、统计信息 |
| `/ai-assistant/chat` (POST) | Standard, Admin | 发起 AI 对话、工具调用 |
| `/ai-assistant/knowledge` (POST) | Standard, Admin | 查询知识库 |
| `/ai-assistant/files` (GET/POST) | Standard, Admin | 上传文件、下载结果 |
| `/ai-assistant/sessions/write` (DELETE) | Admin | 删除会话（普通用户只能删自己的） |
| `/ai-assistant/mcp/admin` (POST/PUT/DELETE) | **仅 Admin** | MCP Server 配置、模板管理 |

### 11.3 菜单配置（前端 `menu.tsx`）

```typescript
// fe/src/components/SideMenu/menu.tsx
{
  key: 'ai-assistant',
  label: 'menu.ai_assistant',
  icon: <RobotOutlined />,
  role: ['Admin'], // Admin 专属菜单，解决 Admin 无 perms 数据的问题
  type: 'tabs',
  children: [
    { key: '/ai-assistant/chat', label: 'menu.ai_assistant.chat' },
    { key: '/ai-assistant/knowledge', label: 'menu.ai_assistant.knowledge' },
    { key: '/ai-assistant/mcp/servers', label: 'menu.ai_assistant.mcp_servers' },
    { key: '/ai-assistant/mcp/tools', label: 'menu.ai_assistant.mcp_tools' },
    { key: '/ai-assistant/sessions', label: 'menu.ai_assistant.sessions' },
  ],
}
```

**注意**：参照 `AGENTS.md` 第 1.2 节，Admin 专属菜单必须添加 `role: ['Admin']` 属性。

---

## 12. 数据库表结构设计

### 12.1 MCP Server 配置表

```sql
CREATE TABLE `mcp_server` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `name` varchar(128) NOT NULL UNIQUE COMMENT 'MCP Server 名称',
  `description` varchar(500) COMMENT '描述',
  
  -- 连接配置（仅 http/sse）
  `server_type` varchar(32) NOT NULL DEFAULT 'http' COMMENT 'http 或 sse',
  `endpoint` varchar(256) NOT NULL COMMENT 'HTTP 端点 URL',
  
  -- 健康检查
  `health_check_url` varchar(256) COMMENT '健康检查 URL（可选，默认 endpoint/health）',
  `health_check_interval` int DEFAULT 60 COMMENT '健康检查间隔（秒）',
  
  -- 安全配置
  `allowed_envs` text COMMENT 'JSON 数组，允许的环境标识 ["prod", "test"]',
  `allowed_prefixes` text COMMENT 'JSON 数组，允许的工具前缀 ["k8s_", "db_"]',
  `allowed_ips` text COMMENT 'JSON 数组，允许的 IP 范围（可选）',
  
  -- 状态
  `enabled` tinyint(1) DEFAULT 1,
  `health_status` int DEFAULT 0 COMMENT '0=未知 1=健康 2=异常',
  `last_check_time` bigint DEFAULT 0,
  `last_check_error` text,
  
  -- 审计
  `create_at` bigint NOT NULL,
  `create_by` varchar(64) NOT NULL,
  `update_at` bigint NOT NULL,
  `update_by` varchar(64) NOT NULL,
  
  KEY `idx_server_type` (`server_type`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP Server 配置表';
```

### 12.2 MCP 工具模板表

```sql
CREATE TABLE `mcp_template` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `name` varchar(128) NOT NULL UNIQUE COMMENT '模板名称',
  `description` varchar(500),
  
  -- 模板内容
  `server_config` text NOT NULL COMMENT 'JSON，MCP Server 配置模板',
  `category` varchar(64) DEFAULT 'custom' COMMENT '分类：k8s/db/monitor/custom',
  
  -- 默认模板标记
  `is_default` tinyint(1) DEFAULT 0 COMMENT '是否为系统默认模板',
  `is_public` tinyint(1) DEFAULT 0 COMMENT '是否公开可见',
  
  -- 审计
  `create_at` bigint NOT NULL,
  `create_by` varchar(64) NOT NULL,
  `update_at` bigint NOT NULL,
  `update_by` varchar(64) NOT NULL,
  
  KEY `idx_category` (`category`),
  KEY `idx_is_default` (`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP 模板商店';
```

### 12.3 会话归档表（可选，持久化归档）

```sql
CREATE TABLE `ai_assistant_session_archive` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `session_id` varchar(64) NOT NULL,
  `user_id` bigint NOT NULL,
  
  -- 会话元数据
  `mode` varchar(32) COMMENT 'chat/knowledge/mcp',
  `message_count` int DEFAULT 0,
  `tool_call_count` int DEFAULT 0,
  `first_message_at` bigint,
  `last_message_at` bigint,
  
  -- 归档内容（压缩存储）
  `messages` longtext COMMENT 'JSON 数组（脱敏后）',
  `tool_calls` text COMMENT 'JSON 数组，工具调用记录',
  `trace_ids` text COMMENT 'JSON 数组，关联的 trace_id',
  
  -- 归档信息
  `archived_at` bigint NOT NULL,
  `archived_by` varchar(64),
  `archive_reason` varchar(128) COMMENT 'manual/auto_expired/user_deleted',
  
  KEY `idx_session_id` (`session_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_archived_at` (`archived_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 助手会话归档表';
```

**说明**：会话归档表用于长期存储已归档的会话，支持审计和数据分析。

### 12.3.1 AI 配置表（核心配置从数据库读取）

**所有 AI 相关配置从数据库读取，支持动态修改，无需重启服务。**

```sql
CREATE TABLE `ai_config` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `config_key` varchar(128) NOT NULL UNIQUE COMMENT '配置项 key',
  `config_value` text NOT NULL COMMENT '配置值（JSON 格式）',
  `config_type` varchar(32) NOT NULL COMMENT '配置类型：ai_model/knowledge/session/file/general',
  `description` varchar(500) COMMENT '配置说明',
  
  -- 作用范围
  `scope` varchar(32) DEFAULT 'global' COMMENT 'global/busi_group',
  `scope_id` bigint DEFAULT 0 COMMENT '业务组 ID（scope=busi_group 时有效）',
  
  -- 状态
  `enabled` tinyint(1) DEFAULT 1,
  
  -- 审计
  `create_at` bigint NOT NULL,
  `create_by` varchar(64) NOT NULL,
  `update_at` bigint NOT NULL,
  `update_by` varchar(64) NOT NULL,
  
  KEY `idx_config_type` (`config_type`),
  KEY `idx_scope` (`scope`, `scope_id`),
  KEY `idx_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 助手配置表';
```

**预置配置项**（初始化 SQL）：

```sql
-- 默认 AI 模型配置
INSERT INTO `ai_config` (`config_key`, `config_value`, `config_type`, `description`, `scope`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
('ai.default_model', '{"provider":"openai","base_url":"https://api.openai.com/v1","api_key":"${OPENAI_API_KEY}","model":"gpt-4","temperature":0.7,"max_tokens":2000}', 'ai_model', '默认 AI 模型配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- K8s 专家配置
('ai.expert.k8s', '{"model":"gpt-4","temperature":0.3}', 'ai_model', 'K8s 专家 Agent 模型配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 数据库专家配置
('ai.expert.database', '{"model":"gpt-3.5-turbo","temperature":0.2}', 'ai_model', '数据库专家 Agent 模型配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 告警专家配置
('ai.expert.alert', '{"model":"gpt-3.5-turbo","temperature":0.1}', 'ai_model', '告警专家 Agent 模型配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 知识库配置
('knowledge.provider', '{"type":"coze","base_url":"https://api.coze.com","api_key":"${COZE_API_KEY}","default_bot_id":""}', 'knowledge', 'Coze 知识库配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 会话管理配置
('session.config', '{"ttl":604800,"max_messages_per_session":2000,"max_sessions_per_user":50}', 'session', '会话管理配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 确认机制配置
('confirmation.config', '{"ttl":300,"cleanup_interval":60}', 'general', '二次确认配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 文件管理配置
('file.config', '{"max_size":10485760,"storage_backend":"local","storage_path":"./data/ai_files","ttl":86400}', 'file', '文件管理配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 归档策略配置
('archive.config', '{"enabled":true,"inactive_threshold":2592000,"cron_schedule":"0 2 * * *"}', 'general', '归档策略配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 工具调用配置
('tool.config', '{"call_timeout":30000,"max_calls_per_session":1000}', 'general', '工具调用配置', 'global', UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system');
```

**配置管理 API**：

```
GET    /api/n9e/ai-assistant/config          # 获取所有配置
GET    /api/n9e/ai-assistant/config/:key     # 获取单个配置
PUT    /api/n9e/ai-assistant/config/:key     # 更新配置
POST   /api/n9e/ai-assistant/config/reload   # 重新加载配置（刷新缓存）
```

### 12.4 数据库迁移（参考 DBM 模式）

在 `models/migrate.go` 中添加：

```go
func migrateMCPTables(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.MCPServer{},
        &models.MCPTemplate{},
        &models.AIAssistantSessionArchive{},
    )
}
```

---

## 13. 复用现有 AI 基础设施（避免重复造轮子）

### 13.1 复用 `callback.HTTPConfig`

参考 `alert/pipeline/processor/callback/callback.go`，AI 助手的 AI 模型调用应复用现有 HTTP 配置：

```go
// 继承现有 HTTPConfig
type AIClientConfig struct {
    callback.HTTPConfig                    // 复用 HTTP 客户端、超时、代理配置
    ModelName      string                 `json:"model_name"`
    APIKey         string                 `json:"api_key"`
    CustomParams   map[string]interface{} `json:"custom_params"`
}

// 复用初始化逻辑
func (c *AIClientConfig) initHTTPClient() {
    // 直接调用 callback.HTTPConfig 的初始化方法
    c.HTTPConfig.InitClient()
}
```

**优势**：
- 共享 SSL 验证、代理配置、超时设置
- 避免重复实现 HTTP 客户端管理
- 与现有告警处理器保持一致

### 13.2 复用模板引擎 `tplx.TemplateFuncMap`

参考 `alert/pipeline/processor/aisummary/ai_summary.go` 的模板系统：

```go
import "github.com/ccfos/nightingale/v6/pkg/tplx"

// 生成 Prompt 时复用现有模板函数
tmpl, err := template.New("prompt").Funcs(tplx.TemplateFuncMap).Parse(promptTemplate)
```

**可用函数**：
- `timeformat` - 时间格式化
- `timestamp` - 时间戳转换
- `humanize` - 数值人性化显示
- 其他自定义函数见 `pkg/tplx/tpl.go`

### 13.3 复用参数转换逻辑

直接使用 `aisummary` 模块的 `convertCustomParam` 函数：

```go
// alert/pipeline/processor/aisummary/ai_summary.go
func convertCustomParam(v interface{}) interface{} {
    // 智能转换字符串参数为正确类型
    // "123" → int64(123)
    // "true" → bool(true)
    // "[1,2,3]" → []interface{}
}
```

---

## 14. MCP 客户端扩展接口定义（基于 JSON-RPC 2.0）

### 14.1 MCP 协议规范

**Model Context Protocol (MCP)** 采用 **JSON-RPC 2.0** 作为通信基础。

**核心特点**：
- 所有消息遵循 JSON-RPC 2.0 规范
- 三种消息类型：Request、Response、Notification
- 传输层：HTTP/SSE（本系统仅使用 HTTP）

**JSON-RPC 消息格式**：

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}

// Response（成功）
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": { ... }
}

// Response（错误）
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request"
  }
}
```

**注意**：与标准 JSON-RPC 2.0 的区别是，MCP 要求 `id` **不能为 null**。

### 14.2 MCP 核心方法

#### 14.2.1 tools/list - 列出工具

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {
    "cursor": "optional_pagination_cursor"
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "k8s_list_pods",
        "description": "List pods in a namespace",
        "inputSchema": {
          "type": "object",
          "properties": {
            "namespace": {
              "type": "string",
              "description": "Kubernetes namespace"
            }
          },
          "required": ["namespace"]
        }
      }
    ],
    "nextCursor": "optional_next_page_cursor"
  }
}
```

#### 14.2.2 tools/call - 调用工具

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "k8s_list_pods",
    "arguments": {
      "namespace": "prod",
      "label_selector": "app=nginx"
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found 3 pods:\n1. nginx-7d6c8f..."
      }
    ],
    "isError": false
  }
}
```

### 14.3 MCPClient 接口定义

```go
// center/aiassistant/mcp/interface.go
package mcp

import "context"

// MCPClientInterface 定义 MCP 客户端标准接口
type MCPClientInterface interface {
    // 健康检查
    Health(ctx context.Context) error
    
    // 列出可用工具（JSON-RPC: tools/list）
    ListTools(ctx context.Context) ([]Tool, error)
    
    // 调用工具（JSON-RPC: tools/call）
    CallTool(ctx context.Context, req *ToolRequest) (*ToolResponse, error)
    
    // 关闭连接
    Close() error
}

// Tool 定义工具结构（对应 MCP tools/list 返回）
type Tool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"inputSchema"`  // JSON Schema
}

// ToolRequest 定义工具调用请求
type ToolRequest struct {
    Name      string                 `json:"name"`
    Arguments map[string]interface{} `json:"arguments"`
    TraceID   string                 `json:"trace_id,omitempty"`
    SessionID string                 `json:"session_id,omitempty"`
}

// ToolResponse 定义工具调用响应
type ToolResponse struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError"`
}

type ContentBlock struct {
    Type string `json:"type"`  // text/image/resource
    Text string `json:"text,omitempty"`
    Data string `json:"data,omitempty"`  // base64 编码
}
```

### 14.4 HTTP MCP Client 实现

```go
// center/aiassistant/mcp/http_client.go
package mcp

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync/atomic"
    "time"
)

type HTTPMCPClient struct {
    endpoint   string
    httpClient *http.Client
    requestID  int64  // 自增的 JSON-RPC 请求 ID
}

type HTTPMCPClientConfig struct {
    Endpoint           string
    HealthCheckURL     string
    HealthCheckInterval int
    Timeout            time.Duration
}

func NewHTTPMCPClient(config HTTPMCPClientConfig) *HTTPMCPClient {
    if config.Timeout == 0 {
        config.Timeout = 30 * time.Second
    }
    
    return &HTTPMCP Client{
        endpoint: config.Endpoint,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
    }
}

// ListTools 实现 tools/list（JSON-RPC 方法）
func (c *HTTPMCPClient) ListTools(ctx context.Context) ([]Tool, error) {
    reqID := atomic.AddInt64(&c.requestID, 1)
    
    request := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      reqID,
        "method":  "tools/list",
        "params":  map[string]interface{}{},
    }
    
    var response struct {
        JSONRPC string `json:"jsonrpc"`
        ID      int64  `json:"id"`
        Result  *struct {
            Tools      []Tool `json:"tools"`
            NextCursor string `json:"nextCursor,omitempty"`
        } `json:"result,omitempty"`
        Error *JSONRPCError `json:"error,omitempty"`
    }
    
    if err := c.doJSONRPCRequest(ctx, request, &response); err != nil {
        return nil, err
    }
    
    if response.Error != nil {
        return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
    }
    
    if response.Result == nil {
        return nil, fmt.Errorf("empty result")
    }
    
    return response.Result.Tools, nil
}

// CallTool 实现 tools/call（JSON-RPC 方法）
func (c *HTTPMCPClient) CallTool(ctx context.Context, req *ToolRequest) (*ToolResponse, error) {
    reqID := atomic.AddInt64(&c.requestID, 1)
    
    request := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      reqID,
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name":      req.Name,
            "arguments": req.Arguments,
        },
    }
    
    var response struct {
        JSONRPC string `json:"jsonrpc"`
        ID      int64  `json:"id"`
        Result  *struct {
            Content []ContentBlock `json:"content"`
            IsError bool           `json:"isError"`
        } `json:"result,omitempty"`
        Error *JSONRPCError `json:"error,omitempty"`
    }
    
    if err := c.doJSONRPCRequest(ctx, request, &response); err != nil {
        return nil, err
    }
    
    if response.Error != nil {
        return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
    }
    
    if response.Result == nil {
        return nil, fmt.Errorf("empty result")
    }
    
    return &ToolResponse{
        Content: response.Result.Content,
        IsError: response.Result.IsError,
    }, nil
}

// Health 健康检查
func (c *HTTPMCPClient) Health(ctx context.Context) error {
    // 通过调用 tools/list 来检查健康状态
    _, err := c.ListTools(ctx)
    return err
}

// Close 关闭连接
func (c *HTTPMCPClient) Close() error {
    // HTTP 客户端无需显式关闭
    return nil
}

// doJSONRPCRequest 发送 JSON-RPC 请求
func (c *HTTPMCPClient) doJSONRPCRequest(ctx context.Context, request interface{}, response interface{}) error {
    jsonData, err := json.Marshal(request)
    if err != nil {
        return err
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    
    httpResp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return err
    }
    defer httpResp.Body.Close()
    
    if httpResp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP error: %d", httpResp.StatusCode)
    }
    
    return json.NewDecoder(httpResp.Body).Decode(response)
}

// JSONRPCError JSON-RPC 错误结构
type JSONRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

### 14.5 Knowledge Provider 接口定义（知识库抽象）

```go
// center/aiassistant/knowledge/interface.go
package knowledge

import "context"

// KnowledgeProvider 知识库查询抽象接口
type KnowledgeProvider interface {
    // Query 查询知识库
    Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error)
    
    // Health 健康检查
    Health(ctx context.Context) error
    
    // GetProviderName 获取 Provider 名称
    GetProviderName() string
}

// QueryRequest 查询请求
type QueryRequest struct {
    UserID         string `json:"user_id"`
    SessionID      string `json:"session_id"`        // 夜莺的 session_id
    ConversationID string `json:"conversation_id"`   // 知识库的 conversation_id
    Message        string `json:"message"`
    BotID          string `json:"bot_id,omitempty"`  // Coze Bot ID（可配置）
}

// QueryResponse 查询响应
type QueryResponse struct {
    ConversationID string `json:"conversation_id"`   // 用于后续对话
    Answer         string `json:"answer"`
    Status         string `json:"status"`   // completed/failed
    Error          string `json:"error,omitempty"`
}
```

### 14.6 Coze Provider 实现

**基于 Coze 官方 API v3**：

```go
// center/aiassistant/knowledge/coze_provider.go
package knowledge

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type CozeProvider struct {
    apiKey  string
    baseURL string
    client  *http.Client
}

type CozeConfig struct {
    BaseURL      string
    APIKey       string
    DefaultBotID string
}

func NewCozeProvider(config CozeConfig) *CozeProvider {
    return &CozeProvider{
        apiKey:  config.APIKey,
        baseURL: config.BaseURL,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (p *CozeProvider) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
    // 构建 Coze API v3 请求
    // 参考：https://www.coze.com/docs/developer_guides/chat_v3
    cozeReq := map[string]interface{}{
        "bot_id":            req.BotID,
        "user_id":           req.UserID,
        "stream":            false,
        "auto_save_history": true,
        "additional_messages": []map[string]interface{}{
            {
                "role":         "user",
                "content":      req.Message,
                "content_type": "text",
            },
        },
    }
    
    // 如果有 conversation_id，则继续之前的对话
    if req.ConversationID != "" {
        cozeReq["conversation_id"] = req.ConversationID
    }
    
    jsonData, _ := json.Marshal(cozeReq)
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v3/chat", bytes.NewBuffer(jsonData))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
    
    httpResp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer httpResp.Body.Close()
    
    var cozeResp struct {
        Code int    `json:"code"`
        Msg  string `json:"msg"`
        Data struct {
            ConversationID string `json:"conversation_id"`
            Status         string `json:"status"`
        } `json:"data"`
        Messages []struct {
            Role        string `json:"role"`
            Content     string `json:"content"`
            ContentType string `json:"content_type"`
        } `json:"messages"`
    }
    
    if err := json.NewDecoder(httpResp.Body).Decode(&cozeResp); err != nil {
        return nil, err
    }
    
    if cozeResp.Code != 0 {
        return &QueryResponse{
            Status: "failed",
            Error:  cozeResp.Msg,
        }, nil
    }
    
    // 提取 assistant 回复
    var answer string
    for _, msg := range cozeResp.Messages {
        if msg.Role == "assistant" && msg.ContentType == "text" {
            answer = msg.Content
            break
        }
    }
    
    return &QueryResponse{
        ConversationID: cozeResp.Data.ConversationID,
        Answer:         answer,
        Status:         cozeResp.Data.Status,
    }, nil
}

func (p *CozeProvider) Health(ctx context.Context) error {
    // 简单的健康检查（可选实现）
    return nil
}

func (p *CozeProvider) GetProviderName() string {
    return "coze"
}
```

### 14.7 Provider 注册机制

```go
// center/aiassistant/knowledge/registry.go
package knowledge

import "fmt"

var providerRegistry = make(map[string]KnowledgeProvider)

func RegisterProvider(name string, provider KnowledgeProvider) {
    providerRegistry[name] = provider
}

func GetProvider(name string) (KnowledgeProvider, error) {
    provider, exists := providerRegistry[name]
    if !exists {
        return nil, fmt.Errorf("knowledge provider not found: %s", name)
    }
    return provider, nil
}

// 在应用启动时注册
func InitProviders(config *Config) {
    if config.KnowledgeProvider.Type == "coze" {
        RegisterProvider("coze", NewCozeProvider(config.KnowledgeProvider.Coze))
    }
    // 未来可以添加更多 Provider: dify, fastgpt, custom
}
```

---


func RegisterMCPClient(serverType string, factory MCPClientFactory) {
    clientRegistry[serverType] = factory
}

func NewMCPClient(serverType string, config interface{}) (MCPClientInterface, error) {
    factory, exists := clientRegistry[serverType]
    if !exists {
        return nil, fmt.Errorf("unsupported MCP server type: %s", serverType)
    }
    return factory(config)
}

// 在各实现的 init() 中注册
func init() {
    RegisterMCPClient("stdio", func(config interface{}) (MCPClientInterface, error) {
        return NewStdioMCPClient(config.(StdioConfig))
    })
    RegisterMCPClient("sse", func(config interface{}) (MCPClientInterface, error) {
        return NewSSEMCPClient(config.(SSEConfig))
    })
}
```

---

## 15. 配置文件规范

### 15.1 配置存储方式

**核心配置从数据库读取，支持动态修改无需重启。**

所有 AI 助手的配置存储在 `ai_config` 表中（见第 12.3.1 节），支持：
- ✅ 动态修改配置
- ✅ 全局配置 + 业务组级配置
- ✅ 配置版本管理
- ✅ 配置热加载（通过缓存刷新）

**config.toml 仅保留基础配置**：

```toml
[AIAssistant]
# Redis 配置（复用全局 Redis）
RedisPrefix = "ai_assistant:"

# 其他配置从数据库读取，见 ai_config 表
EnableDatabaseConfig = true  # 默认 true
```

### 15.2 配置加载实现

```go
// center/aiassistant/config/loader.go
package config

import (
    "context"
    "encoding/json"
    "sync"
    "time"
    
    "github.com/ccfos/nightingale/v6/models"
    "github.com/ccfos/nightingale/v6/pkg/ctx"
)

// ConfigLoader 配置加载器（从数据库读取 + 缓存）
type ConfigLoader struct {
    cache      sync.Map  // 配置缓存
    lastReload time.Time
    ctx        *ctx.Context
}

func NewConfigLoader(c *ctx.Context) *ConfigLoader {
    loader := &ConfigLoader{
        ctx: c,
    }
    
    // 初始加载
    loader.ReloadAll()
    
    // 启动定期刷新（每 60 秒）
    go loader.startAutoReload()
    
    return loader
}

// GetAIModelConfig 获取 AI 模型配置
func (l *ConfigLoader) GetAIModelConfig(key string) (*AIModelConfig, error) {
    // 先从缓存获取
    if cached, ok := l.cache.Load(key); ok {
        return cached.(*AIModelConfig), nil
    }
    
    // 从数据库读取
    configValue, err := l.getConfigFromDB(key)
    if err != nil {
        return nil, err
    }
    
    var config AIModelConfig
    if err := json.Unmarshal([]byte(configValue), &config); err != nil {
        return nil, err
    }
    
    // 支持环境变量替换
    config.APIKey = expandEnvVar(config.APIKey)
    config.BaseURL = expandEnvVar(config.BaseURL)
    
    // 存入缓存
    l.cache.Store(key, &config)
    
    return &config, nil
}

// GetSessionConfig 获取会话管理配置
func (l *ConfigLoader) GetSessionConfig() (*SessionConfig, error) {
    return l.getTypedConfig("session.config", &SessionConfig{})
}

// GetKnowledgeConfig 获取知识库配置
func (l *ConfigLoader) GetKnowledgeConfig() (*KnowledgeConfig, error) {
    config, err := l.getTypedConfig("knowledge.provider", &KnowledgeConfig{})
    if err != nil {
        return nil, err
    }
    
    // 环境变量替换
    config.APIKey = expandEnvVar(config.APIKey)
    config.BaseURL = expandEnvVar(config.BaseURL)
    
    return config, nil
}

// ReloadAll 重新加载所有配置
func (l *ConfigLoader) ReloadAll() error {
    // 清空缓存
    l.cache.Range(func(key, value interface{}) bool {
        l.cache.Delete(key)
        return true
    })
    
    l.lastReload = time.Now()
    return nil
}

// getConfigFromDB 从数据库读取配置
func (l *ConfigLoader) getConfigFromDB(key string) (string, error) {
    var config models.AIConfig
    err := models.DB(l.ctx).Where("config_key = ? AND enabled = 1", key).First(&config).Error
    if err != nil {
        return "", err
    }
    
    return config.ConfigValue, nil
}

// getTypedConfig 通用类型配置获取
func (l *ConfigLoader) getTypedConfig(key string, target interface{}) (interface{}, error) {
    if cached, ok := l.cache.Load(key); ok {
        return cached, nil
    }
    
    configValue, err := l.getConfigFromDB(key)
    if err != nil {
        return nil, err
    }
    
    if err := json.Unmarshal([]byte(configValue), target); err != nil {
        return nil, err
    }
    
    l.cache.Store(key, target)
    return target, nil
}

// startAutoReload 自动刷新配置（每 60 秒）
func (l *ConfigLoader) startAutoReload() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // 检查数据库中的配置是否有更新
        var latestUpdate int64
        models.DB(l.ctx).Model(&models.AIConfig{}).
            Where("enabled = 1").
            Select("MAX(update_at)").
            Scan(&latestUpdate)
        
        if latestUpdate > l.lastReload.Unix() {
            l.ReloadAll()
            logger.Infof("AI config reloaded, latest update: %d", latestUpdate)
        }
    }
}

// expandEnvVar 环境变量展开（支持 ${VAR_NAME}）
func expandEnvVar(s string) string {
    if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
        envName := s[2 : len(s)-1]
        if envValue := os.Getenv(envName); envValue != "" {
            return envValue
        }
    }
    return s
}
```

### 15.3 配置数据结构

```go
// center/aiassistant/config/types.go
package config

type AIModelConfig struct {
    Provider    string  `json:"provider"`     // openai/gemini/azure
    BaseURL     string  `json:"base_url"`
    APIKey      string  `json:"api_key"`
    Model       string  `json:"model"`
    Temperature float64 `json:"temperature"`
    MaxTokens   int     `json:"max_tokens"`
}

type KnowledgeConfig struct {
    Type         string `json:"type"`           // coze/dify/custom
    BaseURL      string `json:"base_url"`
    APIKey       string `json:"api_key"`
    DefaultBotID string `json:"default_bot_id"`
}

type SessionConfig struct {
    TTL                   int64 `json:"ttl"`
    MaxMessagesPerSession int   `json:"max_messages_per_session"`
    MaxSessionsPerUser    int   `json:"max_sessions_per_user"`
}

type FileConfig struct {
    MaxSize        int64  `json:"max_size"`
    StorageBackend string `json:"storage_backend"`
    StoragePath    string `json:"storage_path"`
    TTL            int64  `json:"ttl"`
}

type ArchiveConfig struct {
    Enabled             bool   `json:"enabled"`
    InactiveThreshold   int64  `json:"inactive_threshold"`
    CronSchedule        string `json:"cron_schedule"`
}

type ToolConfig struct {
    CallTimeout         int `json:"call_timeout"`
    MaxCallsPerSession  int `json:"max_calls_per_session"`
}
```

### 15.4 使用示例

```go
// center/aiassistant/chat/handler.go
package chat

func NewChatHandler(ctx *ctx.Context) *ChatHandler {
    configLoader := config.NewConfigLoader(ctx)
    
    // 加载默认 AI 配置
    aiConfig, _ := configLoader.GetAIModelConfig("ai.default_model")
    
    // 加载知识库配置
    knowledgeConfig, _ := configLoader.GetKnowledgeConfig()
    
    // 加载会话配置
    sessionConfig, _ := configLoader.GetSessionConfig()
    
    return &ChatHandler{
        configLoader:    configLoader,
        aiConfig:        aiConfig,
        knowledgeConfig: knowledgeConfig,
        sessionConfig:   sessionConfig,
    }
}

// 动态获取专家 Agent 配置
func (h *ChatHandler) getExpertConfig(agentType AgentType) (*AIModelConfig, error) {
    key := fmt.Sprintf("ai.expert.%s", agentType)
    return h.configLoader.GetAIModelConfig(key)
}
```

### 15.5 配置管理接口实现

```go
// center/router/router_ai_assistant.go

// 获取所有配置
func configListHandler(c *gin.Context) {
    var configs []models.AIConfig
    err := models.DB(rt.Ctx).Where("enabled = 1").Find(&configs).Error
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"dat": configs, "error": ""})
}

// 获取单个配置
func configGetHandler(c *gin.Context) {
    key := c.Param("key")
    
    var config models.AIConfig
    err := models.DB(rt.Ctx).Where("config_key = ? AND enabled = 1", key).First(&config).Error
    if err != nil {
        c.JSON(404, gin.H{"error": "config not found"})
        return
    }
    
    c.JSON(200, gin.H{"dat": config, "error": ""})
}

// 更新配置
func configUpdateHandler(c *gin.Context) {
    key := c.Param("key")
    
    var req struct {
        ConfigValue string `json:"config_value"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }
    
    // 验证 JSON 格式
    var tmp interface{}
    if err := json.Unmarshal([]byte(req.ConfigValue), &tmp); err != nil {
        c.JSON(400, gin.H{"error": "invalid JSON format"})
        return
    }
    
    // 更新数据库
    err := models.DB(rt.Ctx).Model(&models.AIConfig{}).
        Where("config_key = ?", key).
        Updates(map[string]interface{}{
            "config_value": req.ConfigValue,
            "update_at":    time.Now().Unix(),
            "update_by":    c.GetString("username"),
        }).Error
    
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"dat": "ok", "error": ""})
}

// 重新加载配置
func configReloadHandler(c *gin.Context) {
    // 触发配置重载（通过 Redis pub/sub 或直接调用）
    if err := globalConfigLoader().ReloadAll(); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"dat": "config reloaded", "error": ""})
}
```

---

    return &cfg, nil
}
```

---

## 16. 错误码表

### 16.1 标准错误码定义

```go
// center/aiassistant/errors.go
package aiassistant

const (
    // 通用错误 (1000-1099)
    ErrCodeInternal          = "INTERNAL_ERROR"
    ErrCodeInvalidRequest    = "INVALID_REQUEST"
    
    // 会话错误 (1100-1199)
    ErrCodeSessionNotFound   = "SESSION_NOT_FOUND"
    ErrCodeSessionExpired    = "SESSION_EXPIRED"
    ErrCodeSessionLimitExceeded = "SESSION_LIMIT_EXCEEDED"
    
    // 工具错误 (1200-1299)
    ErrCodeToolNotFound      = "TOOL_NOT_FOUND"
    ErrCodeToolCallFailed    = "TOOL_CALL_FAILED"
    ErrCodeToolTimeout       = "TOOL_TIMEOUT"
    ErrCodeUpstreamError     = "UPSTREAM_ERROR"
    
    // 权限错误 (1300-1399)
    ErrCodePermissionDenied  = "PERMISSION_DENIED"
    ErrCodeEnvNotAllowed     = "ENV_NOT_ALLOWED"
    ErrCodeIPNotAllowed      = "IP_NOT_ALLOWED"
    
    // 确认错误 (1400-1499)
    ErrCodeConfirmationExpired  = "CONFIRMATION_EXPIRED"
    ErrCodeConfirmationNotFound = "CONFIRMATION_NOT_FOUND"
    ErrCodeRiskRejected         = "RISK_REJECTED"
    
    // 文件错误 (1500-1599)
    ErrCodeFileNotFound      = "FILE_NOT_FOUND"
    ErrCodeFileTooLarge      = "FILE_TOO_LARGE"
    ErrCodeInvalidFileType   = "INVALID_FILE_TYPE"
    
    // MCP 错误 (1600-1699)
    ErrCodeMCPServerNotFound = "MCP_SERVER_NOT_FOUND"
    ErrCodeMCPConnectionFailed = "MCP_CONNECTION_FAILED"
    ErrCodeMCPHealthCheckFailed = "MCP_HEALTH_CHECK_FAILED"
)

// Error 定义结构化错误
type Error struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

func (e *Error) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// 创建错误的辅助函数
func NewError(code, message string) *Error {
    return &Error{Code: code, Message: message}
}
```

### 16.2 错误码使用示例

```go
// 工具调用失败时返回
return nil, &aiassistant.Error{
    Code:    aiassistant.ErrCodeToolCallFailed,
    Message: "SQL 执行失败",
    Details: "Access denied for user 'readonly'@'%'",
}

// 前端响应示例 (HTTP 200)
{
  "dat": {
    "status": "error",
    "tool": {
      "error": {
        "code": "TOOL_CALL_FAILED",
        "message": "SQL 执行失败",
        "details": "Access denied for user 'readonly'@'%'"
      }
    }
  },
  "error": ""
}
```

---

## 17. 前端兼容性说明（Ant Design 4.x）

### 17.1 版本约束

根据 `AGENTS.md` 第 2.4 节，项目使用 **antd 4.21.0**，开发时必须遵守：

#### Modal 组件
```typescript
// ✅ 正确（antd 4.x）
<Modal visible={visible} onCancel={handleCancel}>

// ❌ 错误（antd 5.x API）
<Modal open={visible} onCancel={handleCancel}>
```

#### Tabs 组件
```typescript
// ✅ 正确（antd 4.x）
<Tabs defaultActiveKey="chat">
  <TabPane tab="对话" key="chat">...</TabPane>
  <TabPane tab="知识库" key="knowledge">...</TabPane>
</Tabs>

// ❌ 错误（antd 5.x API）
<Tabs items={[{key: 'chat', label: '对话', children: ...}]} />
```

### 17.2 确认卡片渲染示例

```typescript
// fe/src/pages/ai-assistant/components/ConfirmationCard.tsx
import React from 'react';
import { Card, Button, Descriptions } from 'antd';
import { WarningOutlined } from '@ant-design/icons';

interface ConfirmationCardProps {
  pendingConfirmation: {
    confirm_id: string;
    risk_level: 'high' | 'medium';
    summary: string;
    proposed_tool: {
      name: string;
      request: any;
    };
  };
  onConfirm: (confirmId: string) => void;
  onReject: (confirmId: string) => void;
}

const ConfirmationCard: React.FC<ConfirmationCardProps> = ({ 
  pendingConfirmation, 
  onConfirm, 
  onReject 
}) => {
  return (
    <Card 
      title={
        <span>
          <WarningOutlined style={{ color: '#faad14', marginRight: 8 }} />
          高风险操作确认
        </span>
      }
      extra={
        <>
          <Button onClick={() => onReject(pendingConfirmation.confirm_id)}>
            拒绝
          </Button>
          <Button 
            type="primary" 
            danger 
            onClick={() => onConfirm(pendingConfirmation.confirm_id)}
            style={{ marginLeft: 8 }}
          >
            确认执行
          </Button>
        </>
      }
    >
      <Descriptions column={1}>
        <Descriptions.Item label="风险等级">
          {pendingConfirmation.risk_level === 'high' ? '高' : '中'}
        </Descriptions.Item>
        <Descriptions.Item label="操作摘要">
          {pendingConfirmation.summary}
        </Descriptions.Item>
        <Descriptions.Item label="工具名称">
          {pendingConfirmation.proposed_tool.name}
        </Descriptions.Item>
      </Descriptions>
    </Card>
  );
};

export default ConfirmationCard;
```

---

## 18. 扩展与未来改进

### 18.1 流式响应支持（未来）

预留接口支持 Server-Sent Events (SSE)：

```go
// GET /api/n9e/ai-assistant/chat/stream
func (rt *Router) aiChatStream(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    // 流式输出 AI 响应
}
```

### 18.2 工具链式编排（未来扩展）

#### 18.2.1 当前模式的限制

**设计约束**：当前采用"单轮单工具"模式

```
用户请求 → AI 解析 → 调用单个工具 → 返回结果
```

**限制场景示例**：

1. **多 namespace 查询问题**
   ```
   用户: "查看 prod、staging、test 三个 namespace 下的 nginx pod"
   
   当前模式:
   - AI 只能选择一个 namespace 调用 list_pods（如 prod）
   - 无法自动遍历多个 namespace
   - 需要用户发起 3 次对话
   ```

2. **依赖链场景**
   ```
   用户: "检查所有 CrashLoopBackOff 的 pod 并查看它们的日志"
   
   当前模式:
   - 第一轮：list_pods (filter: status=CrashLoopBackOff)
   - 第二轮：用户手动选择一个 pod，请求查看日志
   - 无法自动化完成整个流程
   ```

3. **聚合计算场景**
   ```
   用户: "统计每个数据库实例的慢查询总数"
   
   当前模式:
   - 需要多轮对话逐个查询实例
   - 无法一次性完成聚合计算
   ```

#### 18.2.2 批量查询的过渡方案（可先实现）

在引入完整 DAG 之前，可通过以下方式改进批量场景：

##### 方案 A：工具内置批量参数

```go
// MCP 工具定义支持数组参数
type ListPodsRequest struct {
    Namespaces []string `json:"namespaces"` // 支持多个 namespace
    PodName    string   `json:"pod_name"`
}

// 工具内部循环查询
func (c *K8sMCPClient) ListPods(req ListPodsRequest) (*ToolResponse, error) {
    var allPods []Pod
    for _, ns := range req.Namespaces {
        pods, err := c.kubeClient.CoreV1().Pods(ns).List(...)
        if err != nil {
            // 记录错误但继续查询其他 namespace
            continue
        }
        allPods = append(allPods, pods.Items...)
    }
    return formatPodsResponse(allPods), nil
}
```

**优势**：
- 无需改变"单轮单工具"架构
- 工具自行处理批量逻辑

**劣势**：
- 工具超时风险（查询过多 namespace）
- 错误处理复杂（部分成功场景）
- 无法灵活组合多种工具

##### 方案 B：AI 主导的迭代查询

```go
// 在 Chat API 中支持 follow_up 标记
type ChatResponse struct {
    Status   string `json:"status"` // completed / follow_up_needed
    FollowUp *FollowUpPlan `json:"follow_up,omitempty"`
    // ...
}

type FollowUpPlan struct {
    RemainingTasks []TaskDefinition `json:"remaining_tasks"`
    AutoExecute    bool             `json:"auto_execute"` // 是否自动执行
}

// 示例流程
// Round 1: list_pods(namespace=prod) → 返回 follow_up_needed
// Round 2: list_pods(namespace=staging) → 自动执行
// Round 3: list_pods(namespace=test) → 自动执行
// Round 4: 汇总结果返回给用户
```

**优势**：
- 保持单轮单工具不变
- AI 可以动态调整策略

**劣势**：
- 增加对话轮次（延迟高）
- 用户等待时间长

#### 18.2.3 DAG 工作流编排（完整方案）

##### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      AI Chat API                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Intent Parser (意图解析)                            │    │
│  │  - 识别是否需要多步骤                                │    │
│  │  - 生成 Workflow DAG                                 │    │
│  └─────────────────┬───────────────────────────────────┘    │
│                    │                                          │
│                    ▼                                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Workflow Engine (工作流引擎)                        │    │
│  │  ┌────────────┐  ┌───────────┐  ┌──────────────┐   │    │
│  │  │ Task Queue │  │ Executor  │  │ Result Store │   │    │
│  │  └────────────┘  └───────────┘  └──────────────┘   │    │
│  └─────────────────┬───────────────────────────────────┘    │
│                    │                                          │
│                    ▼                                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Tool Caller (工具调用层)                            │    │
│  │  - MCP Tools                                         │    │
│  │  - 内置工具 (DBM, Alert Mute)                        │    │
│  └──────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

##### Workflow 定义格式

```yaml
# YAML 定义（存储在 Redis 或数据库）
workflow:
  id: wf_check_pods_multiNS_20260107
  name: "检查多个 namespace 的 nginx pod"
  created_by: "user_123"
  tasks:
    - id: task_1_list_prod
      name: "查询 prod namespace"
      tool: k8s.list_pods
      params:
        namespace: prod
        label_selector: "app=nginx"
      on_success: task_aggregate
      on_error: continue  # 单个 namespace 失败不中断
      
    - id: task_2_list_staging
      name: "查询 staging namespace"
      tool: k8s.list_pods
      params:
        namespace: staging
        label_selector: "app=nginx"
      on_success: task_aggregate
      on_error: continue
      
    - id: task_3_list_test
      name: "查询 test namespace"
      tool: k8s.list_pods
      params:
        namespace: test
        label_selector: "app=nginx"
      on_success: task_aggregate
      on_error: continue
      
    - id: task_aggregate
      name: "汇总结果"
      tool: builtin.aggregate
      depends_on: [task_1_list_prod, task_2_list_staging, task_3_list_test]
      params:
        strategy: merge_array
        output_format: markdown_table

  # 执行策略
  execution:
    mode: parallel  # parallel / sequential
    max_concurrency: 3
    timeout: 60000  # 整体超时 60 秒
    retry_policy:
      max_retries: 2
      backoff: exponential
```

##### Go 数据结构

```go
// center/aiassistant/workflow/types.go
package workflow

type Workflow struct {
    ID        string                 `json:"id"`
    Name      string                 `json:"name"`
    CreatedBy string                 `json:"created_by"`
    Tasks     []Task                 `json:"tasks"`
    Execution ExecutionConfig        `json:"execution"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Task struct {
    ID        string                 `json:"id"`
    Name      string                 `json:"name"`
    Tool      string                 `json:"tool"` // e.g., "k8s.list_pods"
    Params    map[string]interface{} `json:"params"`
    DependsOn []string               `json:"depends_on,omitempty"`
    OnSuccess string                 `json:"on_success,omitempty"` // 下一个任务 ID
    OnError   ErrorStrategy          `json:"on_error"`             // fail / continue / retry
}

type ErrorStrategy string

const (
    ErrorStrategyFail     ErrorStrategy = "fail"
    ErrorStrategyContinue ErrorStrategy = "continue"
    ErrorStrategyRetry    ErrorStrategy = "retry"
)

type ExecutionConfig struct {
    Mode           ExecutionMode `json:"mode"`
    MaxConcurrency int           `json:"max_concurrency"`
    Timeout        int64         `json:"timeout"` // 毫秒
    RetryPolicy    *RetryPolicy  `json:"retry_policy,omitempty"`
}

type ExecutionMode string

const (
    ExecutionModeParallel   ExecutionMode = "parallel"
    ExecutionModeSequential ExecutionMode = "sequential"
)

type RetryPolicy struct {
    MaxRetries int    `json:"max_retries"`
    Backoff    string `json:"backoff"` // fixed / exponential
}
```

##### 执行引擎实现

```go
// center/aiassistant/workflow/engine.go
package workflow

import (
    "context"
    "sync"
)

type Engine struct {
    toolCaller ToolCaller
    storage    WorkflowStorage
}

func (e *Engine) Execute(ctx context.Context, wf *Workflow) (*ExecutionResult, error) {
    // 1. 构建 DAG
    dag, err := buildDAG(wf.Tasks)
    if err != nil {
        return nil, err
    }
    
    // 2. 拓扑排序，分层执行
    layers := topologicalSort(dag)
    
    results := make(map[string]*TaskResult)
    var mu sync.Mutex
    
    // 3. 按层执行任务
    for _, layer := range layers {
        if wf.Execution.Mode == ExecutionModeParallel {
            // 并行执行同一层的任务
            var wg sync.WaitGroup
            sem := make(chan struct{}, wf.Execution.MaxConcurrency)
            
            for _, taskID := range layer {
                task := findTask(wf.Tasks, taskID)
                wg.Add(1)
                sem <- struct{}{}
                
                go func(t Task) {
                    defer wg.Done()
                    defer func() { <-sem }()
                    
                    // 执行任务
                    result := e.executeTask(ctx, t, results)
                    
                    mu.Lock()
                    results[t.ID] = result
                    mu.Unlock()
                }(*task)
            }
            wg.Wait()
        } else {
            // 顺序执行
            for _, taskID := range layer {
                task := findTask(wf.Tasks, taskID)
                result := e.executeTask(ctx, *task, results)
                results[task.ID] = result
                
                // 检查错误策略
                if result.Error != nil && task.OnError == ErrorStrategyFail {
                    return &ExecutionResult{
                        WorkflowID: wf.ID,
                        Status:     "failed",
                        TaskResults: results,
                    }, result.Error
                }
            }
        }
    }
    
    // 4. 汇总结果
    return &ExecutionResult{
        WorkflowID:  wf.ID,
        Status:      "completed",
        TaskResults: results,
    }, nil
}

func (e *Engine) executeTask(ctx context.Context, task Task, prevResults map[string]*TaskResult) *TaskResult {
    // 1. 解析参数（支持引用前序任务结果）
    params := resolveParams(task.Params, prevResults)
    
    // 2. 调用工具
    toolResp, err := e.toolCaller.Call(ctx, task.Tool, params)
    
    // 3. 返回结果
    return &TaskResult{
        TaskID:    task.ID,
        StartTime: time.Now().Unix(),
        EndTime:   time.Now().Unix(),
        Output:    toolResp,
        Error:     err,
    }
}

// DAG 构建和拓扑排序
func buildDAG(tasks []Task) (*DAG, error) {
    dag := &DAG{
        Nodes: make(map[string]*Node),
        Edges: make(map[string][]string),
    }
    
    for _, task := range tasks {
        dag.Nodes[task.ID] = &Node{Task: task}
        for _, dep := range task.DependsOn {
            dag.Edges[dep] = append(dag.Edges[dep], task.ID)
        }
    }
    
    // 检测环
    if hasCycle(dag) {
        return nil, errors.New("workflow contains cycle")
    }
    
    return dag, nil
}

func topologicalSort(dag *DAG) [][]string {
    // Kahn 算法实现分层拓扑排序
    inDegree := make(map[string]int)
    for nodeID := range dag.Nodes {
        inDegree[nodeID] = 0
    }
    for _, targets := range dag.Edges {
        for _, target := range targets {
            inDegree[target]++
        }
    }
    
    var layers [][]string
    queue := []string{}
    
    // 找入度为 0 的节点
    for nodeID, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, nodeID)
        }
    }
    
    for len(queue) > 0 {
        // 当前层的所有节点
        currentLayer := queue
        queue = []string{}
        layers = append(layers, currentLayer)
        
        // 处理下一层
        for _, nodeID := range currentLayer {
            for _, target := range dag.Edges[nodeID] {
                inDegree[target]--
                if inDegree[target] == 0 {
                    queue = append(queue, target)
                }
            }
        }
    }
    
    return layers
}
```

##### AI 自动生成 Workflow

```go
// AI 提示词模板
const workflowGenerationPrompt = `
根据用户请求生成 Workflow YAML:

用户请求: {{.UserMessage}}
可用工具: {{.AvailableTools}}

要求:
1. 识别需要多步骤的场景
2. 合理拆分为独立任务
3. 设置正确的依赖关系
4. 选择合适的执行模式（并行/顺序）
5. 处理错误策略

输出格式: 严格遵循 YAML schema
`

// AI 生成示例
// 输入: "检查所有 CrashLoopBackOff 的 pod 并查看它们的最新 100 行日志"
// 输出:
workflow:
  tasks:
    - id: task_find_pods
      tool: k8s.list_pods
      params:
        status_filter: CrashLoopBackOff
      
    - id: task_get_logs_pod1
      tool: k8s.get_logs
      depends_on: [task_find_pods]
      params:
        pod_name: "{{task_find_pods.output.pods[0].name}}"
        namespace: "{{task_find_pods.output.pods[0].namespace}}"
        tail: 100
        
    - id: task_get_logs_pod2
      tool: k8s.get_logs
      depends_on: [task_find_pods]
      params:
        pod_name: "{{task_find_pods.output.pods[1].name}}"
        namespace: "{{task_find_pods.output.pods[1].namespace}}"
        tail: 100
        
    # ... 动态生成 N 个日志查询任务
    
  execution:
    mode: parallel
    max_concurrency: 5
```

#### 18.2.4 渐进式迁移路径

**阶段 1**：保持单轮单工具 + 工具内置批量参数（当前可实现）
- 修改 K8s MCP 工具支持 `namespaces: []string`
- 数据库工具支持 `instance_ids: []int64`
- 用户体验提升但架构不变

**阶段 2**：引入简单 Workflow（3-6 个月后）
- 仅支持顺序执行（sequential）
- 固定模板（如"巡检流程"）
- AI 不参与 Workflow 生成，由用户选择模板

**阶段 3**：完整 DAG + AI 自动编排（1 年后）
- 支持并行执行
- AI 动态生成 Workflow
- 复杂依赖和错误处理

#### 18.2.5 技术栈选型建议

| 方案 | 优势 | 劣势 | 适用阶段 |
|------|------|------|----------|
| 自研轻量引擎 | 轻量、可控、与 AI 深度集成 | 功能有限 | 阶段 1-2 |
| Temporal/Cadence | 成熟、分布式、重试机制完善 | 重量级、学习成本高 | 阶段 3（大规模） |
| Argo Workflows | K8s 原生、社区活跃 | 依赖 K8s、非通用 | 仅 K8s 场景 |
| Airflow | Python 生态丰富 | 太重、定时任务导向 | 不推荐 |

**推荐**：阶段 1-2 自研，阶段 3 根据规模决定是否引入 Temporal

### 18.3 Metrics 指标建议

```go
// prometheus metrics
ai_assistant_chat_requests_total{mode="chat|knowledge|mcp", status="success|error"}
ai_assistant_tool_calls_total{tool_name="...", status="success|error"}
ai_assistant_confirmation_pending_total
ai_assistant_session_active_count
ai_assistant_file_upload_size_bytes
```

---

**版本**: 2.0.0  
**更新日期**: 2026-01-07  
**架构**: 独立模块，复用现有基础设施，最小化侵入性修改