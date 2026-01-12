-- AI 助手配置初始化数据
-- n9e-2kai: AI 助手模块 - 初始配置数据

-- 插入默认 AI 模型配置
INSERT INTO ai_config (config_key, config_value, config_type, description, scope, scope_id, enabled, create_at, create_by, update_at, update_by) VALUES
('ai.default_model', '{"provider":"openai","model":"gpt-3.5-turbo","api_key":"${OPENAI_API_KEY}","base_url":"https://api.openai.com/v1","temperature":0.7,"max_tokens":4096,"timeout":30}', 'ai_model', '默认 AI 模型配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

('ai.expert.k8s', '{"provider":"openai","model":"gpt-4","api_key":"${OPENAI_API_KEY}","base_url":"https://api.openai.com/v1","temperature":0.3,"max_tokens":4096,"timeout":30,"system_prompt":"你是一个 Kubernetes 专家，专门帮助用户解决 K8s 相关问题。"}', 'ai_model', 'Kubernetes 专家 Agent 配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

('ai.expert.database', '{"provider":"openai","model":"gpt-4","api_key":"${OPENAI_API_KEY}","base_url":"https://api.openai.com/v1","temperature":0.2,"max_tokens":4096,"timeout":30,"system_prompt":"你是一个数据库专家，专门帮助用户进行 SQL 查询、数据库优化和故障排查。"}', 'ai_model', '数据库专家 Agent 配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

('ai.expert.alert', '{"provider":"openai","model":"gpt-4","api_key":"${OPENAI_API_KEY}","base_url":"https://api.openai.com/v1","temperature":0.3,"max_tokens":4096,"timeout":30,"system_prompt":"你是一个告警分析专家，专门帮助用户分析告警事件、排查故障原因和提供解决方案。"}', 'ai_model', '告警专家 Agent 配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 知识库配置
('knowledge.provider', '{"provider":"coze","api_key":"${COZE_API_KEY}","base_url":"https://api.coze.cn","bot_id":"${COZE_BOT_ID}","timeout":30}', 'knowledge', '知识库提供者配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 会话管理配置
('session.config', '{"ttl":604800,"max_messages_per_session":2000,"max_sessions_per_user":50,"redis_prefix":"ai_assistant:session:"}', 'session', '会话管理配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 确认管理配置
('confirmation.config', '{"ttl":300,"redis_prefix":"ai_assistant:confirm:","high_risk_keywords":["DELETE","DROP","TRUNCATE","ALTER","UPDATE","INSERT"]}', 'general', '二次确认配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 文件管理配置
('file.config', '{"max_size":104857600,"allowed_types":["image/jpeg","image/png","image/gif","text/plain","application/json","application/pdf"],"storage_path":"/tmp/ai_assistant_files","download_token_ttl":3600}', 'file', '文件管理配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 归档配置
('archive.config', '{"auto_archive_days":30,"batch_size":100,"keep_messages":true,"anonymize_content":true}', 'general', '会话归档配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system'),

-- 工具配置
('tool.config', '{"timeout":60,"max_retries":3,"allowed_tools":["kubectl","mysql","prometheus"],"security_check":true}', 'general', '工具调用配置', 'global', 0, 1, UNIX_TIMESTAMP(), 'system', UNIX_TIMESTAMP(), 'system');