# agent_gateway 破坏性重构开发计划

日期：2026-05-14

关联设计文档：

- `docs/2026-05-14/agent-gateway-mvc-gin-acp-refactor-plan.md`

## 目标

将 `agent_gateway` 重构为不兼容旧实现的新架构：

- 使用 Go + Gin + GORM + SQLite(no cgo) + WebSocket。
- 删除旧 `pkg/httpx` 主路由。
- 删除旧 CRUD alias：`create/update/delete/page/list/getById/status`。
- 删除旧 management settings 聚合接口。
- 使用 MVC 分层：Controller / Service / Repository / Model / Runtime。
- 使用对象注入：通过 `bootstrap.Container` 显式装配依赖。
- 通过 `acp-go-sdk` 连接 AI Agent，并通过 ACP extension methods 给 Agent 暴露网关管理能力。

## 不兼容声明

本开发计划默认破坏旧接口，不做兼容层：

- 不兼容旧 `httpx` 路由。
- 不兼容旧管理接口路径。
- 不兼容旧 management settings 聚合读写模型。
- 不保证 `agent_chat` 旧 gateway client 可继续工作。
- 旧测试需要删除或重写为新 REST API / WebSocket / ACP extension 测试。

## 总体里程碑

| 阶段 | 名称 | 目标 | 依赖 |
| --- | --- | --- | --- |
| M0 | 破坏性基线 | 移除旧兼容假设，恢复可编译 | 无 |
| M1 | Gin MVC 骨架 | 建立新应用装配、路由、响应模型 | M0 |
| M2 | SQLite + 统一 CRUD | 完成核心资源持久化与 REST API | M1 |
| M3 | MCP Runtime | MCP 服务连接、工具发现、调用与状态事件 | M2 |
| M4 | Skill Runtime | AI Skill 扫描、加载、重载、文档读取 | M2 |
| M5 | Scheduler Runtime | 定时任务执行、恢复、状态回写 | M2 |
| M6 | ACP 主链路 | Agent 进程管理、会话、事件、extension methods | M3/M4/M5 |
| M7 | 调用方迁移与收口 | agent_chat 迁移、新 smoke、文档完成 | M6 |

## 开发原则

- 先删除旧兼容假设，再实现新主链路。
- Controller 不直接访问数据库或 runtime manager。
- Service 负责业务编排、权限、事务和事件发布。
- Repository 只负责 GORM 持久化。
- Runtime manager 只负责长生命周期对象：ACP 连接、MCP 连接、Scheduler、Skill registry。
- 所有依赖通过构造函数注入，不使用包级全局单例。
- SQLite 必须使用 `github.com/glebarez/sqlite`，不得引入 `github.com/mattn/go-sqlite3`。

## M0：破坏性基线

目标：移除旧兼容假设，让项目恢复为可编译、可继续重构的状态。

开发 TODO：

- [x] 删除或绕开 `internal/app/wire.go` 旧装配路径。
- [x] 删除旧 `services.GatewayCRUD` 聚合接口。
- [x] 删除不可达的 `crudservice.NewGatewayCRUD` 引用。
- [x] 删除旧 `internal/handlers` 中依赖 `pkg/httpx` 的主路由注册。
- [x] 删除旧 CRUD alias 测试。
- [x] 删除旧 management settings 路由测试。
- [x] 统一 Agent 协议值为 `acp`。
- [x] 输出 breaking changes 清单。

验收 TODO：

- [x] `cd agent_gateway && go test ./...` 无编译失败。
- [x] 代码中不再有 gateway 主链路依赖 `pkg/httpx`。
- [x] 不再注册旧 CRUD alias。

## M1：Gin MVC 骨架

目标：建立新应用框架和依赖注入入口。

开发 TODO：

- [x] 增加 `github.com/gin-gonic/gin`。
- [x] 新建 `internal/bootstrap/container.go`。
- [x] 新建 `internal/bootstrap/router.go`。
- [x] 新建 `internal/bootstrap/lifecycle.go`。
- [x] 新建 `internal/controllers/response.go`。
- [x] 新建 `internal/controllers/health_controller.go`。
- [x] 新建 `internal/database/sqlite.go`。
- [x] 新建 `internal/database/migrate.go`。
- [x] 修改 `internal/runtime/server.go`，使用 Gin router。
- [x] 修改 `cmd/agent-gateway/main.go`，启动新 bootstrap。

验收 TODO：

- [x] `GET /health` 正常。
- [x] `go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once` 正常。
- [x] `go test ./internal/bootstrap ./internal/controllers ./internal/database` 通过。

## M2：SQLite + 统一 CRUD

目标：完成 Agent、AgentRole、MCPServer、ScheduleTask、Skill 的统一 CRUD。

开发 TODO：

- [x] 扩展 `models.BaseModel`：`ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt`。
- [x] 重建 `models.MCPServer`。
- [x] 重建 `models.Agent`。
- [x] 新建 `models.AgentRole`。
- [x] 重建 `models.ScheduleTask`。
- [x] 重建 `models.Skill`。
- [x] 新建 `internal/repositories/crud.go`。
- [x] 新建五类资源 repository。
- [x] 新建 `internal/services/crud.go`。
- [x] 新建五类资源 service。
- [x] 新建 `internal/controllers/crud_controller.go`。
- [x] 新建五类资源 controller。
- [x] 注册 REST API：
  - [x] `/v1/agents`
  - [x] `/v1/agent-roles`
  - [x] `/v1/mcp-servers`
  - [x] `/v1/schedule-tasks`
  - [x] `/v1/skills`
- [x] 实现分页、排序、过滤、软删除。
- [x] 实现启停接口：`PATCH /v1/:resource/:id/status`。

验收 TODO：

- [x] 五类资源 create/update/delete/get/list/page 测试通过。
- [x] `data_dir/gateway.db` 自动创建。
- [x] AutoMigrate 覆盖五类核心表。
- [x] `go list -deps ./... | Select-String "mattn/go-sqlite3"` 无输出。

## M3：MCP Runtime

目标：MCP 服务可连接、刷新工具、调用工具并发布事件。

开发 TODO：

- [x] 新建 `internal/runtime/mcp/manager.go`。
- [ ] 实现 MCP stdio 连接。
- [ ] 实现 MCP sse/http 连接。
- [x] 支持 env/envFile/header/cwd。
- [x] 支持 tools list。
- [x] tools 写回 `MCPServer.ToolsJSON`。
- [x] 创建/更新 MCP 后异步刷新 tools。
- [x] 禁用/删除 MCP 后关闭连接。
- [x] MCP 调用失败后重连一次。
- [ ] 发布 MCP runtime events。

验收 TODO：

- [x] `POST /v1/mcp-servers/:id/refresh-tools` 正常。
- [x] 禁用 MCP 后 runtime 状态变为 disconnected/disabled。
- [x] MCP manager 单测覆盖连接失败、刷新失败、重连、关闭。

## M4：Skill Runtime

目标：支持 AI Skill 扫描、加载、重载、启停和文档读取。

开发 TODO：

- [x] 新建 `internal/runtime/skills/registry.go`。
- [x] 新建 `internal/runtime/skills/loader.go`。
- [x] 新建 `internal/runtime/skills/watcher.go`。
- [x] 扫描 `data_dir/skills`。
- [x] 扫描 workspace skills。
- [x] 解析 `SKILL.md`。
- [x] 计算 content hash。
- [x] scan 结果同步 SQLite。
- [x] 实现 `POST /v1/skills/scan`。
- [x] 实现 `POST /v1/skills/:id/reload`。
- [x] 实现 `GET /v1/skills/:id/documentation`。

验收 TODO：

- [x] 新增 `SKILL.md` 后可发现。
- [x] 修改 `SKILL.md` 后 hash 更新。
- [x] 禁用 Skill 后 Agent 不再暴露该 Skill。
- [x] Skill 单测覆盖 scan/reload/invalid/not found。

## M5：Scheduler Runtime

目标：定时任务可执行、可恢复、可查询执行状态。

开发 TODO：

- [x] 新建 `internal/runtime/scheduler/scheduler.go`。
- [x] 新建 `internal/runtime/scheduler/runner.go`。
- [x] 支持 `cron`。
- [x] 支持 `every`。
- [x] 支持 `at`。
- [x] 支持 timezone。
- [x] 计算 `NextRunAt`。
- [x] 任务更新后 wake scheduler。
- [x] 执行前锁定任务，避免重复执行。
- [x] 执行后回写 `LastRunAt`、`LastStatus`、`LastError`。
- [x] 支持任务 payload 调用 Agent prompt。
- [x] 支持一次性任务执行后删除或禁用。

验收 TODO：

- [x] gateway 重启后恢复 enabled tasks。
- [x] 到期任务只执行一次。
- [x] 失败状态可查询。
- [x] Scheduler 单测覆盖 cron/every/at/disable/recovery。

## M6：Agent / AgentRole / ACP 主链路

目标：通过 ACP 连接外部 AI Agent，并允许 Agent 通过 extension methods 管理网关资源。

开发 TODO：

- [x] 新建 `internal/runtime/acp/client.go`。
- [x] 新建 `internal/runtime/acp/manager.go`。
- [x] 新建 `internal/runtime/acp/mapper.go`。（当前映射逻辑收敛在 `client.go`，后续需要更细投影时再拆文件。）
- [x] 新建 `internal/runtime/acp/extension_gateway.go`。（当前文件名为 `extension.go`。）
- [x] Agent 启用时启动 ACP 进程。
- [x] Agent 禁用/删除时停止 ACP 进程。
- [x] Agent 配置变化时 restart。
- [x] 实现 `Initialize` capability meta。
- [x] 实现 `NewSession`。
- [x] 实现 `Prompt`。
- [x] 实现 `Cancel`。
- [x] 实现 `CloseSession`。
- [x] 将 `SessionUpdate` 映射为 gateway event。
- [x] 将 `RequestPermission` 接入 Approval Service。
- [x] 实现 `_icoo.gateway/mcp.*`。
- [x] 实现 `_icoo.gateway/agent.*`。
- [x] 实现 `_icoo.gateway/agent-role.*`。
- [x] 实现 `_icoo.gateway/schedule.*`。
- [x] 实现 `_icoo.gateway/skill.*`。
- [x] 实现 AgentRole 权限策略。
- [x] 所有 extension method 写 audit event。

验收 TODO：

- [x] HTTP 创建 Agent 后 ACP 进程可启动。
- [x] 可创建 session 并发送 prompt。
- [x] WebSocket 可收到 ACP session update。
- [x] Agent 可通过 `_icoo.gateway/schedule.create` 创建任务。
- [x] 未授权 extension 调用返回结构化错误。

## M7：调用方迁移与收口

目标：删除旧实现，迁移调用方，完成文档和 smoke。

开发 TODO：

- [x] 删除旧 CRUD alias 残留。
- [x] 删除旧 management settings 聚合接口残留。
- [x] 删除旧 `pkg/httpx` 主路由相关代码。
- [x] 改造 `agent_chat` gateway client 到新 REST API。
- [x] 改造 `agent_chat` 事件订阅到新 WebSocket 协议。
- [x] 提供旧数据到新表结构的一次性迁移工具。
- [x] 更新 `agent_gateway/README.md`。
- [x] 更新 `agent_chat` 对接文档。
- [x] 新增 smoke：启动 gateway。
- [x] 新增 smoke：CRUD。
- [x] 新增 smoke：WebSocket。
- [x] 新增 smoke：ACP 最小链路。

验收 TODO：

- [x] `agent_chat` 已迁移到新接口。
- [x] 旧接口测试已删除或改写。
- [x] 新 REST API 测试通过。
- [x] smoke 脚本通过。
- [x] `cd agent_gateway && go test ./...` 通过。

## 并行开发建议

M2 完成后可并行：

- MCP Worker：负责 M3，只改 `internal/runtime/mcp`、MCP service/controller 测试。
- Skill Worker：负责 M4，只改 `internal/runtime/skills`、Skill service/controller 测试。
- Scheduler Worker：负责 M5，只改 `internal/runtime/scheduler`、Schedule service/controller 测试。

M6 必须等待 M3/M4/M5 的 Service 接口稳定后再合并。

## 当前阻塞

- [x] 当前 `agent_gateway` 存在编译失败，必须先完成 M0。
- [ ] 根仓库中 `acp-go-sdk`、`guada_ai`、`picoclaw`、`redka` 显示为未跟踪目录，提交前需要确认是否纳入版本控制。
- [ ] `.gitignore` 忽略 `*.json`，后续如需提交 JSON fixture 或 config example，需要调整策略。

## 验证命令

```powershell
cd E:\codes\icoo_ai\agent_gateway
go test ./...
go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once
go list -deps ./... | Select-String "mattn/go-sqlite3"
```
