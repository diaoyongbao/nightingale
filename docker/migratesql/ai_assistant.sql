-- n9e-2kai: AI 助手模块数据库迁移脚本
-- 使用方法: mysql -u root -p n9e_v5 < ai_assistant.sql

USE n9e_v5;

-- MCP Server 配置表
CREATE TABLE IF NOT EXISTS `mcp_server` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL COMMENT 'Server 名称',
    `description` VARCHAR(500) DEFAULT '' COMMENT '描述',
    
    -- 连接配置
    `server_type` VARCHAR(32) NOT NULL DEFAULT 'http' COMMENT 'http 或 sse',
    `endpoint` VARCHAR(256) NOT NULL COMMENT 'MCP Server 地址',
    
    -- 健康检查
    `health_check_url` VARCHAR(256) DEFAULT '' COMMENT '健康检查 URL',
    `health_check_interval` INT DEFAULT 60 COMMENT '健康检查间隔（秒）',
    
    -- 安全配置
    `allowed_envs` TEXT COMMENT '允许的环境列表 JSON',
    `allowed_prefixes` TEXT COMMENT '允许的工具前缀 JSON',
    `allowed_ips` TEXT COMMENT '允许的 IP 列表 JSON',
    
    -- 状态
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    `health_status` INT DEFAULT 0 COMMENT '0=未知 1=健康 2=异常',
    `last_check_time` BIGINT DEFAULT 0 COMMENT '最后检查时间',
    `last_check_error` TEXT COMMENT '最后检查错误信息',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_mcp_server_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP Server 配置表';

-- MCP 模板表
CREATE TABLE IF NOT EXISTS `mcp_template` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL COMMENT '模板名称',
    `description` VARCHAR(500) DEFAULT '' COMMENT '描述',
    
    -- 模板内容
    `server_config` TEXT NOT NULL COMMENT 'Server 配置 JSON',
    `category` VARCHAR(64) DEFAULT 'custom' COMMENT '分类: k8s/db/monitor/custom',
    
    -- 标记
    `is_default` TINYINT(1) DEFAULT 0 COMMENT '是否默认模板',
    `is_public` TINYINT(1) DEFAULT 0 COMMENT '是否公开',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_mcp_template_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP 工具模板表';

-- AI 配置表
CREATE TABLE IF NOT EXISTS `ai_config` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `config_key` VARCHAR(128) NOT NULL COMMENT '配置键',
    `config_value` TEXT NOT NULL COMMENT '配置值 JSON',
    `config_type` VARCHAR(32) NOT NULL COMMENT '配置类型',
    `description` VARCHAR(500) DEFAULT '' COMMENT '描述',
    
    -- 作用范围
    `scope` VARCHAR(32) DEFAULT 'global' COMMENT 'global/busi_group',
    `scope_id` BIGINT DEFAULT 0 COMMENT '业务组 ID',
    
    -- 状态
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_ai_config_key` (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 助手配置表';

-- AI 助手会话归档表
CREATE TABLE IF NOT EXISTS `ai_assistant_session_archive` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `session_id` VARCHAR(64) NOT NULL COMMENT '会话 ID',
    `user_id` BIGINT NOT NULL COMMENT '用户 ID',
    
    -- 会话元数据
    `mode` VARCHAR(32) DEFAULT '' COMMENT '会话模式: chat/knowledge/mcp',
    `message_count` INT DEFAULT 0 COMMENT '消息数量',
    `tool_call_count` INT DEFAULT 0 COMMENT '工具调用次数',
    `first_message_at` BIGINT DEFAULT 0 COMMENT '首条消息时间',
    `last_message_at` BIGINT DEFAULT 0 COMMENT '末条消息时间',
    
    -- 归档内容
    `messages` LONGTEXT COMMENT '消息内容 JSON',
    `tool_calls` TEXT COMMENT '工具调用记录 JSON',
    `trace_ids` TEXT COMMENT 'Trace ID 列表 JSON',
    
    -- 归档信息
    `archived_at` BIGINT NOT NULL COMMENT '归档时间',
    `archived_by` VARCHAR(64) DEFAULT '' COMMENT '归档人',
    `archive_reason` VARCHAR(128) DEFAULT '' COMMENT '归档原因: manual/auto_expired/user_deleted',
    
    PRIMARY KEY (`id`),
    KEY `idx_session_archive_session_id` (`session_id`),
    KEY `idx_session_archive_user_id` (`user_id`),
    KEY `idx_session_archive_archived_at` (`archived_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 助手会话归档表';

-- 插入默认配置
INSERT INTO `ai_config` (`config_key`, `config_value`, `config_type`, `description`, `scope`, `scope_id`, `enabled`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
('ai.default_model', '{"provider":"openai","endpoint":"https://api.openai.com/v1","model":"gpt-4o","api_key":"","max_tokens":4096,"temperature":0.7}', 'ai_model', '默认 AI 模型配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),
('session.config', '{"max_history":20,"idle_timeout":1800,"max_duration":7200}', 'session', '会话配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),
('confirmation.config', '{"enabled":true,"dangerous_patterns":["delete","drop","truncate","kill","restart"],"risk_levels":{"low":1,"medium":2,"high":3}}', 'general', '确认机制配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),
('file.config', '{"max_size":10485760,"allowed_types":["text/plain","application/json","text/csv","image/png","image/jpeg"],"retention_hours":24}', 'file', '文件配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),
('archive.config', '{"auto_archive_days":30,"compression":true,"encrypt":false}', 'session', '归档配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system')
ON DUPLICATE KEY UPDATE `update_at` = UNIX_TIMESTAMP();

-- 权限点已在 ops.go 中定义，无需单独插入
-- Admin 角色自动拥有所有权限，无需插入 role_operation


-- ============================================
-- 知识库 Function Calling 架构改造
-- n9e-2kai: AI 助手模块 - 知识库 Provider 和工具表
-- ============================================

-- 知识库 Provider 配置表
CREATE TABLE IF NOT EXISTS `knowledge_provider` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL COMMENT 'Provider 名称',
    `provider_type` VARCHAR(64) NOT NULL COMMENT 'Provider 类型: cloudflare_autorag/coze/elasticsearch/custom_http',
    `description` VARCHAR(500) DEFAULT '' COMMENT '描述',
    `config` TEXT NOT NULL COMMENT 'Provider 配置 JSON',
    
    -- 状态
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    `health_status` INT DEFAULT 0 COMMENT '0=未知 1=健康 2=异常',
    `last_check_time` BIGINT DEFAULT 0 COMMENT '最后检查时间',
    `last_check_error` TEXT COMMENT '最后检查错误信息',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_knowledge_provider_name` (`name`),
    KEY `idx_knowledge_provider_type` (`provider_type`),
    KEY `idx_knowledge_provider_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识库 Provider 配置表';

-- 知识库工具配置表
CREATE TABLE IF NOT EXISTS `knowledge_tool` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL COMMENT '工具名称，如 search_ops_kb',
    `description` VARCHAR(1000) NOT NULL COMMENT '工具描述，供 LLM 理解何时调用',
    `provider_id` BIGINT NOT NULL COMMENT '关联的 Provider ID',
    
    -- 工具参数配置
    `parameters` TEXT COMMENT '额外参数配置 JSON',
    `keywords` TEXT COMMENT '触发关键词 JSON 数组',
    
    -- 状态
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    `priority` INT DEFAULT 0 COMMENT '优先级，多个工具匹配时使用',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_knowledge_tool_name` (`name`),
    KEY `idx_knowledge_tool_provider_id` (`provider_id`),
    KEY `idx_knowledge_tool_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='知识库工具配置表';

-- 插入默认 Cloudflare AutoRAG Provider
INSERT INTO `knowledge_provider` (`name`, `provider_type`, `description`, `config`, `enabled`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
('cloudflare-aiops', 'cloudflare_autorag', 'Cloudflare AutoRAG - 运维知识库', '{
  "account_id": "449309bb90ef97c592a6a6e40ca87884",
  "rag_name": "aiops",
  "api_token": "${CLOUDFLARE_API_TOKEN}",
  "model": "cerebras/gpt-oss-120b",
  "rewrite_query": true,
  "max_num_results": 10,
  "score_threshold": 0.3,
  "timeout": 30
}', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system')
ON DUPLICATE KEY UPDATE `update_at` = UNIX_TIMESTAMP();

-- 插入默认知识库工具
INSERT INTO `knowledge_tool` (`name`, `description`, `provider_id`, `parameters`, `keywords`, `enabled`, `priority`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
('search_ops_kb', '搜索运维知识库，用于查询系统配置、报错处理、最佳实践、操作指南等运维相关问题。当用户询问"怎么配置"、"如何操作"、"报错处理"、"地址是什么"、"文档在哪"等问题时调用此工具。', 
 (SELECT id FROM knowledge_provider WHERE name = 'cloudflare-aiops'), 
 '{"max_results": 10, "score_threshold": 0.3}', 
 '["配置", "怎么", "如何", "报错", "错误", "地址", "最佳实践", "操作指南", "文档", "帮助", "教程", "说明"]', 
 1, 100, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system')
ON DUPLICATE KEY UPDATE `update_at` = UNIX_TIMESTAMP();


-- ============================================
-- AI Agent 动态架构
-- n9e-2kai: AI 助手模块 - Agent 和工具动态配置
-- ============================================

-- AI Agent 定义表
CREATE TABLE IF NOT EXISTS `ai_agent` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL COMMENT 'Agent 名称，唯一标识',
    `description` VARCHAR(500) DEFAULT '' COMMENT '描述，用于路由决策',
    
    -- System Prompt
    `system_prompt` TEXT NOT NULL COMMENT '系统提示词',
    
    -- 模型配置 (JSON)
    `model_config` TEXT COMMENT '模型配置 JSON: {"model": "gpt-4", "temperature": 0.5, "max_tokens": 4096}',
    
    -- 路由配置
    `keywords` TEXT COMMENT '路由关键词 JSON 数组',
    `priority` INT DEFAULT 0 COMMENT '路由优先级，数值越大优先级越高',
    
    -- Agent 类型
    `agent_type` VARCHAR(32) DEFAULT 'expert' COMMENT 'system/expert/knowledge',
    
    -- 状态
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_ai_agent_name` (`name`),
    KEY `idx_ai_agent_type` (`agent_type`),
    KEY `idx_ai_agent_enabled` (`enabled`),
    KEY `idx_ai_agent_priority` (`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI Agent 定义表';

-- AI 工具定义表
CREATE TABLE IF NOT EXISTS `ai_tool` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(128) NOT NULL COMMENT '工具名称，唯一标识',
    `description` VARCHAR(500) NOT NULL COMMENT '工具描述，给 LLM 看的',
    
    -- 工具类型
    `implementation_type` VARCHAR(32) NOT NULL COMMENT 'native/api/mcp/knowledge',
    
    -- API 映射配置 (当 implementation_type='api' 时)
    `method` VARCHAR(10) DEFAULT '' COMMENT 'GET/POST/PUT/DELETE',
    `url_path` VARCHAR(256) DEFAULT '' COMMENT 'API 路径',
    `parameter_schema` TEXT COMMENT '参数 JSON Schema',
    `response_mapping` TEXT COMMENT '响应字段映射配置',
    
    -- MCP 配置 (当 implementation_type='mcp' 时)
    `mcp_server_id` BIGINT DEFAULT 0 COMMENT '关联的 MCP Server ID',
    `mcp_tool_name` VARCHAR(128) DEFAULT '' COMMENT 'MCP Server 暴露的工具名',
    
    -- Native 配置 (当 implementation_type='native' 时)
    `native_handler` VARCHAR(128) DEFAULT '' COMMENT 'Go 代码中注册的 handler 名称',
    
    -- Knowledge 配置 (当 implementation_type='knowledge' 时)
    `knowledge_provider_id` BIGINT DEFAULT 0 COMMENT '关联的知识库 Provider ID',
    
    -- 风险等级
    `risk_level` VARCHAR(16) DEFAULT 'low' COMMENT 'low/medium/high',
    
    -- 状态
    `enabled` TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    
    -- 审计
    `create_at` BIGINT NOT NULL COMMENT '创建时间',
    `create_by` VARCHAR(64) NOT NULL COMMENT '创建人',
    `update_at` BIGINT NOT NULL COMMENT '更新时间',
    `update_by` VARCHAR(64) NOT NULL COMMENT '更新人',
    
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_ai_tool_name` (`name`),
    KEY `idx_ai_tool_type` (`implementation_type`),
    KEY `idx_ai_tool_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 工具定义表';

-- Agent 与工具的关联表
CREATE TABLE IF NOT EXISTS `ai_agent_tool_rel` (
    `agent_id` BIGINT NOT NULL COMMENT 'Agent ID',
    `tool_id` BIGINT NOT NULL COMMENT '工具 ID',
    PRIMARY KEY (`agent_id`, `tool_id`),
    KEY `idx_ai_agent_tool_rel_tool` (`tool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent 与工具关联表';

-- 插入系统级 Agent
INSERT INTO `ai_agent` (`name`, `description`, `system_prompt`, `model_config`, `keywords`, `priority`, `agent_type`, `enabled`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
-- 通用 Agent (默认)
('sys_general', '通用运维助手，处理一般性问题和闲聊', '你是夜莺监控系统的运维助手。

## 核心职责
- 回答系统使用方法
- 提供配置说明
- 给出最佳实践建议

## 回答原则
- 如果知识库返回了相关结果，基于结果回答
- 如果知识库没有相关信息，用你的知识直接回答用户问题
- 禁止编造不存在的信息
- 保持专业、友好的语气', 
'{"model": "gpt-4o", "temperature": 0.7, "max_tokens": 4096}', 
'[]', 0, 'system', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 知识库 Agent
('sys_knowledge', '知识库专家，回答关于系统使用说明、运维文档、故障排查手册、API 文档等静态知识的问题', '你是知识库助手，专门从运维知识库中检索信息来回答用户问题。

## 职责
- 使用知识库工具搜索相关文档
- 基于搜索结果回答用户问题
- 标注信息来源

## 工具使用原则
当调用知识库工具时，query 参数必须：
- 只包含用户问题中的核心关键词（通常 1-3 个词）
- 不要添加任何额外词汇
- 使用用户原话中的关键词

## 回答原则
- 优先使用知识库检索结果
- 如果未找到相关信息，明确告知用户
- 不要编造不存在的信息', 
'{"model": "gpt-4o", "temperature": 0.3, "max_tokens": 4096}', 
'["文档", "手册", "说明", "指南", "教程", "怎么", "如何", "是什么"]', 100, 'system', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 路由决策 Agent
('sys_router', '路由决策专家，负责分析用户意图并选择合适的 Agent', '你是一个任务分发员。请根据用户问题，从候选专家中选择最合适的一个。

## 决策规则
1. 如果问题是询问信息、文档、配置方法，选择 "sys_knowledge"
2. 如果问题涉及 K8s/容器/Pod，选择 "k8s_expert"（如果存在）
3. 如果问题涉及数据库/SQL，选择 "db_expert"（如果存在）
4. 如果问题涉及告警/屏蔽，选择 "alert_expert"（如果存在）
5. 如果是闲聊或通用问题，选择 "sys_general"

## 输出格式
只返回 Agent 名称，不要其他内容。', 
'{"model": "gpt-4o-mini", "temperature": 0.1, "max_tokens": 100}', 
'[]', 1000, 'system', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 汇总 Agent
('sys_summary', '结果汇总专家，负责整合工具执行结果并生成用户友好的回答', '你是结果汇总助手。根据工具返回的结果，为用户生成清晰、有条理的回答。

## 汇总原则
- 提取关键信息
- 使用 Markdown 格式化输出
- 如果工具执行失败，解释原因并提供替代建议
- 保持简洁，避免冗余', 
'{"model": "gpt-4o", "temperature": 0.5, "max_tokens": 4096}', 
'[]', 999, 'system', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system')
ON DUPLICATE KEY UPDATE `update_at` = UNIX_TIMESTAMP();

-- 插入默认专家 Agent
INSERT INTO `ai_agent` (`name`, `description`, `system_prompt`, `model_config`, `keywords`, `priority`, `agent_type`, `enabled`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
-- K8s 专家
('k8s_expert', 'Kubernetes 运维专家，处理 Pod 故障诊断、资源调度分析、配置优化等问题', '你是 Kubernetes 运维专家，擅长：
- Pod 故障诊断（CrashLoopBackOff、ImagePullBackOff、OOMKilled 等）
- 资源调度分析（CPU/内存超限、节点亲和性）
- 配置优化建议（副本数、资源限制、探针配置）

## 诊断步骤
1. 先查看 Pod 状态和事件
2. 检查日志（最近 100 行）
3. 分析资源使用情况
4. 给出可操作的修复建议

遵循 Kubernetes 最佳实践。', 
'{"model": "gpt-4o", "temperature": 0.3, "max_tokens": 4096}', 
'["pod", "deployment", "service", "k8s", "kubernetes", "容器", "镜像", "namespace", "node", "ingress", "kubectl"]', 80, 'expert', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 数据库专家
('db_expert', '数据库运维专家，处理 SQL 查询优化、慢查询分析、执行计划分析等问题', '你是数据库运维专家，擅长：
- SQL 查询优化（慢查询分析、索引建议）
- 执行计划分析
- 数据库性能诊断

## 分析步骤
1. 先检查 SQL 语法
2. 分析执行计划（EXPLAIN）
3. 提出索引优化建议
4. 评估数据量和查询成本

## 注意事项
- 所有写操作（INSERT/UPDATE/DELETE）必须二次确认
- SELECT 未加 LIMIT 时提醒用户
- 多语句查询需要额外审核', 
'{"model": "gpt-4o", "temperature": 0.2, "max_tokens": 4096}', 
'["sql", "数据库", "慢查询", "query", "table", "索引", "mysql", "postgresql", "redis", "mongodb", "select", "insert", "update", "delete"]', 70, 'expert', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 告警专家
('alert_expert', '告警管理专家，处理告警规则配置、告警屏蔽策略设计、告警降噪优化等问题', '你是告警管理专家，擅长：
- 告警规则配置
- 告警屏蔽策略设计
- 告警降噪优化

## 操作步骤
1. 理解用户屏蔽需求
2. 设计精准的屏蔽规则（tags 过滤）
3. 使用 preview 评估影响范围
4. 超过 50 条匹配时要求用户确认

## 原则
- 避免过宽的屏蔽规则（tags 为空、正则 .*）
- 时间范围应明确（避免永久屏蔽）
- 优先使用 tags 而非 datasource 全量', 
'{"model": "gpt-4o", "temperature": 0.1, "max_tokens": 4096}', 
'["告警", "屏蔽", "mute", "alert", "通知", "规则", "订阅", "报警"]', 60, 'expert', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system')
ON DUPLICATE KEY UPDATE `update_at` = UNIX_TIMESTAMP();

-- 插入知识库工具到 ai_tool 表
INSERT INTO `ai_tool` (`name`, `description`, `implementation_type`, `knowledge_provider_id`, `parameter_schema`, `risk_level`, `enabled`, `create_at`, `create_by`, `update_at`, `update_by`) VALUES
('search_knowledge', '搜索运维知识库，用于查询系统配置、报错处理、最佳实践、操作指南等运维相关问题', 'knowledge', 
 (SELECT id FROM knowledge_provider WHERE name = 'cloudflare-aiops' LIMIT 1), 
 '{"type": "object", "properties": {"query": {"type": "string", "description": "搜索查询关键词"}}, "required": ["query"]}', 
 'low', 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system')
ON DUPLICATE KEY UPDATE `update_at` = UNIX_TIMESTAMP();

-- 绑定知识库工具到知识库 Agent
INSERT INTO `ai_agent_tool_rel` (`agent_id`, `tool_id`)
SELECT 
    (SELECT id FROM ai_agent WHERE name = 'sys_knowledge'),
    (SELECT id FROM ai_tool WHERE name = 'search_knowledge')
FROM dual
WHERE EXISTS (SELECT 1 FROM ai_agent WHERE name = 'sys_knowledge')
  AND EXISTS (SELECT 1 FROM ai_tool WHERE name = 'search_knowledge')
ON DUPLICATE KEY UPDATE `agent_id` = VALUES(`agent_id`);
