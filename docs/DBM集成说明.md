# 夜莺监控平台 - 数据库管理模块 (DBM)

## 项目概述

DBM（Database Management）模块实现了数据库的直接管理功能，**无需依赖外部服务**，所有操作直接在夜莺平台内完成。

## 架构设计

### 核心特点

- ✅ **直连数据库** - 直接连接 MySQL 等数据库，无外部依赖
- ✅ **连接池管理** - 统一的连接池管理器，支持多实例
- ✅ **实例配置存储** - 数据库实例信息存储在本地数据库
- ✅ **密码加密** - AES-256-GCM 加密存储敏感信息
- ✅ **健康检查** - 自动健康状态检测

### 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                      夜莺前端 (React)                        │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │
│  │实例管理 │  │会话管理 │  │慢查询   │  │SQL查询工作台    │ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └───────┬─────────┘ │
└───────┼────────────┼────────────┼───────────────┼───────────┘
        │            │            │               │
        ▼            ▼            ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│                   夜莺后端 API (Go)                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                router/router_dbm.go                   │   │
│  │  • 实例 CRUD    • 会话管理    • SQL执行    • 慢查询  │   │
│  └─────────────────────────┬────────────────────────────┘   │
│                            │                                 │
│  ┌─────────────────────────▼────────────────────────────┐   │
│  │                    dbm/                               │   │
│  │  ┌──────────────────┐  ┌──────────────────────────┐  │   │
│  │  │ ConnectionManager│  │      MySQLClient         │  │   │
│  │  │   连接池管理器   │  │  • GetSessions           │  │   │
│  │  │   • 多实例支持   │──│  • KillSession           │  │   │
│  │  │   • 自动重连     │  │  • ExecuteSQL            │  │   │
│  │  └──────────────────┘  │  • GetSlowQueries        │  │   │
│  │                        │  • GetUncommittedTrx     │  │   │
│  │                        └──────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    目标数据库实例                            │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │ MySQL-1 │  │ MySQL-2 │  │ MySQL-3 │  │  ...    │        │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## 功能特性

### 1. 实例管理

- 支持多种数据库类型：MySQL、PostgreSQL、Redis、MongoDB
- 实例配置的 CRUD 操作
- 连接健康检查
- 密码加密存储

### 2. 会话管理 (Processlist)

- 实时查看数据库连接会话
- 多维度筛选（命令类型、用户、数据库、执行时间）
- 批量 Kill 会话
- 会话详细信息展示

### 3. 慢查询分析

- 基于 `performance_schema` 的慢查询统计
- SQL 指纹识别
- 性能指标统计（平均/最大/最小耗时、扫描行数）

### 4. SQL 查询工作台

- SQL 编辑器
- SQL 语法检查（EXPLAIN）
- 查询结果表格展示
- 结果数量限制

### 5. 事务与锁监控

- 未提交事务列表
- 锁等待信息
- InnoDB 锁详情

## 文件结构

### 后端代码

```
nightingale/
├── center/
│   ├── dbm/                              # DBM 核心包
│   │   ├── interface.go                  # 客户端接口定义
│   │   ├── mysql_client.go               # MySQL 客户端实现
│   │   └── connection_manager.go         # 连接池管理器
│   └── router/
│       ├── router_dbm.go                 # DBM API 路由
│       └── router_dbm_helper.go          # 辅助函数
├── models/
│   └── db_instance.go                    # 数据库实例模型
```

### 前端代码

```
fe/src/
├── pages/dbm/
│   ├── index.tsx                         # 实例管理
│   ├── Sessions/index.tsx                # 会话管理
│   ├── SlowQueries/index.tsx             # 慢查询分析
│   ├── SQLQuery/index.tsx                # SQL查询工作台
│   └── locale/                           # 国际化
├── services/
│   └── dbm.ts                            # API 服务封装
```

## API 接口

### 实例管理

| 方法   | 路径                           | 说明           |
|--------|--------------------------------|----------------|
| GET    | `/api/n9e/dbm/instances`       | 获取实例列表   |
| GET    | `/api/n9e/dbm/instance/:id`    | 获取单个实例   |
| POST   | `/api/n9e/dbm/instance`        | 创建实例       |
| PUT    | `/api/n9e/dbm/instance/:id`    | 更新实例       |
| DELETE | `/api/n9e/dbm/instances`       | 批量删除实例   |
| POST   | `/api/n9e/dbm/instance/:id/health` | 健康检查   |

### 会话管理

| 方法   | 路径                           | 说明           |
|--------|--------------------------------|----------------|
| POST   | `/api/n9e/dbm/sessions`        | 获取会话列表   |
| POST   | `/api/n9e/dbm/sessions/kill`   | 批量 Kill 会话 |

### SQL 执行

| 方法   | 路径                           | 说明           |
|--------|--------------------------------|----------------|
| POST   | `/api/n9e/dbm/sql/query`       | 执行 SQL 查询  |
| POST   | `/api/n9e/dbm/sql/check`       | SQL 语法检查   |

### 事务与锁

| 方法   | 路径                              | 说明             |
|--------|-----------------------------------|------------------|
| POST   | `/api/n9e/dbm/uncommitted-trx`    | 获取未提交事务   |
| POST   | `/api/n9e/dbm/lock-waits`         | 获取锁等待信息   |
| GET    | `/api/n9e/dbm/innodb-locks`       | 获取 InnoDB 锁   |

### 慢查询

| 方法   | 路径                           | 说明           |
|--------|--------------------------------|----------------|
| POST   | `/api/n9e/dbm/slow-queries`    | 获取慢查询列表 |

## 数据库表结构

### db_instance 表

```sql
CREATE TABLE `db_instance` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `instance_name` varchar(128) NOT NULL UNIQUE,
  `db_type` varchar(32) NOT NULL DEFAULT 'mysql',
  
  -- 连接信息
  `host` varchar(128) NOT NULL,
  `port` int NOT NULL,
  `username` varchar(64) NOT NULL,
  `password` varchar(256) NOT NULL,  -- AES加密
  `charset` varchar(32) DEFAULT 'utf8mb4',
  
  -- 连接池配置
  `max_connections` int DEFAULT 10,
  `max_idle_conns` int DEFAULT 5,
  
  -- 状态
  `is_master` tinyint(1) DEFAULT 1,
  `enabled` tinyint(1) DEFAULT 1,
  `health_status` int DEFAULT 0,
  `last_check_time` bigint DEFAULT 0,
  `last_check_error` text,
  
  -- 元数据
  `description` varchar(500),
  `create_at` bigint NOT NULL,
  `create_by` varchar(64) NOT NULL,
  `update_at` bigint NOT NULL,
  `update_by` varchar(64) NOT NULL,
  
  KEY `idx_db_type` (`db_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 使用指南

### 1. 添加数据库实例

1. 导航到：**集成中心 → 数据库管理 → 实例管理**
2. 点击「新建」按钮
3. 填写实例信息：
   - 实例名称（唯一标识）
   - 数据库类型（MySQL/PostgreSQL 等）
   - 连接信息（Host、Port、用户名、密码）
   - 连接池配置（可选）
4. 点击「保存」

### 2. 查看会话

1. 进入「会话管理」页面
2. 选择要查看的数据库实例
3. 可按命令类型、用户、数据库筛选
4. 支持批量 Kill 选中的会话

### 3. 执行 SQL

1. 进入「SQL 查询」页面
2. 选择数据库实例和目标数据库
3. 输入 SQL 语句
4. 点击「执行」查看结果

## 权限配置

在 `center/cconf/ops.go` 中已配置：

```go
{Name: "/dbm", Ops: []string{"GET"}},           // DBM 只读权限
{Name: "/dbm/write", Ops: []string{"POST", "PUT", "DELETE"}}, // DBM 写权限
```

## 安全特性

1. **密码加密**：使用 AES-256-GCM 加密存储数据库密码
2. **权限控制**：区分只读和写操作权限
3. **审计日志**：所有操作记录操作用户和时间
4. **连接池管理**：自动管理连接生命周期，防止连接泄漏

## 扩展说明

### 添加新的数据库类型支持

1. 在 `models/db_instance.go` 中添加类型常量
2. 在 `dbm/` 目录下创建对应的客户端实现（如 `postgresql_client.go`）
3. 实现 `DBClient` 接口
4. 更新 `router_dbm_helper.go` 中的客户端工厂方法

### DBClient 接口

```go
type DBClient interface {
    Ping(ctx context.Context) error
    Close() error
    GetSessions(ctx context.Context, filter SessionFilter) ([]Session, error)
    KillSession(ctx context.Context, sessionID int64) error
    ExecuteSQL(ctx context.Context, req SQLExecuteRequest) (*SQLExecuteResult, error)
    CheckSQL(ctx context.Context, sql string) error
    GetUncommittedTransactions(ctx context.Context) ([]Transaction, error)
    GetLockWaits(ctx context.Context) ([]LockWait, error)
    GetSlowQueries(ctx context.Context, filter SlowQueryFilter) ([]SlowQuery, error)
}
```

## 兼容性

- **夜莺版本**: v6.x / v7.x
- **数据库支持**: MySQL 5.7+, MySQL 8.0+
- **浏览器**: Chrome 90+, Firefox 88+, Safari 14+

---

**版本**: 2.0.0  
**更新日期**: 2026-01-07  
**架构**: 直连数据库（无外部依赖）
