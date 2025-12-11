# AGENT.md 协作手册

## 文档目的
- 说明在本仓库中扮演 AI Coding Agent 时的工作方式、沟通准则与交付要求，防止上下文遗失。
- 汇总 CLAUDE.md 中的强制规范与额外定制规则，保证多次迭代后仍能对齐项目标准。

## 项目记忆
- **仓库结构速览**：根目录包含 `cmd`（启动入口）、`internal`（业务实现）、`configs`（配置）、`scripts/sql`（初始化 SQL）、`templates`（代码生成模板）、`docs`（Swagger 文档）等。`internal` 下遵循分层：`bootstrap` 负责依赖注入与路由；`api` 暴露 Gin Handler；`service` 承担业务逻辑；`repository` 读写数据库；`model` 定义 GORM 实体；`pkg` 存放通用组件（redis、jwt、generator、cron、eventbus 等）。
- **短链接核心流程**：
  - 创建：`internal/api/v1/shortlink.go` 根据登录态选择 `CreateForUser` 或 `CreateForGuest`，均在 `internal/service/shortlink.go` 内完成 URL 校验（`validateAndFormatURL`）、MD5 去重、事务写入 `shortlinks` 表并生成 Base62 短码，依赖 `repository.ShortlinkRepository` 与 `internal/pkg/generator`。
  - 列表与管理：`ListMyShortlinks`、`Update`、`Delete`、`UpdateStatus`、`ExtendExpiration` 等接口通过 `getAndCheckOwnership` 校验归属，更新后都会删除 Redis 缓存键 `cache:short_code:{code}`，避免脏读。
  - 跳转：公共路由 `/:shortCode` 调用 `ShortlinkHandler.Redirect`，服务层先查 Redis，命中 null 缓存可快速返回 404。未命中时采用 `SetNX` 实现的分布式锁防穿透，回写缓存带随机抖动；每次成功跳转都会向 `eventbus.PublishAccessLog` 投递访问事件，供异步统计使用。
- **统计与日志链路**：
  - `startLogConsumer`（`internal/bootstrap/router.go`）启动多 goroutine 消费 `eventbus`，`internal/service/log.go` 富化 IP、UA 并以 Pipeline 写入 Redis：一份 JSON 放入 `logs:buffer:raw`，同时维护 `stats:total:*`、`stats:daily:*`、`stats:region:*`、`stats:device:*` 等哈希。
  - `internal/service/writer.go` 的 `BatchWriterService` 由 `cron.InitCron` 周期性触发，`syncRawLogs` 将 Redis list 原子 rename 为临时 key，调用 `internal/repository/log.go` 写入月度分表 `access_logs_YYYYMM`；`syncStatsCounters` 扫描各类统计 key，批量 upsert 到 `stats_*` 表并在成功后删除临时 key。
  - `internal/service/stats.go` 通过 `StatsRepository` 读取 `shortlinks`、`stats_daily`、`stats_region_daily`、`stats_device_daily` 以及日志分表获取概览、趋势、地区、设备、来源、原始日志与全局统计；所有 `/api/v1/shortlinks/{code}/stats/*` 路由均经 `middleware.Auth` 校验当前用户对短链的所有权。
- **后台任务**：`internal/pkg/cron/cron.go` 初始化定时器，默认每 10 秒同步 Redis→DB，每天 20:35 执行 `MaintenanceService.CleanupOldLogs` 清理过期分表，需注意这些任务依赖 `config.yml` 中的生命周期配置。

## 沟通与反馈
- **所有输出（回答、文档、代码注释、Git 提交信息等）必须使用简体中文。**
- 回答保持精炼、结构清晰，必要时引用文件路径与行号，便于复检。
- 碰到需求或实现存在不确定性，必须立即向用户汇报并等待反馈，不得自行臆测或擅自实现。
- 若任务受限于环境（例如只读文件系统或缺少依赖），需明确记录受限原因与需要的协助。

## 协作流程
1. **需求理解**：阅读任务描述、CLAUDE.md 以及相关源文件，确认复用点与限制。
2. **计划阶段**：对于非极简任务先制定可执行计划，使用任务追踪工具更新状态。
3. **实现阶段**：
   - 复用现有模块与工具，遵循项目既有命名和代码风格。
   - 禁止破坏或覆盖用户已存在的未提交改动。
   - 任何较大操作前应列出现有方案或参考实现，避免重复造轮子。
4. **验证阶段**：在允许的情况下运行可复现的本地验证（单测、脚本等）；如无法执行需说明原因和替代方案。
5. **交付阶段**：总结变更点、测试结果与潜在风险，若功能完成须附上一条符合规范的 Git 提交信息。

## 开发准则
- 代码与文档遵循 CLAUDE.md 中的强制规则：
  - 简体中文唯一、UTF-8 编码、注释聚焦设计意图。
  - 遵循 SOLID/DRY、限制耦合、优先复用标准组件。
  - 禁止半成品/MVP 交付，提交内容需可直接使用。
  - 引入或修改功能前，至少对比三个现有实现（仓库内或参考实现），并记录差异。
- 性能、可靠性与可维护性需在设计阶段评估并记录，必要时补充监控或 TODO。

## 验证与记录
- 每次开发结束需在总结中记录：涉及文件、使用的复用组件、遵循的命名/风格规范、对比过的方案以及测试结果。
- 如无法执行验证脚本，需写明阻塞原因、影响面与建议的验证途径。
- 重要决策、外部约束或遗留风险应追加到 `operations-log.md`（若存在），保持可追溯性。

## Git 提交流程
- 每个已完成的功能或修复都要提供一条提议的提交信息，格式参照现有记录：`<类型>: <中文描述>` 或 `<类型>：<中文描述>`，例如 `fix：修复短链接统计日志路径`、`feat: 新增统计设备维度接口`。
- 类型统一使用 `feat`、`fix`、`docs`、`refactor`、`chore` 等约定前缀，描述须具体到本次改动的用户价值。
- 若任务尚未得到确认或存在疑问，不得编写提交信息，而是等待用户澄清。

## 记忆提示
- 遇到新的全局约束（如新的安全策略或交付模板）要在本文件追加条目，确保后续任务沿用。
- 若需要引用 CLAUDE.md 的特定规则，可在此文件内添加快捷摘要，减少反复查找。
