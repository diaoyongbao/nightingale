# AI助手 - 动态 Agent 架构与系统功能集成设计

> **文档目的**: 针对用户提出的"动态扩展 Agent"与"系统功能 Agent 化"需求，设计一套无需修改代码即可定义、组合和发布新 Agent 的架构方案。

---

## 1. 核心设计理念：Agent as Configuration (数据驱动)

目前的架构中，专家 Agent（如 K8s 专家、DB 专家）是硬编码在 `center/aiassistant/agent/experts.go` 中的。为了实现动态扩展，我们需要将 Agent 的**定义**与**执行逻辑**解耦。

### 1.1 核心转变

| 维度 | 硬编码模式 (当前) | 动态模式 (目标) |
|------|-------------------|-----------------|
| **Agent 定义** | Go Struct 代码 | 数据库记录 (`ai_agent`) |
| **Prompt 管理** | 代码中的字符串常量 | 数据库字段 (支持在线编辑/版本管理) |
| **工具绑定** | 代码中 `Tools: []Tool{...}` | 关联表 (`ai_agent_tool_rel`) |
| **路由策略** | 代码中的 `switch/case` 或硬编码关键词 | 动态匹配 (关键词/语义路由) |
| **新功能扩展** | 需开发、编译、部署 | 在后台配置 API 映射 -> 立即生效 |

---

## 2. 数据库设计方案

通过三张核心表实现 Agent 的完全动态化。

### 2.1 `ai_tool` (工具定义表)

将系统中的“功能点”抽象为原子工具。

```sql
CREATE TABLE `ai_tool` (
    `id` bigint PRIMARY KEY AUTO_INCREMENT,
    `name` varchar(128) NOT NULL UNIQUE,  -- 例如: "alert_mute_create"
    `description` varchar(500) NOT NULL,  -- 给 LLM 看的工具描述
    
    -- 工具类型
    `type` varchar(32) NOT NULL,          -- 'api' (内部API), 'mcp' (外部MCP), 'script' (简单脚本)
    
    -- API 映射配置 (当 type='api' 时)
    `method` varchar(10),                 -- GET, POST
    `url_path` varchar(256),              -- 例如: "/api/n9e/alert-mutes"
    `parameter_schema` text,              -- JSON Schema，定义 LLM 需要提取的参数
    
    -- MCP 配置 (当 type='mcp' 时)
    `mcp_server_id` bigint,               -- 关联到 mcp_server 表
    `mcp_tool_name` varchar(128),         -- MCP Server 暴露的工具名
    
    `created_at` bigint,
    `updated_at` bigint 
);
```

### 2.2 `ai_agent` (Agent 定义表)

定义专家的“人设”和大脑配置。

```sql
CREATE TABLE `ai_agent` (
    `id` bigint PRIMARY KEY AUTO_INCREMENT,
    `name` varchar(128) NOT NULL UNIQUE,  -- 例如: "网络诊断专家"
    `description` varchar(500),           -- 用于父 Agent 路由决策
    `system_prompt` text NOT NULL,        -- 完整的 System Prompt 模板
    
    -- 模型配置
    `model_config` text,                  -- JSON: { "model": "gpt-4", "temperature": 0.5 }
    
    -- 路由配置
    `keywords` text,                      -- JSON数组: ["ping", "丢包", "延迟"]
    `enabled` tinyint(1) DEFAULT 1
);
```

### 2.3 `ai_agent_tool_rel` (关联表)

实现 Agent 与能力的灵活组合。一个工具可以被多个 Agent 复用。

```sql
CREATE TABLE `ai_agent_tool_rel` (
    `agent_id` bigint NOT NULL,
    `tool_id` bigint NOT NULL,
    PRIMARY KEY (`agent_id`, `tool_id`)
);
```

---

## 3. 系统功能 Agent 化方案 (API Converter)

如何快速将现有的 HTTP API 转化为 AI 可用的工具？

### 3.1 通用 API 适配器模式

我们需要实现一个通用的 `GeneralAPIExecutor`，它的作用是将 LLM 生成的 JSON 参数，转化为实际的内部 HTTP 请求。

**流程：**
1. **定义**: 在 `ai_tool` 表中注册一个 API，填入 `url_path` (e.g., `/api/n9e/users`) 和 `method`。
2. **Schema**: 无论 API 需要什么参数，都在 `parameter_schema` 中定义。
3. **执行**: 当 Agent 决定调用此工具时，适配器自动构建 HTTP Request，注入 `Authorization` 头，发起请求，并将 Response JSON 截断（防止过长）后返回给 LLM。

### 3.2 自动化导入 (Swagger/OpenAPI Import)

为了避免手动录入成百上千个 API，可以开发一个 **"Swagger Import"** 功能：
1. 读取 `docs/06-API接口文档.md` 或 Swagger JSON。
2. 自动提取 API 的 path, method 和 参数定义。
3. 为每个 API 生成一个初步的 `ai_tool` 记录。
4. 管理员微调 Description（这步很重要，因为 LLM 依赖 Description 理解何时调用）。

---

## 4. 动态路由机制 (Dynamic Router)

父 Agent (`Coordinator`) 需要从硬编码逻辑升级为动态逻辑。

### 4.1 启动加载
系统启动时，`Coordinator` 从 `ai_agent` 表加载所有启用的 Agent 及其 `keywords` 和 `description` 到内存缓存。

### 4.2 路由策略升级
当前：
```go
if contains(msg, "k8s") { return K8sAgent }
```

升级后：
1. **第一层：精确关键词匹配**
   遍历缓存中所有 Agent 的 `keywords`。如果用户命中 "抓包"，且 "网络专家" Agent 配置了该关键词，直接路由。

2. **第二层：语义/LLM 路由 (Fallback)**
   如果无关键词命中，构造如下 Prompt 发给 LLM（使用轻量模型如 GPT-3.5）：

   ```text
   用户输入: "最近这台机器总是连不上外网"
   
   可用专家列表:
   1. [network_expert]: 用于排查网络连通性、丢包、DNS问题。
   2. [db_expert]: 用于排查慢查询和死锁。
   3. [k8s_expert]: 用于排查 Pod 状态。
   
   请返回最合适的专家 ID，如果都不匹配返回 "general"。
   ```

---

## 5. 工作流示例：创建一个 "Nginx 日志分析专家"

**需求**：用户希望通过对话分析 Nginx 日志，目前系统中已有 `GET /api/logs/query` 接口。

### 步骤 1: 注册工具 (无需写代码)
在后台管理页面 "工具管理" -> "新增":
- **Name**: `search_nginx_logs`
- **Type**: `api`
- **URL**: `/api/n9e/logs/query`
- **Method**: `POST`
- **Schema**:
  ```json
  {
    "type": "object",
    "properties": {
      "query": { "type": "string", "description": "Lucene查询语法，e.g. 'status:500'" },
      "limit": { "type": "integer", "default": 20 }
    },
    "required": ["query"]
  }
  ```

### 步骤 2: 创建 Agent (无需写代码)
在 "Agent 管理" -> "新增":
- **Name**: `NginxLogExpert`
- **Description**: "专门用于分析 Web 服务器日志，排查 5xx 错误和流量异常。"
- **System Prompt**: "你是一名 Web 运维专家。当用户询问日志时，使用工具查询数据，并总结错误原因。"
- **Keywords**: `["nginx", "access.log", "502", "504", "流量突增"]`
- **Tools**: 勾选 `search_nginx_logs`。

### 步骤 3: 立即使用
用户在对话框输入："帮我看看最近 Nginx 有没有 500 报错"。
1. `Coordinator` 检测到 "Nginx"，路由给 `NginxLogExpert`。
2. `NginxLogExpert` 分析意图，调用 `search_nginx_logs` 工具，参数 `query="status:500"`.
3. 通用 API 适配器执行请求，返回日志 JSON。
4. Agent 总结日志并回复用户。

**全过程无需重启后端服务。**

---

---

## 6. 大规模 Agent 路由与管理 (Scalable Router)

针对用户提出的核心问题：**当专家 Agent 越来越多（例如从 5 个增加到 50 个），如何避免 Context Window 爆炸？如何让 LLM 准确选择是查知识库、调工具还是直接回答？**

我们采用 **"向量检索 + LLM 重排" 的二阶段路由架构** (RAG for Router)。

### 6.1 路由架构图

```mermaid
graph TD
    UserQuery[用户提问: "K8s 集群里有哪些 Pod 重启了？"] --> Layer0
    
    subgraph Layer0 [L0: 规则层 (零成本)]
        Check{匹配精确指令?}
        Check -- Yes --> Direct[直连专家/工具]
        Check -- No --> Layer1
    end
    
    subgraph Layer1 [L1: 召回层 (向量检索)]
        Embed[Text Embedding]
        VectorDB[(Agent 向量索引)]
        Embed --> VectorDB
        VectorDB -- Top K --> Candidates[候选专家列表 (Top 3~5)]
    end
    
    subgraph Layer2 [L2: 决策层 (LLM Router)]
        Prompt[构造路由 Prompt]
        Candidates --> Prompt
        LLM[LLM (GPT-3.5/4o-mini)]
        Prompt --> LLM
        LLM -- 决策结果 --> Action[分发任务]
    end
    
    Action -- "Type: Tool" --> ExpertAgent[执行专家 Agent]
    Action -- "Type: Knowledge" --> KnowledgeAgent[查询知识库]
    Action -- "Type: Chat" --> DirectReply[直接回复 (闲聊)]
```

### 6.2 L1 召回层：解决数量扩展问题
当系统中有 100 个 Agent 时，我们将每个 Agent 的 `Description` 和 `Keywords` 预先计算 Embedding 并存储。
*   **输入**：用户 Query
*   **动作**：Calculated Query Embedding -> Search Vector DB
*   **输出**：相似度最高的 Top 5 Agent（例如：K8s专家、容器监控专家、日志分析专家...）。
*   **优势**：Token 消耗恒定，与 Agent 总数无关。

### 6.3 L2 决策层：解决意图精准度问题
仅将 L1 召回的 Top 5 Agent 的精简描述放入 Prompt，询问 LLM：

> **System**: 你是一个任务分发员。请根据用户问题，从以下 5 个候选专家中选择最合适的一个。如果都不合适且问题是询问信息，选择 "KnowledgeBase"。如果是闲聊，选择 "None"。
>
> **Candidates**:
> 1. [K8sExpert]: 管理 Pod, Node, Service...
> 2. [LogExpert]: 查询服务器日志...
> ...
>
> **User**: "K8s 集群里有哪些 Pod 重启了？"

### 6.4 路由决策状态机 (FSM)

```text
State: Routing
  ├── Hit Keyword/Regex? ──> [Exact Match Expert] (e.g. "@db_expert help")
  └── No Match ──> Vector Search (Top 5)
         │
         ├── LLM Router Decision
         │     ├── Case 1: Specific Expert ──> [Execute Expert Agent]
         │     │     └── (In Expert) Call Tools / Answer
         │     │
         │     ├── Case 2: Information Seeking ──> [Execute Knowledge Agent]
         │     │     └── (In Knowledge) RAG Search -> Answer
         │     │
         │     └── Case 3: Chitchat/Unknown ──> [Direct LLM Reply]
```

---

## 7. 知识库与功能执行的融合

我们将 **知识库 (Knowledge Base)** 视为一个特殊的、系统级的 **标准 Agent**。

### 7.1 知识库 Agent 定义
在 `ai_agent` 表中预置一条记录：
*   **Name**: `KnowledgeExpert`
*   **Description**: "回答关于系统使用说明、运维文档、故障排查手册、API 文档等静态知识的问题。当用户询问'如何'、'文档'、'是什么'时使用。"
*   **Tools**: `[knowledge_search_tool]`

### 7.2 意图判断逻辑
LLM Router 如何区分是去“查库”还是“执行”？主要依赖 Agent 的 Description 差异：
*   **功能 Agent 描述**: 侧重动词，如 "执行"、"修改"、"查询实时状态"、"诊断"。
*   **知识库 Agent 描述**: 侧重名词和获取信息，如 "文档"、"说明"、"历史案例"。

**示例 Case**:
1.  **用户**: "怎么配置告警屏蔽？"
    *   **L1 召回**: `AlertExpert` (管理屏蔽), `KnowledgeExpert` (文档)。
    *   **L2 决策**: 问题是 "怎么配置" (How-to)，属于知识咨询。LLM Router 判定给 `KnowledgeExpert`。
    
2.  **用户**: "帮我屏蔽 host-01 的 CPU 告警 1 小时"
    *   **L1 召回**: `AlertExpert`, `KnowledgeExpert`。
    *   **L2 决策**: 问题是明确的指令 (Action)，包含具体参数。LLM Router 判定给 `AlertExpert`。

---

---

## 9. Agent Mentions 精确调用机制 (@功能)

为了满足高级用户“直达专家”的需求，系统支持 **Mention** 机制。此机制作为 **Layer 0 (规则层)** 的最高优先级策略。

### 9.1 交互设计
1.  **触发**：前段输入框键入 `@`。
2.  **联想**：弹窗列出当前启用的所有 Agent (Name + Description)。
3.  **选中**：输入框头部显示高亮 Tag `[@网络专家]`。

### 9.2 后端逻辑
当 `ChatHandler` 接收到消息 `[@NetworkExpert] ping 1.1.1.1` 时：
1.  **解析**：提取 `@` 标记，识别目标 Agent ID。
2.  **验证**：检查该 Agent 是否启用（Enabled）。
3.  **直连 (Direct Link)**：
    *   **跳过** L1 向量检索和 L2 LLM 路由。
    *   **加载** 该 Agent 的 System Prompt 和 Tools。
    *   **执行** 就像普通的单 Agent 对话一样，LLM 仅在该 Agent 的上下文中工作。
4.  **上下文隔离**：建议 Mention 模式下的对话历史单独标记，避免与其他 Agent 的上下文混淆。

### 9.3 优势
*   **零误判**：完全遵循用户意图。
*   **低延迟**：无路由开销。
*   **深度调试**：用户可以强制与某个专家（如“SQL 优化专家”）进行多轮深入对话，而不必担心被路由跳转到其他 Agent。

---

## 10. 代码改造路线图

1.  **Model 层**: 创建 `ai_agent`, `ai_tool`, `ai_agent_tool_rel` GORM 模型。
2.  **Loader 层**: 实现 `AgentLoader`，替代 `experts.go` 中的硬编码 map。
3.  **Executor 层**: 实现 `GenericAPIToolExecutor`，能够根据数据库配置发起 HTTP 请求。
4.  **Router 层**: 改造 `Coordinator.Route` 方法，支持基于内存配置的动态匹配。
5.  **前端**: 新增 "AI Agent 管理" 和 "AI 工具管理" 页面。

这种架构不仅解决了可扩展性问题，也为将来社区共享 Agent（导出/导入 Agent 配置 JSON）打下了基础。
