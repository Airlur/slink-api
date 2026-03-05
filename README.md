# slink-api

`slink-api` 是一个基于 Go 的短链接服务，支持：

- 长链接转短链接（支持自动短码与自定义短码）
- 短链访问跳转
- 用户认证与权限控制（普通用户/管理员）
- 访问日志采集与多维统计（趋势、地域、设备、来源）
- 标签与分享信息管理

项目当前是后端服务（Gin + MySQL + Redis），默认提供 REST API 与 Swagger 文档。

## Features

- 用户模块
  - 注册、登录、登出
  - Access Token / Refresh Token
  - 忘记密码与重置密码
  - 账号恢复流程
  - 管理员用户管理（列表、状态更新、强制下线）
- 短链模块
  - 游客创建短链（默认有效期）
  - 登录用户创建短链（支持自定义短码）
  - 短链详情、更新、删除、状态变更、有效期延长
  - `GET /:shortCode` 302 跳转
- 统计模块
  - 单短链概览、趋势、地域、设备、来源、访问日志
  - 用户聚合统计（overview/trend）
  - 管理员全局统计
- 其他
  - 标签管理（添加/删除/列表）
  - 分享信息管理（查询/更新）
  - 多级限流与缓存策略
  - 异步日志处理 + 定时批量落库

## Tech Stack

- Go `1.24.1`
- Gin
- GORM + MySQL
- Redis
- JWT
- Viper
- Zap
- Robfig Cron
- Swagger (`swaggo/gin-swagger`)

## Project Structure

```text
slink-api/
├─ cmd/                     # 程序入口
├─ configs/                 # 配置文件（config.yaml）
├─ docs/                    # Swagger 产物
├─ internal/
│  ├─ api/                  # Handler / Middleware / Routes
│  ├─ bootstrap/            # 启动初始化（DB、Redis、Router）
│  ├─ dto/                  # 请求/响应 DTO
│  ├─ model/                # 数据模型
│  ├─ repository/           # 数据访问层
│  ├─ service/              # 业务逻辑层
│  └─ pkg/                  # 公共组件（jwt、logger、redis 等）
├─ scripts/
│  ├─ sql/                  # 建表 SQL
│  └─ gen_module.bat        # 模块代码生成脚本
├─ templates/               # 代码生成模板
└─ Makefile
```

## Quick Start

### 1. Prerequisites

- Go `>= 1.24`
- MySQL `>= 8.0`
- Redis `>= 6.0`

### 2. Configure

编辑 `configs/config.yaml`，至少确认以下配置：

- `server.port`
- `database.*`
- `redis.*`
- `jwt.secret`
- `app.scheme` / `app.domain`
- `email.*`（如需邮件验证码）

### 3. Initialize Database

注意：当前代码里 `AutoMigrate` 仅包含 `users` 表。要使用完整功能，请先执行 SQL 脚本。

建议执行：

```bash
# 用户、短链、标签、分享
mysql -u <user> -p <database> < scripts/sql/user.sql
mysql -u <user> -p <database> < scripts/sql/shortlink.sql
mysql -u <user> -p <database> < scripts/sql/tag.sql
mysql -u <user> -p <database> < scripts/sql/share.sql

# 统计汇总表
mysql -u <user> -p <database> < scripts/sql/summary.sql

# 访问日志模板表（非常重要）
mysql -u <user> -p <database> < scripts/sql/accesslog.sql
```

`access_logs_template` 必须存在，因为运行时会按月动态创建 `access_logs_YYYYMM` 分表。

### 4. Run

```bash
make run
```

或：

```bash
go run cmd/main.go
```

服务默认监听 `:8080`（以 `configs/config.yaml` 为准）。

## Common Commands

```bash
make build       # 构建 bin/server
make run         # 本地运行
make test        # 运行测试
make clean       # 清理构建产物
make list-sql    # 列出可用 SQL 模板
make gen MODULE=shortlink
```

## API Entry

- Base URL: `http://localhost:8080`
- API Prefix: `/api/v1`
- Swagger: `/swagger/index.html`
- Redirect: `GET /:shortCode`

### Main API Groups

- Public
  - `POST /api/v1/register`
  - `POST /api/v1/login`
  - `POST /api/v1/users/token/refresh`
  - `POST /api/v1/shortlinks`（支持游客）
- User (Auth Required)
  - `/api/v1/users/*`
  - `/api/v1/shortlinks/*`
  - `/api/v1/tags/*`
  - `/api/v1/shares/*`
- Admin (Auth + Admin Role)
  - `/api/v1/admin/users/*`
  - `/api/v1/admin/stats/global`

## Stats Pipeline

访问短链后，统计流程是：

1. Redirect 时发布访问事件
2. 后台 Worker 做 UA/IP 解析并写入 Redis
3. Cron 定时任务将 Redis 统计与日志批量落库
4. 维护任务按保留周期清理过期日志分表

## Notes

- `data/regexes.yaml` 与 `data/IP2LOCATION-LITE-DB3.IPV6.BIN` 用于设备与地理解析，建议保留。
- 更完整的接口说明可参考 `API_REFERENCE.md` 与 Swagger 页面。
