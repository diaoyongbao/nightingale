-- AI LLM 模型配置表
-- n9e-2kai: AI 助手模块 - 自定义 LLM 模型管理

CREATE TABLE IF NOT EXISTS ai_llm_model (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL COMMENT '模型名称（显示用）',
    model_id VARCHAR(128) NOT NULL COMMENT '模型 ID（API 调用用）',
    provider VARCHAR(64) NOT NULL COMMENT '提供商',
    api_key VARCHAR(256) NOT NULL COMMENT 'API 密钥',
    base_url VARCHAR(256) NOT NULL COMMENT 'API 基础地址',
    temperature DECIMAL(3,2) DEFAULT 0.70 COMMENT '温度参数',
    max_tokens INT DEFAULT 4096 COMMENT '最大 Token 数',
    timeout INT DEFAULT 60 COMMENT '超时时间（秒）',
    description VARCHAR(500) COMMENT '模型描述',
    is_default TINYINT(1) DEFAULT 0 COMMENT '是否为默认模型',
    enabled TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    create_at BIGINT NOT NULL COMMENT '创建时间',
    create_by VARCHAR(64) NOT NULL COMMENT '创建人',
    update_at BIGINT NOT NULL COMMENT '更新时间',
    update_by VARCHAR(64) NOT NULL COMMENT '更新人',
    PRIMARY KEY (id),
    UNIQUE KEY idx_name (name),
    KEY idx_enabled (enabled),
    KEY idx_is_default (is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI LLM 模型配置表';
