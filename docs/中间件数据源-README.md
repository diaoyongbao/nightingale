# 中间件数据源系统 - 总览

## 📋 项目概述

本项目实现了一个灵活的中间件数据源管理系统,用于统一管理 Archery、JumpServer、Jenkins 等运维中间件的连接配置。

### 核心特性

- ✅ **统一管理**: 所有中间件配置集中存储在数据库中
- ✅ **动态配置**: 支持运行时增删改查,无需重启服务
- ✅ **多实例支持**: 可配置同一类型的多个中间件实例
- ✅ **多种认证**: 支持 Token、Basic Auth、Session、OAuth2 等认证方式
- ✅ **加密存储**: 敏感信息(密码、Token)使用 RSA 加密
- ✅ **健康检查**: 内置健康检查机制,实时监控中间件状态
- ✅ **易扩展**: 设计良好,可轻松添加新的中间件类型

## 📁 文件结构

```
nightingale/
├── models/
│   ├── middleware_datasource.go           # 核心数据模型 ⭐
│   ├── middleware_datasource_migrate.go   # 迁移辅助函数
│   └── migrate/
│       └── migrate_middleware_datasource.go # 数据库迁移脚本
├── docs/
│   ├── 中间件数据源设计.md                # 详细设计文档 📖
│   ├── 中间件数据源实施总结.md            # 实施总结 📋
│   └── 中间件数据源快速开始.md            # 快速开始指南 🚀
└── center/
    └── dbm/                               # (现有) Archery 集成代码
        ├── archery_config.go
        ├── archery_client.go
        └── ...
```

### ⭐ 核心文件说明

| 文件 | 行数 | 说明 |
|------|-----|------|
| `models/middleware_datasource.go` | ~550 | 核心数据模型,包含完整的 CRUD 操作 |
| `models/middleware_datasource_migrate.go` | ~260 | Archery 配置迁移和转换函数 |
| `models/migrate/migrate_middleware_datasource.go` | ~55 | 数据库迁移脚本 |

## 📖 文档导航

### 1. 快速开始 🚀
**文件**: `docs/中间件数据源快速开始.md`

**适合人群**: 新手开发者、需要快速上手的用户

**内容**:
- 基本概念介绍
- 数据库表结构
- 支持的中间件类型和认证方式
- 常用代码示例
- API 方法参考
- 常见问题

**快速链接**:
```bash
# 查看快速开始指南
cat nightingale/docs/中间件数据源快速开始.md
```

### 2. 详细设计文档 📖
**文件**: `docs/中间件数据源设计.md`

**适合人群**: 架构师、核心开发者

**内容**:
- 需求分析
- 数据库表设计 (完整 SQL)
- 认证配置结构详解
- Go 模型定义
- 迁移方案
- API 接口设计
- 前端改造建议

### 3. 实施总结 📋
**文件**: `docs/中间件数据源实施总结.md`

**适合人群**: 项目经理、开发团队

**内容**:
- 已完成的工作清单
- 待实现功能列表
- 实施步骤指南
- 使用示例
- 安全注意事项
- 测试清单

## 🚀 快速开始

### 步骤 1: 查看数据库表结构

```sql
CREATE TABLE `middleware_datasource` (
  `id` bigint PRIMARY KEY AUTO_INCREMENT,
  `name` varchar(191) UNIQUE NOT NULL,
  `type` varchar(64) NOT NULL,
  `address` varchar(500) NOT NULL,
  `auth_type` varchar(32) NOT NULL,
  `auth_config` text,
  `status` varchar(32) DEFAULT 'enabled',
  -- ... 更多字段
);
```

### 步骤 2: 创建第一个中间件数据源

```go
import "github.com/ccfos/nightingale/v6/models"

ds := &models.MiddlewareDatasource{
    Name:        "archery-prod",
    Type:        models.MiddlewareTypeArchery,
    Address:     "https://archery.example.com",
    AuthType:    models.AuthTypeToken,
    AuthConfigJson: map[string]interface{}{
        "token": "your-token-here",
    },
    Status:      models.MiddlewareStatusEnabled,
}

err := ds.Add(ctx)
```

### 步骤 3: 查询数据源

```go
// 获取所有 Archery 实例
list, err := models.GetMiddlewareDatasourcesByType(ctx, models.MiddlewareTypeArchery)

// 获取单个实例
ds, err := models.MiddlewareDatasourceGetByName(ctx, "archery-prod")
```

## 🔧 集成指南

### 当前状态

✅ **已完成**:
- 数据库模型设计和实现
- 完整的 CRUD 方法
- Archery 配置迁移支持
- 加密/解密功能
- 数据验证

⏳ **待完成**:
- API 路由实现
- 前端管理界面
- Archery 客户端重构(从数据库读取配置)
- 健康检查定时任务

### 下一步实施

1. **数据库迁移集成**
   ```go
   // 在 models/migrate/migrate.go 中添加
   func MigrateTables(db *gorm.DB) error {
       dts := []interface{}{
           // ... 现有表
           &models.MiddlewareDatasource{}, // 新增
       }
       // ...
   }
   ```

2. **启动时自动迁移**
   ```go
   // 在 main.go 或启动函数中
   if config.Integrations.Archery.Enable {
       models.MigrateArcheryConfigToDB(ctx, config.Integrations.Archery)
   }
   ```

3. **实现 API 接口** (详见 `中间件数据源实施总结.md`)

4. **开发前端界面** (详见 `中间件数据源实施总结.md`)

## 📊 支持的中间件类型

| 类型 | 常量 | 说明 | 推荐认证方式 |
|------|------|------|------------|
| Archery | `MiddlewareTypeArchery` | SQL 审核平台 | Token / Basic |
| JumpServer | `MiddlewareTypeJumpServer` | 堡垒机 | Token |
| Jenkins | `MiddlewareTypeJenkins` | CI/CD | Basic / Token |
| GitLab | `MiddlewareTypeGitLab` | 代码仓库 | Token |
| Nacos | `MiddlewareTypeNacos` | 配置中心 | Basic |
| Consul | `MiddlewareTypeConsul` | 服务发现 | Token |

## 🔐 认证方式

| 方式 | 常量 | 配置示例 | 适用场景 |
|------|------|---------|---------|
| Token | `AuthTypeToken` | `{"token": "xxx"}` | API Token、Bearer Token |
| Basic Auth | `AuthTypeBasic` | `{"username": "admin", "password": "xxx"}` | HTTP Basic 认证 |
| Session | `AuthTypeSession` | `{"username": "admin", "password": "xxx", "login_url": "/login"}` | 需要登录获取 Cookie |
| OAuth2 | `AuthTypeOAuth2` | `{"client_id": "xxx", "client_secret": "xxx"}` | OAuth2 流程 |
| None | `AuthTypeNone` | `{}` | 无需认证 |

## 💡 使用示例

### 示例 1: 创建 Archery (Token 认证)

```go
ds := &models.MiddlewareDatasource{
    Name:    "archery-prod",
    Type:    models.MiddlewareTypeArchery,
    Address: "https://archery.example.com",
    AuthType: models.AuthTypeToken,
    AuthConfigJson: map[string]interface{}{
        "token": "eyJhbGci...",
        "header_name": "Authorization",
        "header_prefix": "Bearer",
    },
    Status: models.MiddlewareStatusEnabled,
}
ds.Add(ctx)
```

### 示例 2: 从配置文件迁移

```go
err := models.MigrateArcheryConfigToDB(ctx, config.Integrations.Archery)
```

### 示例 3: 查询所有启用的 Archery

```go
list, err := models.GetEnabledMiddlewareDatasourcesByType(ctx, models.MiddlewareTypeArchery)
```

更多示例请查看 `docs/中间件数据源快速开始.md`

## 🔒 安全特性

### 1. 敏感信息加密
```go
// 加密
ds.Encrypt(rsaConfig.OpenRSA, publicKey)

// 解密
ds.Decrypt(privateKey, password)
```

### 2. 明文清理
```go
// 加密后自动清理明文
ds.ClearPlaintext()
```

### 3. 审计字段
- `created_by` / `updated_by`: 记录操作人
- `created_at` / `updated_at`: 记录操作时间

## 📝 API 设计 (待实现)

```
GET    /api/n9e/middleware-datasources           # 获取列表
POST   /api/n9e/middleware-datasources           # 创建
GET    /api/n9e/middleware-datasources/:id       # 获取详情
PUT    /api/n9e/middleware-datasources/:id       # 更新
DELETE /api/n9e/middleware-datasources/:id       # 删除
POST   /api/n9e/middleware-datasources/:id/test  # 测试连接
GET    /api/n9e/middleware-datasources/types     # 获取类型列表
```

## 🧪 测试建议

- [ ] 单元测试: CRUD 操作
- [ ] 集成测试: 数据库迁移
- [ ] 加密测试: 敏感信息加密/解密
- [ ] 并发测试: 多线程读写
- [ ] 前端测试: 表单验证、UI 交互

## 📚 相关文档

1. **快速开始**: `docs/中间件数据源快速开始.md` (⭐ 推荐新手阅读)
2. **详细设计**: `docs/中间件数据源设计.md`
3. **实施总结**: `docs/中间件数据源实施总结.md`
4. **DBM 集成**: `docs/DBM集成说明.md` (现有 Archery 集成文档)

## 🤝 贡献指南

1. 阅读设计文档
2. 了解现有代码结构
3. 遵循代码规范
4. 提交清晰的 PR 说明

## 📞 支持与反馈

- 查看文档: `nightingale/docs/`
- 查看代码注释: `models/middleware_datasource.go`
- 提交 Issue 或 Pull Request

---

**版本**: 1.0.0  
**创建时间**: 2025-01-06  
**状态**: 核心模型已完成,API 和前端待实现
