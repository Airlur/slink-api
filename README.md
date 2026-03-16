# slink-api

`slink-api` 是短链接平台的后端服务，提供短链创建、跳转与多维统计能力，默认提供 REST API 与 Swagger 文档。

## 核心能力

- 短链：游客/登录创建（自动短码与自定义短码）、详情更新删除、状态变更、有效期延长、`GET /:shortCode` 跳转
- 用户与权限：注册/登录/刷新 Token/登出、账号恢复、管理员用户管理（列表/状态/强制下线）
- 统计与分析：单链概览/趋势/地域/设备/来源/日志/地图；用户聚合 overview/trend/regions/devices/sources/top-links；来源趋势、标签分析、dashboard-actions；中国/世界地图统计与单链对比指标；时间范围与对比统计；管理员全局统计
- 其他：标签管理、分享信息管理、多级限流与缓存、异步日志处理与定时聚合

## 技术栈

- Go 1.24.1
- Gin
- GORM + MySQL
- Redis
- JWT
- Viper
- Zap
- Robfig Cron
- Swagger

## 目录结构

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
├─ scripts/                 # 开发脚本与 SQL
├─ templates/               # 代码生成模板
└─ Makefile
```

## 快速开始

### 1. 环境要求

- Go `>= 1.24`
- MySQL `>= 8.0`
- Redis `>= 6.0`

### 2. 配置

编辑 `configs/config.yaml`，至少确认以下配置：

- `server.port`
- `database.*`
- `redis.*`
- `jwt.secret`
- `app.scheme` / `app.domain`
- `email.*`（如需邮件验证码）

### 3. 初始化数据库

确保数据库结构已初始化（可参考项目内 SQL 脚本）。

### 4. 运行

```bash
make run
```

或：

```bash
go run cmd/main.go
```

服务默认监听 `:8080`（以 `configs/config.yaml` 为准）。

## API 入口

- Base URL: `http://localhost:8080`
- API Prefix: `/api/v1`
- Swagger: `/swagger/index.html`
- Redirect: `GET /:shortCode`

## 常用命令

```bash
make build       # 构建 bin/server
make run         # 本地运行
make test        # 运行测试
make clean       # 清理构建产物
make list-sql    # 列出可用 SQL 模板
make gen MODULE=shortlink
```

## Swagger 文档更新

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/main.go -o docs
```

## 相关仓库

- 前端 Web：`https://github.com/Airlur/slink-web`

## 许可证

本项目采用 `Apache-2.0` 协议，详见 `LICENSE`。

## 备注

- 数据解析依赖 `data/` 目录下的规则与库文件，请保留。
- 更完整的接口说明可参考 `API_REFERENCE.md` 与 Swagger 页面。
