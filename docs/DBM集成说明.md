# 夜莺监控平台 - Archery数据库管理集成

## 项目概述

本项目实现了将Archery SQL审计平台的数据库管理功能完整集成到夜莺(Nightingale v6)监控平台中。所有数据库管理操作均在夜莺UI内完成,无需跳转到Archery界面。

## 功能特性

### 1. 实例管理
- 实例列表展示(支持多种数据库类型:MySQL、Redis、MongoDB、PostgreSQL等)
- 实例搜索与类型筛选
- Archery服务健康检查
- 实时状态监控

### 2. 会话管理(Processlist)
- 实时查看数据库连接会话
- 多维度筛选(命令类型、用户、数据库)
- 批量Kill会话功能
- 会话详细信息展示(线程ID、耗时、状态、SQL信息)

### 3. 慢查询分析
- 慢查询列表(基于pt-query-digest分析)
- SQL指纹识别
- 性能指标统计(平均/最大/最小耗时、扫描行数)
- 详细分析报告:
  - 执行计划(EXPLAIN)
  - 优化建议
  - 索引建议

### 4. SQL查询工作台
- SQL编辑器(带语法高亮)
- SQL语法检查
- 查询结果表格展示
- 结果数量限制配置
- SQL工单提交(用于DDL/DML变更审核)

## 技术架构

### 后端 (Go)

#### 1. API适配器层 (`nightingale/center/dbm/`)
- `archery_client.go` - HTTP客户端
- `archery_types.go` - 基础类型定义
- `archery_config.go` - 配置管理
- `archery_session.go` - 会话管理API
- `archery_slowquery.go` - 慢查询分析API
- `archery_sql.go` - SQL执行API

#### 2. 路由层 (`nightingale/center/router/`)
- `router_dbm.go` - DBM业务路由处理器
- `router.go` - 路由注册

**API端点列表**:
```
GET  /api/n9e/dbm/archery/instances          # 获取实例列表
GET  /api/n9e/dbm/archery/health             # 健康检查
POST /api/n9e/dbm/archery/sessions           # 获取会话列表
POST /api/n9e/dbm/archery/sessions/kill      # Kill会话
POST /api/n9e/dbm/archery/uncommitted-trx    # 获取未提交事务
POST /api/n9e/dbm/archery/slow-queries       # 获取慢查询列表
GET  /api/n9e/dbm/archery/slow-query/:instance_id/:checksum  # 慢查询详情
POST /api/n9e/dbm/archery/sql/query          # 执行SQL查询
POST /api/n9e/dbm/archery/sql/check          # SQL语法检查
POST /api/n9e/dbm/archery/sql/workflow       # 提交SQL工单
```

#### 3. 配置文件 (`etc/config.toml`)
```toml
[Integrations.Archery]
Enable = true
Address = "http://archery-server:8000"
Token = "your-archery-api-token"
Timeout = 30
```

### 前端 (React + TypeScript)

#### 1. 服务层 (`fe/src/services/dbm.ts`)
- API调用封装
- TypeScript类型定义

#### 2. 页面组件 (`fe/src/pages/dbm/`)
- `index.tsx` - 实例管理页面
- `Sessions/index.tsx` - 会话管理页面
- `SlowQueries/index.tsx` - 慢查询分析页面
- `SQLQuery/index.tsx` - SQL查询工作台

#### 3. 国际化 (`fe/src/pages/dbm/locale/`)
- `zh_CN.ts` - 中文简体
- `zh_HK.ts` - 中文繁体
- `en_US.ts` - 英文

#### 4. 路由配置 (`fe/src/routers/index.tsx`)
```
/dbm                  -> 实例管理
/dbm/sessions         -> 会话管理
/dbm/slow-queries     -> 慢查询分析
/dbm/sql-query        -> SQL查询工作台
```

#### 5. 菜单配置 (`fe/src/components/SideMenu/`)
- 集成中心 > 数据库管理
  - 实例管理
  - 会话管理
  - 慢查询分析
  - SQL查询

## 部署指南

### 1. 配置Archery

确保Archery服务已启动并开启API访问:

```bash
# Archery配置文件
# archery/settings.py
ALLOWED_HOSTS = ['*']
CORS_ORIGIN_ALLOW_ALL = True

# 生成API Token
python manage.py createtoken <username>
```

### 2. 配置夜莺

编辑 `nightingale/etc/config.toml`:

```toml
[Integrations.Archery]
Enable = true
Address = "http://your-archery-server:8000"
Token = "your-archery-api-token"
Timeout = 30
```

### 3. 启动服务

```bash
# 启动夜莺后端
cd nightingale
./n9e

# 启动前端(开发模式)
cd fe
npm install
npm run dev

# 或构建生产版本
npm run build
```

### 4. 访问

访问夜莺平台: `http://your-nightingale-host`
导航到: **集成中心 > 数据库管理**

## 权限配置

需要在 `nightingale/center/cconf/ops.go` 中添加权限定义:

```go
{Name: "/dbm", Ops: []string{"GET"}},          // DBM只读权限
{Name: "/dbm/write", Ops: []string{"POST", "PUT", "DELETE"}}, // DBM写权限
```

## 安全特性

1. **认证**: 复用夜莺JWT认证机制
2. **审计日志**: 所有数据库操作记录操作用户和时间
3. **权限控制**: 区分只读和写操作权限
4. **系统进程保护**: 禁止Kill系统关键进程(如Binlog Dump)

## 监控指标

集成后可监控:
- Archery服务健康状态
- 数据库连接会话数
- 慢查询趋势
- SQL执行成功率

## 故障排查

### 问题1: 无法连接Archery
**排查步骤**:
1. 检查Archery服务是否运行: `curl http://archery-server:8000/api/v1/health/`
2. 检查网络连通性
3. 验证Token是否正确
4. 查看夜莺日志: `tail -f nightingale/logs/n9e.log`

### 问题2: 前端页面空白
**排查步骤**:
1. 打开浏览器开发者工具查看Console错误
2. 检查前端构建是否成功
3. 验证路由配置是否正确

### 问题3: API返回错误
**排查步骤**:
1. 检查Archery API日志
2. 验证请求参数格式
3. 确认Archery版本兼容性

## 兼容性

- **夜莺版本**: v6.x
- **Archery版本**: v1.9.x+
- **数据库支持**: MySQL 5.7+, PostgreSQL 10+, Redis 5.0+, MongoDB 4.0+
- **浏览器**: Chrome 90+, Firefox 88+, Safari 14+

## 后续优化建议

1. **Monaco编辑器集成**: 替换TextArea为Monaco Editor,提供更好的SQL编写体验
2. **实时监控**: WebSocket实时推送会话变化
3. **批量操作**: 支持批量执行SQL
4. **历史记录**: SQL执行历史记录功能
5. **性能优化**: 慢查询结果缓存
6. **图表可视化**: 慢查询趋势图表

## 文件清单

### 后端新增文件
```
nightingale/center/dbm/
├── archery_client.go
├── archery_config.go
├── archery_session.go
├── archery_slowquery.go
├── archery_sql.go
└── archery_types.go

nightingale/center/router/
└── router_dbm.go
```

### 前端新增文件
```
fe/src/pages/dbm/
├── index.tsx
├── index.less
├── locale/
│   ├── zh_CN.ts
│   ├── zh_HK.ts
│   └── en_US.ts
├── Sessions/
│   ├── index.tsx
│   └── index.less
├── SlowQueries/
│   ├── index.tsx
│   └── index.less
└── SQLQuery/
    ├── index.tsx
    └── index.less

fe/src/services/
└── dbm.ts
```

### 修改文件
```
nightingale/center/router/router.go  # 路由注册
fe/src/routers/index.tsx             # 前端路由
fe/src/components/SideMenu/menu.tsx  # 菜单配置
fe/src/components/SideMenu/locale/zh_CN.ts  # 菜单国际化
```

## 开发者

- 项目: Nightingale v6 + Archery 集成
- 时间: 2026-01-05
- 技术栈: Go 1.24, React 17, TypeScript 4.x, Ant Design 4.x

## 许可证

遵循夜莺项目许可证: Apache License 2.0
