# agent_gateway MVC + Gin + GORM + ACP 重构计划

日期：2026-05-14

## 目标

将 `icoo_ai/agent_gateway` 重构为独立本地网关服务，使用以下技术栈与结构：

- Go + Gin：HTTP API、WebSocket、统一中间件、统一响应。
- SQLite(no cgo) + GORM：使用 `github.com/glebarez/sqlite`，禁止引入 `mattn/go-sqlite3`。
- MVC：Controller 只处理协议入参和响应，Service 处理业务编排，Repository 处理数据库访问，Model 定义持久化结构。
- 对象注入：集中构建 `AppContainer`，所有 Controller/Service/Repository/Manager 通过构造函数注入依赖。
- 统一 CRUD：为 MCP 服务、Agent、定时任务、AI Skill、Agent 角色定义等管理资源提供泛型 Repository、Service 和 Controller 基类。
- ACP 集成：通过 `acp-go-sdk` 连接外部 AI Agent 工具，同时通过 ACP extension methods 暴露网关管理能力给 AI Agent，使 Agent 可动态增删改查 MCP 服务、定时任务、AI 技能、Agent 角色定义等资源。

本次重构采用破坏性设计：不兼容旧 `httpx` 路由、不兼容旧 `/create|/update|/delete|/page|/list|getById|status` CRUD alias、不兼容旧 management settings 聚合接口，也不保证 `agent_chat` 旧网关客户端无需修改。所有调用方需要迁移到新的 Gin REST API、WebSocket 事件协议和 ACP extension methods。

## 当前状态

当前重构状态：

- `agent_gateway` 已使用 Gin、GORM、`github.com/glebarez/sqlite`、`github.com/gorilla/websocket` 和 `github.com/coder/acp-go-sdk v0.12.2`。
- `internal/bootstrap` 已集中装配 Container、Router、Lifecycle、Repository、Service、Controller 和 Runtime Manager。
- `internal/controllers` 已承载统一响应、健康检查、五类资源 REST CRUD、Approval 和 WebSocket 事件接口。
- `internal/repositories` 与 `internal/services/admin` 已提供五类资源的统一 CRUD、分页、过滤、排序、启停和软删除。
- `pkg/wshub` 已适配 Gin Controller，当前事件入口为 `GET /v1/events` WebSocket。
- ACP runtime 已接入 Agent 进程生命周期、Agent-scoped session API、Approval broker 和 `_icoo.gateway/*` extension methods。
- 旧 `internal/handlers`、`pkg/httpx` 主路由、management settings 聚合接口和旧 CRUD alias 已删除。

## 参考项目可复用优点

### guada_ai

可借鉴点：

- NestJS 风格的模块边界清晰：Controller -> Service -> Repository，适合映射到 Go 的 MVC 分层。
- `mcp-servers` 模块在创建/更新后自动尝试刷新工具列表，失败不阻塞主体资源创建，可作为 MCP 服务管理的体验基线。
- `ToolOrchestrator` 使用 provider registry 管理不同工具命名空间，支持 `namespace__tool` 的工具命名、lazy/eager 加载和统一调用入口。
- `skills` 模块将 Discovery、Registry、Loader、VersionManager、Orchestrator 分开，可迁移为 Go 中的 Skill Manager 设计。

### picoclaw

可借鉴点：

- `pkg/mcp.Manager` 使用长期连接、并发安全 map、事件发布、失败重连、关闭等待 in-flight 调用，适合作为网关 MCP runtime manager 的核心参考。
- `pkg/cron.CronService` 的定时任务状态机完整：`nextRun`、`lastRun`、启停、一次性任务、wake channel、执行结果回写，可迁移到 SQLite/GORM 存储。
- `pkg/skills.RegistryManager` 支持多个 registry provider、并发搜索、安装目标校验，适合 AI Skill 管理扩展。
- `pkg/agent.AgentRegistry` 支持多 Agent 注册、默认 Agent、子 Agent 授权、MCP allowlist，可作为 Agent 角色定义与路由设计参考。
- `pkg/channels.Manager` 的事件发布、限流、重试、生命周期管理可以简化后迁移到 gateway runtime manager。

### acp-go-sdk

本地 `acp-go-sdk/README.md` 显示当前 SDK 用法：

- 连接外部 Agent：实现 `acp.Client`，通过 `acp.NewClientSideConnection(client, stdin, stdout)` 建立连接，然后 `Initialize`、`NewSession`、`Prompt`。
- 实现 Agent 端：实现 `acp.Agent`，通过 `acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)` 对外提供 ACP 能力。
- 自定义能力：使用 ACP extension methods，方法名以 `_` 开头，通过 `HandleExtensionMethod` 处理入站扩展，通过 `CallExtension`/`NotifyExtension` 调用扩展。

重构时建议把网关管理 API 提供为 ACP extension，而不是修改 ACP 标准方法。例如：

- `_icoo.gateway/mcp.create`
- `_icoo.gateway/mcp.update`
- `_icoo.gateway/mcp.delete`
- `_icoo.gateway/mcp.list`
- `_icoo.gateway/agent.create`
- `_icoo.gateway/schedule.create`
- `_icoo.gateway/skill.reload`

## 目标架构

```text
cmd/agent-gateway
  -> internal/bootstrap
       -> config
       -> database(gorm sqlite no-cgo)
       -> repositories
       -> services
       -> runtime managers
       -> controllers
       -> gin router

HTTP/WS Client
  -> Gin Controller
  -> Service
  -> Repository(GORM)
  -> SQLite

AI Agent(ACP stdio)
  <-> ACP Client Connection
  <-> ACP Bridge Service
  <-> Gateway Management Services
  <-> Repository/Runtime Managers
```

## 推荐目录结构

```text
agent_gateway/
  cmd/agent-gateway/main.go
  internal/bootstrap/
    container.go
    router.go
    lifecycle.go
  internal/config/
    config.go
    file.go
  internal/database/
    sqlite.go
    migrate.go
  internal/models/
    base.go
    agent.go
    agent_role.go
    mcp_server.go
    schedule_task.go
    skill.go
    audit_event.go
  internal/repositories/
    crud.go
    agent_repository.go
    agent_role_repository.go
    mcp_server_repository.go
    schedule_task_repository.go
    skill_repository.go
  internal/services/
    crud.go
    agent_service.go
    agent_role_service.go
    mcp_service.go
    schedule_service.go
    skill_service.go
    gateway_service.go
  internal/controllers/
    response.go
    crud_controller.go
    agent_controller.go
    agent_role_controller.go
    mcp_controller.go
    schedule_controller.go
    skill_controller.go
    health_controller.go
    websocket_controller.go
  internal/runtime/
    acp/
      client.go
      manager.go
      extension_gateway.go
      mapper.go
    mcp/
      manager.go
      tool_cache.go
    scheduler/
      scheduler.go
      runner.go
    skills/
      registry.go
      loader.go
      watcher.go
  pkg/wshub/
```

## 核心模型

### BaseModel

统一字段：

- `ID string`
- `CreatedAt time.Time`
- `UpdatedAt time.Time`
- `DeletedAt gorm.DeletedAt`
- `Enabled bool` 只放在需要启停的资源中，不强行塞入 Base。

### MCPServer

字段建议：

- `Name`
- `Description`
- `Type`: `stdio`、`sse`、`http`
- `URL`
- `Command`
- `ArgsJSON`
- `EnvJSON`
- `HeadersJSON`
- `Cwd`
- `ToolsJSON`
- `Enabled`
- `LastSyncAt`
- `LastError`

### Agent

字段建议：

- `Name`
- `Protocol`: `acp`
- `Command`
- `ArgsJSON`
- `EnvJSON`
- `Cwd`
- `Model`
- `RoleID`
- `MCPAllowlistJSON`
- `SkillAllowlistJSON`
- `Enabled`
- `Position`
- `RuntimeStatus`

### AgentRole

字段建议：

- `Name`
- `Description`
- `SystemPrompt`
- `DefaultModel`
- `ToolPolicyJSON`
- `MCPPolicyJSON`
- `SkillPolicyJSON`
- `CanSpawnAgentsJSON`
- `Enabled`

### ScheduleTask

字段建议：

- `Name`
- `Kind`: `cron`、`every`、`at`
- `CronExpr`
- `EveryMS`
- `AtMS`
- `Timezone`
- `PayloadJSON`
- `TargetAgentID`
- `Enabled`
- `NextRunAt`
- `LastRunAt`
- `LastStatus`
- `LastError`
- `DeleteAfterRun`

### Skill

字段建议：

- `Name`
- `Slug`
- `Version`
- `Description`
- `Path`
- `ManifestJSON`
- `ContentHash`
- `Source`
- `Enabled`
- `LastLoadedAt`
- `LastError`

## 统一 CRUD 设计

Repository 泛型接口：

```go
type CRUDRepository[T any] interface {
    Create(ctx context.Context, item *T) error
    Update(ctx context.Context, item *T) error
    Delete(ctx context.Context, id string) error
    GetByID(ctx context.Context, id string) (*T, error)
    List(ctx context.Context, q ListQuery) ([]T, int64, error)
}
```

Service 泛型接口：

```go
type CRUDService[T any, CreateReq any, UpdateReq any] interface {
    Create(ctx context.Context, req CreateReq) (*T, error)
    Update(ctx context.Context, id string, req UpdateReq) (*T, error)
    Delete(ctx context.Context, id string) error
    Get(ctx context.Context, id string) (*T, error)
    Page(ctx context.Context, q PageQuery) (PageResult[T], error)
    List(ctx context.Context, q ListQuery) ([]T, error)
}
```

HTTP 路由只采用 REST 风格：

- `GET /v1/mcp-servers`
- `POST /v1/mcp-servers`
- `GET /v1/mcp-servers/:id`
- `PUT /v1/mcp-servers/:id`
- `DELETE /v1/mcp-servers/:id`
- `PATCH /v1/mcp-servers/:id/status`
- `POST /v1/mcp-servers/:id/refresh-tools`

旧接口全部删除，不提供 alias：

- 删除 `/v1/*/create`
- 删除 `/v1/*/update`
- 删除 `/v1/*/delete`
- 删除 `/v1/*/page`
- 删除 `/v1/*/list`
- 删除 `/v1/*/getById`
- 删除 `/v1/*/status`

## 对象注入设计

建议新增 `bootstrap.Container`：

```go
type Container struct {
    Config config.Config
    DB *gorm.DB
    EventBus *events.Bus

    Repos Repositories
    Services Services
    Managers Managers
    Controllers Controllers

    Router *gin.Engine
}
```

构建顺序：

1. Load Config。
2. Open SQLite with `glebarez/sqlite`。
3. AutoMigrate。
4. New Repositories。
5. New EventBus。
6. New Services。
7. New Runtime Managers：ACP、MCP、Scheduler、Skill Registry。
8. New Controllers。
9. Register Gin routes。
10. Start lifecycle managers。

所有对象只通过构造函数注入依赖，避免包级单例。

## ACP 接入计划

### 作为 ACP Client 连接 AI Agent

`runtime/acp.Manager` 负责：

- 根据启用的 Agent 配置启动外部 ACP 进程。
- 创建 `acp.ClientSideConnection`。
- 初始化 capability：文件、terminal、extension metadata。
- 为每个 Agent 维护连接状态、会话映射、取消函数、stderr 日志。
- 将 `SessionUpdate` 转换为 gateway event，通过 WebSocket 推送。
- 将 `RequestPermission` 转为网关 Approval 记录，等待外部 HTTP/WS 客户端或策略决策。

### 给 AI Agent 暴露网关管理能力

在 `runtime/acp.GatewayExtensionHandler` 中实现 `acp.ExtensionMethodHandler`：

- 入站 `_icoo.gateway/*` 方法解析 JSON 参数。
- 调用对应 Service，例如 `MCPService.Create`、`ScheduleService.Update`、`SkillService.Reload`。
- 所有变更写审计日志并发布事件。
- 只允许在 Agent 权限策略允许的范围内调用，例如某个 Agent 只能管理自己的 schedule 或 allowlist 中的 MCP。

### 安全边界

- HTTP API 使用本地 bearer token。
- ACP extension 调用必须携带 agent identity，由 ACP Manager 根据连接绑定，而不是信任请求体。
- 对 `command`、`cwd`、`env`、文件路径做白名单/工作区限制。
- 敏感字段如 token/header/env 中的 secret 不直接返回，只返回 masked 值。

## WebSocket 设计

保留 `pkg/wshub` 的 JSON-RPC 事件模型，但适配 Gin：

- `GET /v1/events`
- 服务端 notification method：`event`
- 客户端 request method：
  - `gateway.ping`
  - `approval.decide`
  - `session.cancel`
  - `mcp.refreshTools`

事件类型建议：

- `mcp.server.connected`
- `mcp.server.failed`
- `mcp.tool.discovered`
- `agent.connected`
- `agent.disconnected`
- `agent.session.update`
- `schedule.task.due`
- `schedule.task.completed`
- `skill.loaded`
- `skill.failed`
- `crud.created`
- `crud.updated`
- `crud.deleted`

## 分阶段实施计划

### P0：恢复基线与边界冻结

目标：先冻结旧实现边界，确认删除范围，然后建立破坏性重构基线。

任务：

- 删除或隔离旧 `internal/app/wire.go` 装配路径，直接替换为新 `bootstrap.Container`。
- 删除旧 `pkg/httpx` 路由依赖，不再修复旧 management settings 路由测试。
- 删除旧 CRUD alias 测试，改写为新 REST API 测试。
- 清理 `AgentProtocol` 常量与默认 Agent profile，统一为新 `acp` 协议值。
- 明确不兼容清单，要求 `agent_chat` 后续按新 API 改造。

验收：

- `cd agent_gateway && go test ./...` 无编译失败。
- 不再存在旧 `httpx` router 主链路。
- 不再存在旧 CRUD alias 路由。

### P1：引入 Gin 与 MVC 骨架

目标：建立新 MVC 结构，但不一次性迁移所有业务。

任务：

- 增加 `github.com/gin-gonic/gin`。
- 新建 `internal/bootstrap`、`internal/controllers`、`internal/repositories`、`internal/database`。
- 用 Gin 注册 `/health` 和一个最小 CRUD 资源。
- 只注册新 REST API，不保留旧 API 兼容入口。

验收：

- `/health`、`/v1/mcp-servers` 可用。
- `go test ./internal/bootstrap ./internal/controllers ./internal/repositories` 通过。

### P2：SQLite + GORM 统一 CRUD

目标：把 Agent、AgentRole、MCPServer、ScheduleTask、Skill 统一迁移到 GORM Repository。

任务：

- 使用 `gorm.Open(sqlite.Open(path))`，驱动必须是 `github.com/glebarez/sqlite`。
- 实现 `CRUDRepository`、`CRUDService`、`CRUDController`。
- 实现统一分页、排序、过滤、启停、软删除。
- AutoMigrate 所有核心模型。
- 迁移当前 `internal/store/*` 的重复 CRUD 逻辑。

验收：

- 五类资源 CRUD 测试覆盖 create/update/delete/page/list/get/status。
- SQLite 文件落在 `data_dir/gateway.db`。

### P3：MCP 管理与工具发现

目标：形成可运行的 MCP 服务管理能力。

任务：

- 参考 `picoclaw/pkg/mcp.Manager` 实现 runtime MCP Manager。
- 支持 stdio、sse、http/streamable HTTP。
- 支持 env/envFile/header/cwd。
- 创建/更新 MCP 后异步刷新 tools，失败只写 `LastError`，不阻塞资源保存。
- tools 缓存到 `ToolsJSON`，并发布 `mcp.tool.discovered` 事件。

验收：

- `POST /v1/mcp-servers/:id/refresh-tools` 可刷新工具。
- 禁用 MCP 后 runtime 断开连接。
- MCP 调用失败可重连一次。

### P4：Skill 管理

目标：实现本地 AI Skill 的扫描、注册、加载、重载和启停。

任务：

- 参考 `guada_ai` 的 Discovery/Registry/Loader/Orchestrator 分层。
- 参考 `picoclaw` 的 registry provider 和安装目标校验。
- 支持扫描 `data_dir/skills`、工作区 `skills`、用户配置路径。
- Skill 记录持久化到 SQLite，内容 hash 用于变更检测。
- 提供 `GET /v1/skills/:id/documentation`、`POST /v1/skills/scan`、`POST /v1/skills/:id/reload`。

验收：

- 新增/修改 `SKILL.md` 后可 scan/reload。
- Agent 可通过 ACP extension 查询 skill 列表和文档。

### P5：定时任务 runtime

目标：定时任务不只是 CRUD，而是可执行、可恢复。

任务：

- 参考 `picoclaw/pkg/cron.CronService` 实现 scheduler loop。
- 使用数据库状态字段维护 `NextRunAt`、`LastRunAt`、`LastStatus`、`LastError`。
- 支持 `cron`、`every`、`at`。
- 任务 payload 支持向指定 Agent 发送 prompt，或调用某个 gateway extension action。
- 更新任务时 wake scheduler。

验收：

- 重启 gateway 后能从 SQLite 恢复任务。
- 到期任务只执行一次，执行结果可查询。

### P6：Agent 管理、角色定义与 ACP 会话主链路

目标：通过 ACP 连接 AI Agent，并让 Agent 使用 MCP/Skill/Schedule 管理能力。

任务：

- 实现 `AgentService` 与 `AgentRuntimeManager`。
- Agent 配置变更时动态启动/停止/restart ACP 连接。
- 实现 AgentRole 的 system prompt、tool policy、MCP allowlist、Skill allowlist。
- 将 ACP `SessionUpdate` 投影为 message/run/tool/approval events。
- 将 ACP permission request 接入 Approval Service。
- 实现 ACP extension methods 调用 CRUD Service。

验收：

- 可通过 HTTP 创建 Agent，启动 ACP 外部进程。
- 可新建会话、发送 prompt、接收流式事件。
- Agent 可调用 `_icoo.gateway/schedule.create` 创建任务。

### P7：破坏性迁移与收口

目标：完成破坏性切换后的调用方迁移、旧代码删除和文档收口。

任务：

- 删除旧 management settings 聚合接口。
- 删除旧 CRUD alias。
- 删除旧 `pkg/httpx` 相关代码和测试。
- 将 `agent_chat` 网关客户端迁移到新 REST API 和 WebSocket 事件协议。
- 提供旧 SQLite/配置数据到新表结构的一次性迁移工具；不保证 API 兼容。
- 更新 `agent_gateway/README.md` 和 `agent_chat` 对接说明，明确 breaking changes。

验收：

- `agent_chat` 已改造为只访问新接口。
- 旧接口测试已删除或改写。
- 新 REST 接口测试通过。

## 风险与决策

- Gin 与现有 `httpx` 不并存。重构后删除 `httpx` 主链路，必要时只保留 `pkg/wshub` 的 WebSocket 能力。
- `acp-go-sdk` extension methods 是管理能力的最佳承载方式，因为不会污染标准 ACP 方法。
- MCP runtime manager 与 MCP CRUD service 必须分离。CRUD 写库，runtime 负责连接生命周期；通过事件或 service hook 同步。
- Scheduler 必须使用数据库状态恢复，不能只依赖内存。
- Agent permission request 不能直接在 ACP client 里阻塞命令行输入，必须走 Approval Service。
- no-cgo SQLite 只能使用 `glebarez/sqlite`，CI 需要检查依赖树，避免误引入 `github.com/mattn/go-sqlite3`。

## 验收命令

```powershell
cd E:\codes\icoo_ai\agent_gateway
go test ./...
go test ./internal/controllers ./internal/services ./internal/repositories ./internal/runtime/...
go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once
```

依赖检查：

```powershell
go list -deps ./... | Select-String "mattn/go-sqlite3"
```

预期：无输出。

## 建议优先级

1. P0 恢复可编译基线。
2. P1-P2 完成 Gin MVC、对象注入、统一 CRUD、SQLite 持久化。
3. P3-P5 完成 MCP、Skill、Schedule 三类资源的 runtime 能力。
4. P6 完成 ACP 主链路和 extension 管理能力。
5. P7 做调用方迁移、旧实现删除和文档收口。

## 开发计划 TODO

说明：

- 每个阶段完成后必须更新本 TODO 状态。
- 默认按 P0 -> P7 顺序执行；P3、P4、P5 在 P2 完成后可并行推进。
- 每个阶段完成条件以“验收 TODO”全部勾选为准。

### P0：破坏性重构基线

目标：冻结删除范围，移除旧兼容假设，让 `agent_gateway` 进入可测试的新架构基线。

TODO：

- [x] 删除或绕开 `internal/app/wire.go` 旧装配路径。
- [x] 删除旧 `services.GatewayCRUD` 聚合接口，替换为分模块 Service。
- [x] 删除不可达的 `crudservice.NewGatewayCRUD` 引用。
- [x] 删除旧 `handlers` 管理配置路由测试，改写为新 REST 测试。
- [x] 统一 `AgentProtocol` 常量与默认 Agent profile 的协议值为 `acp`。
- [x] 记录 breaking changes 清单，作为调用方迁移依据。

验收 TODO：

- [x] `cd agent_gateway && go test ./...` 无编译失败。
- [x] 旧 `httpx` router 不再作为 gateway 主路由。
- [x] 旧 CRUD alias 不再注册。

### P1：建立 Gin MVC 骨架

目标：引入 Gin，并建立 Controller / Service / Repository / Model / Bootstrap 分层。

TODO：

- [x] 在 `agent_gateway/go.mod` 增加 `github.com/gin-gonic/gin`。
- [x] 新建 `internal/bootstrap/container.go`，定义 `Container`。
- [x] 新建 `internal/bootstrap/router.go`，统一注册 Gin 路由。
- [x] 新建 `internal/controllers/response.go`，封装统一响应与错误格式。
- [x] 新建 `internal/controllers/health_controller.go`，迁移 `/health`。
- [x] 新建 `internal/database/sqlite.go`，封装 GORM SQLite(no cgo) 初始化。
- [x] 新建 `internal/database/migrate.go`，集中执行 AutoMigrate。
- [x] 将 runtime server 改为持有 Gin router。
- [x] 只注册新 `/v1/*` REST 接口，不保留旧接口入口。

验收 TODO：

- [x] `GET /health` 正常返回。
- [x] `go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once` 正常退出。
- [x] `go test ./internal/bootstrap ./internal/controllers ./internal/database` 通过。

### P2：统一 CRUD 与 SQLite 持久化

目标：完成管理资源的统一 CRUD 能力，替换重复 store 代码。

TODO：

- [x] 新建 `internal/repositories/crud.go`，实现泛型 CRUD Repository。
- [x] 新建 `internal/services/crud.go`，实现泛型 CRUD Service。
- [x] 新建 `internal/controllers/crud_controller.go`，实现泛型 CRUD Controller helper。
- [x] 扩展 `BaseModel`：`ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt`。
- [x] 重构 `MCPServer` 模型，补齐 type/url/env/headers/tools/last error 字段。
- [x] 重构 `Agent` 模型，补齐 role、cwd、env、allowlist、runtime status 字段。
- [x] 新增 `AgentRole` 模型。
- [x] 重构 `ScheduleTask` 模型，补齐 kind/every/at/next/last/status 字段。
- [x] 重构 `Skill` 模型，补齐 slug/version/path/manifest/hash/source 字段。
- [x] 实现 `AgentRepository`、`AgentRoleRepository`、`MCPServerRepository`、`ScheduleTaskRepository`、`SkillRepository`。
- [x] 实现对应 Service。
- [x] 实现对应 Controller。
- [x] 注册 REST 路由。
- [x] 增加分页、排序、过滤、启停、软删除测试。

验收 TODO：

- [x] 五类资源均支持 create/update/delete/page/list/get/status。
- [x] SQLite 数据文件固定落到 `data_dir/gateway.db`。
- [x] `go list -deps ./... | Select-String "mattn/go-sqlite3"` 无输出。
- [x] `go test ./internal/repositories ./internal/services ./internal/controllers` 通过。

### P3：MCP 服务管理与工具发现

目标：MCP 服务从“配置记录”升级为可连接、可刷新工具、可调用的 runtime 能力。

TODO：

- [x] 新建 `internal/runtime/mcp/manager.go`。
- [ ] 支持 stdio transport。
- [ ] 支持 sse/http transport。
- [x] 支持 command path `~` 展开。
- [x] 支持 env/envFile/header/cwd。
- [x] 支持连接状态：connecting/connected/failed/disconnected。
- [x] 实现 tools list 刷新并写入 `ToolsJSON`。
- [x] 实现 create/update 后异步 refresh tools。
- [x] 实现禁用 MCP 时关闭 runtime connection。
- [ ] 实现 MCP tool call API。
- [x] 实现 session lost 后最多重连一次。

验收 TODO：

- [x] `POST /v1/mcp-servers/:id/refresh-tools` 可刷新 tools。
- [x] `GET /v1/mcp-servers/:id` 返回 tools 和 last error。
- [ ] MCP runtime 关闭时等待 in-flight tool call 完成。
- [x] MCP manager 单测覆盖连接失败、刷新失败、禁用关闭、重连。

### P4：AI Skill 管理

目标：实现 Skill 扫描、注册、加载、重载、启停和文档读取。

TODO：

- [x] 新建 `internal/runtime/skills/registry.go`。
- [x] 新建 `internal/runtime/skills/loader.go`。
- [x] 新建 `internal/runtime/skills/watcher.go`。
- [x] 支持扫描 `data_dir/skills`。
- [x] 支持扫描 workspace skills。
- [x] 支持解析 `SKILL.md` metadata。
- [x] 计算 content hash 并检测变更。
- [x] Skill scan 结果同步到 SQLite。
- [x] 实现 `POST /v1/skills/scan`。
- [x] 实现 `POST /v1/skills/:id/reload`。
- [x] 实现 `GET /v1/skills/:id/documentation`。
- [x] 实现 Skill allowlist 查询，供 AgentRole 使用。

验收 TODO：

- [x] 新增 `SKILL.md` 后 scan 能生成 Skill 记录。
- [x] 修改 `SKILL.md` 后 reload 能更新 hash。
- [x] 禁用 Skill 后 ACP extension 和 Agent tool list 不再暴露该 Skill。
- [x] Skill runtime 单测覆盖 scan/reload/not found/invalid manifest。

### P5：定时任务 Scheduler

目标：定时任务具备真实调度、状态恢复和执行结果回写能力。

TODO：

- [x] 新建 `internal/runtime/scheduler/scheduler.go`。
- [x] 新建 `internal/runtime/scheduler/runner.go`。
- [x] 支持 `kind=cron`。
- [x] 支持 `kind=every`。
- [x] 支持 `kind=at`。
- [x] 支持 timezone。
- [x] 实现 next run 计算。
- [x] 实现任务更新后 wake scheduler。
- [x] 实现任务执行前将 `NextRunAt` 置空，避免重复执行。
- [x] 实现执行结果回写 `LastRunAt`、`LastStatus`、`LastError`。
- [x] 支持一次性任务执行后删除或禁用。
- [x] 支持 payload 向指定 Agent 发送 prompt。

验收 TODO：

- [x] gateway 重启后能从 SQLite 恢复 enabled tasks。
- [x] 到期任务只执行一次。
- [x] 任务执行失败可查询错误。
- [x] Scheduler 单测覆盖 cron/every/at/disable/restart recovery。

### P6：Agent、AgentRole 与 ACP 主链路

目标：通过 ACP 连接外部 AI Agent，并让 Agent 能动态管理 Gateway 资源。

TODO：

- [x] 新建 `internal/runtime/acp/client.go`，封装 `acp.Client` 实现。
- [x] 新建 `internal/runtime/acp/manager.go`，管理 Agent ACP 进程生命周期。
- [x] 新建 `internal/runtime/acp/mapper.go`，转换 ACP update 到 gateway event。（当前映射逻辑收敛在 `client.go`，后续细化投影时再拆文件。）
- [x] 新建 `internal/runtime/acp/extension_gateway.go`，实现 extension methods。（当前文件名为 `extension.go`。）
- [x] 实现 Agent 启用时启动 ACP 连接。
- [x] 实现 Agent 禁用/删除时停止 ACP 连接。
- [x] 实现 Agent 配置变更时 restart。
- [x] 实现 `Initialize` capability meta，声明 `_icoo.gateway/*` 扩展能力。
- [x] 实现 `NewSession`、`Prompt`、`Cancel`、`CloseSession`。
- [x] 将 `SessionUpdate` 投影为 message/run/tool events。（当前先投影为统一 `acp.session_update` 事件。）
- [x] 将 `RequestPermission` 接入 Approval Service，不再阻塞 stdin。
- [x] 实现 `_icoo.gateway/mcp.*` extension methods。
- [x] 实现 `_icoo.gateway/agent.*` extension methods。
- [x] 实现 `_icoo.gateway/agent-role.*` extension methods。
- [x] 实现 `_icoo.gateway/schedule.*` extension methods。
- [x] 实现 `_icoo.gateway/skill.*` extension methods。
- [x] 实现 AgentRole 权限策略检查。
- [x] 所有 extension 写审计日志。

验收 TODO：

- [x] HTTP 创建 Agent 后能启动 ACP 外部进程。
- [x] 可通过 `/v1/agents/:id/sessions` 创建会话并发送 prompt。
- [x] WebSocket 可收到 ACP session update。
- [x] Agent 可通过 `_icoo.gateway/schedule.create` 创建定时任务。
- [x] 未授权 Agent 调用受限 extension 会返回结构化错误。

### P7：调用方迁移与文档收口

目标：完成破坏性切换后调用方迁移、旧实现删除和文档收口。

TODO：

- [x] 删除旧 CRUD alias。
- [x] 删除旧 management settings 聚合接口。
- [x] 删除旧 `pkg/httpx` 主路由相关代码。
- [x] 将旧 management settings 数据迁移到新表结构。
- [x] 为迁移失败提供可读错误和备份策略。
- [x] 改造 `agent_chat` 网关客户端，迁移到新 REST API。
- [x] 改造 `agent_chat` 事件订阅，迁移到新 WebSocket 事件协议。
- [x] 更新 `agent_gateway/README.md`。
- [x] 更新 `docs/agent-chat-gateway-bootstrap-integration-plan.md` 中相关说明。
- [x] 增加 smoke 脚本验证 gateway 启动、CRUD、WS、ACP 最小链路。
- [x] 增加 no-cgo SQLite 依赖检查到验证文档。

验收 TODO：

- [x] `agent_chat` 已完成新接口迁移。
- [x] 旧接口测试已删除或改写。
- [x] 新 REST 接口测试通过。
- [x] `cd agent_gateway && go test ./...` 通过。
- [x] smoke 脚本通过。

## 当前阻塞 TODO

- [x] `agent_gateway` 当前 `go test ./...` 存在编译失败，必须先处理 P0。
- [ ] 当前 `guada_ai`、`picoclaw`、`acp-go-sdk` 在根仓库显示为未跟踪目录，后续提交前需要确认这些目录是否应纳入版本控制。
- [ ] `.gitignore` 忽略 `*.json`，若后续需要提交配置样例或测试 fixture，需要单独调整或使用非 JSON 后缀。
