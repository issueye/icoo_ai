# agent_chat × agent_gateway 对接复盘与缺口补齐开发计划（2026-05-10）

> 复盘时间：2026-05-10  
> 复盘范围：`E:/codes/icoo_ai` 当前代码  
> 基线要求：配置文件驱动（`chat.toml`），不依赖环境变量；去 mock，不兼容旧 mock 行为

## 1. 结论摘要

1. `agent_chat` 到 `agent_gateway` 的基础对接链路已经成型：启动拉起、状态查询、会话/消息/运行/审批/技能接口和 SSE 拉流都已具备。  
2. 当前主阻塞不在“有没有接口”，而在“是否真实可用且端到端一致”：`agent_gateway` 默认仍走 mock service 分支，ACP 启用链路未对外可配置，前端未消费 `agent:event`，上下文字段未全链路透传。  
3. 用户侧新增要求（host/port 配置、默认 `17889`、重启/关闭二次确认、slog 落盘、配置文件化）已落地，但重启后偶发“状态 ready 但事件流拒连”仍需专项修复。

## 2. 对接完成度矩阵（当前代码事实）

| 领域 | 完成度 | 现状 | 代码证据 |
| --- | --- | --- | --- |
| 网关生命周期托管 | ✅ 已完成 | chat 启动确保网关、支持重启/关闭，程序退出时停止**托管**网关进程 | `agent_chat/internal/bridge/agent_service.go` (`ServiceStartup/RestartGateway/StopGateway/ServiceShutdown`), `agent_chat/internal/bridge/gateway_bootstrap.go` |
| host/port 配置与默认值 | ✅ 已完成 | 设置页可配置 host/port，默认 `127.0.0.1:17889`，启动网关时通过 CLI 参数传入 | `agent_chat/internal/bridge/settings.go`, `agent_chat/internal/bridge/gateway_bootstrap.go` (`gatewayLaunchArgsFromSettings`) |
| 日志（slog + 配置化 + 落盘） | ✅ 已完成 | `agent_chat` 使用 slog，日志级别/格式/文件路径来自 `chat.toml`，默认落盘 `logs/agent_chat.log` | `agent_chat/internal/logging/logging.go`, `agent_chat/internal/bridge/settings.go` |
| 设置页 UED 下拉与二次确认 | ✅ 已完成 | 日志选项使用下拉组件；重启、关闭、保存后重启均为二次确认弹窗 | `agent_chat/frontend/src/components/settings/SettingsWorkspace.vue` |
| chat -> gateway 核心 HTTP 接口 | ✅ 已完成 | 已接 `/v1/sessions`、`/prompt`、`/cancel`、`/messages`、`/runs`、`/approvals`、`/skills` | `agent_chat/internal/bridge/agent_service.go` |
| SSE 链路（后端） | ✅ 已完成 | bridge 已持续拉取 `/v1/events/stream` 并发出 `agent:event` | `agent_chat/internal/bridge/agent_service.go`, `agent_chat/main.go` |
| SSE 链路（前端消费） | ❌ 未完成 | 前端未订阅 `agent:event`，当前仍以轮询/主动加载为主 | `agent_chat/frontend/src/components/app/AppShell.vue`, `agent_chat/frontend/src/stores/*` |
| Agent/模式/模型真实联动 | 🟡 部分完成 | UI 有 mode/model/workspace 选择，但数据源硬编码且未由 gateway 驱动 | `agent_chat/frontend/src/stores/conversations.js` |
| 上下文透传完整性 | 🟡 部分完成 | `NewSessionRequest`/`PromptRequest` 包含 `workspaceId/mode/model/cwd`，但映射到 gateway 请求时未全量透传 | `agent_chat/internal/bridge/types.go`, `agent_chat/internal/bridge/agent_service.go` (`mapCreateSessionRequest/mapPromptRequest`) |
| `/v1/agents` 对接 | ❌ 未完成 | gateway 已有 `/v1/agents`，chat bridge 和前端未接入 | `agent_gateway/internal/api/routes.go`, `agent_chat/internal/bridge/*`, `agent_chat/frontend/src/services/agentBridge.js` |
| 去 mock（no-mock 不兼容） | ❌ 未完成 | gateway runtime 在 `ACP.Enabled=false` 时仍装配 `NewMockGatewayServiceWithStore` | `agent_gateway/internal/runtime/server.go` |
| ACP 启用可配置性 | ❌ 未完成 | gateway CLI 当前仅 `host/port/data-dir/once`，无 ACP 参数；默认 ACP 关闭 | `agent_gateway/cmd/agent-gateway/main.go`, `agent_gateway/internal/config/config.go` |

## 3. 关键缺口与影响

### P0-1：默认仍可落入 mock 分支，违背 no-mock 目标

- 现状：`agent_gateway` 在 ACP 未启用时直接返回 mock service。  
- 影响：表面接口可用，但不是严格真实链路；一旦进入 Prompt 主链路，在无 connector 场景会出现 `connector_unavailable`，与“去 mock 不兼容”目标冲突。

### P0-2：ACP 启用链路不完整

- 现状：配置结构有 `ACP.Enabled/Command/Args`，但 CLI 没有对应参数；chat 启动网关时无法从配置文件传 ACP 参数。  
- 影响：无法稳定启用真实 connector，导致联调依赖手工改代码或非标准启动方式。

### P0-3：SSE 事件没有在前端生效

- 现状：后端在发 `agent:event`，前端没有 `EventsOn` 订阅。  
- 影响：消息、run、审批、网关状态无法走增量事件，体验退化为轮询，且状态延迟。

### P1-1：上下文映射断层（workspace/mode/model）

- 现状：前端传了上下文，bridge 映射只保留部分字段；gateway request schema 也未承接全部字段。  
- 影响：选择器“看起来可用”，但对后端行为影响不完整。

### P1-2：重启后 ready 但事件流拒连（稳定性缺陷）

- 现象（用户日志）：`RestartGateway` 返回 `gateway_ready` 后，随即多次 `dial tcp 127.0.0.1:17889 ... actively refused`。  
- 影响：状态与真实可用性不一致；用户误判“已恢复”。

## 4. 缺口补齐开发计划（No-Mock，不兼容旧行为）

### 阶段 A（P0）：彻底去 mock + ACP 启用闭环

1. runtime 移除 `ACP.Disabled -> MockGatewayService` 分支，改为“未配置即启动失败（结构化错误）”。  
2. gateway CLI 增加 ACP 参数：`-acp-enabled`、`-acp-command`、`-acp-args`（可多值或逗号分隔）。  
3. `chat.toml` 扩展 ACP 配置项；`agent_chat` 启动网关时统一带上 ACP 参数。  
4. 启动前做参数校验，错误直接返回 bridge 结构化错误，不回退 mock。

验收标准：

- 无 ACP 配置时，网关启动失败且错误可读；不会进入 mock。  
- 有 ACP 配置时，`CreateSession + Prompt + Cancel` 全链路可用。  
- `connector_unavailable` 不再作为“默认路径”出现。

### 阶段 B（P0）：前端接入实时事件

1. 在前端根组件或 store 初始化中订阅 `agent:event`。  
2. 按事件类型更新 `messages/runs/approvals/gatewayStatus`，并做 `event.id` 去重。  
3. 轮询仅保留为兜底（异常恢复），默认由事件驱动。

验收标准：

- Prompt 后消息/审批无需手动刷新即可出现。  
- 网关状态变化（connecting/reconnecting/failed/ready）能实时反映。

### 阶段 C（P1）：上下文与 agent 能力真实联动

1. bridge 新增 `ListAgents`，前端从 gateway 拉取 agent/mode/model 选项，移除硬编码列表。  
2. 扩展 gateway API request schema，明确 `agentId/workspaceId/mode/model/cwd` 字段契约。  
3. bridge 映射函数全量透传，前后端字段定义统一。

验收标准：

- 前端选择 agent/mode/model 后，后端行为可观测变化。  
- 新建会话与 prompt 的上下文在 gateway 侧可追踪。

### 阶段 D（P1）：重启/关闭稳定性增强

1. `ensureGatewayRunning` 从“单次 health 成功”升级为“稳定就绪判定”（连续健康检查 + 短窗口存活确认）。  
2. 事件流连接失败时，状态不能保持 ready，应降级为 reconnecting/failed。  
3. 增加托管进程退出监控（PID 存活探测），避免“假 ready”。

验收标准：

- 重启后不再出现“ready 立即拒连”假阳性。  
- 任意时刻网关崩溃，状态能在短时间内正确收敛。

### 阶段 E（P2）：测试与交付

1. 单测：bridge 映射、生命周期、事件订阅、ACP 参数拼装。  
2. 集成测试：chat 拉起 gateway、重启、关闭、退出联动。  
3. 文档同步：配置字段、错误码、排查手册、no-mock 不兼容声明。

验收标准：

- `go test ./...` 在 `agent_chat` 与 `agent_gateway` 通过。  
- 提供可复现的 smoke 脚本覆盖“启动-对话-重启-关闭”链路。

## 5. 三 worker 并发协同建议（按你要求）

1. Worker A（gateway runtime owner）  
职责：阶段 A（去 mock + ACP CLI/配置 + runtime 装配）  
写入范围：`agent_gateway/internal/runtime/*`, `agent_gateway/cmd/agent-gateway/*`, `agent_gateway/internal/config/*`

2. Worker B（chat bridge owner）  
职责：阶段 A/C/D 的 chat 侧实现（配置扩展、参数透传、稳定性判定）  
写入范围：`agent_chat/internal/bridge/*`, `agent_chat/internal/gatewayclient/*`, `agent_chat/internal/logging/*`

3. Worker C（frontend owner）  
职责：阶段 B/C 的前端事件消费与选项联动、UED 交互收口  
写入范围：`agent_chat/frontend/src/stores/*`, `agent_chat/frontend/src/components/*`, `agent_chat/frontend/src/services/*`

协同约束：

- 以 `gateway API contract` 文档为唯一字段真相源。  
- 不做向后兼容；删除旧 mock 和硬编码回退。  
- 每阶段先合约后实现，再联调再回归。

## 6. 当前测试基线

2026-05-10 本地执行：

- `cd agent_chat && go test ./...`：通过  
- `cd agent_gateway && go test ./...`：通过

说明：单测通过不代表联调闭环完成，P0 缺口（去 mock + ACP 配置闭环 + 前端事件消费）仍是上线阻塞项。
