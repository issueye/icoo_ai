# Agent Gateway 多任务并行阶段计划

> 本计划基于 `docs/acp-gateway-service-design.md`，用于把独立 `agent_gateway` 服务拆成可并行推进、可阶段验收的任务。执行时每个 worker 只修改自己负责的文件集合，避免覆盖彼此改动。

## 1. 目标

将网关层从 `agent_chat` 桌面进程逐步拆出，形成独立本地服务。第一阶段先建立 `agent_gateway/` 服务骨架，后续再把会话、事件流、ACP connector、审批和持久化迁入 gateway。

阶段完成后的目标形态：

- `agent_gateway` 能独立启动并暴露 `/health`。
- gateway 默认只监听 `127.0.0.1`，支持随机端口。
- gateway 启动时生成本地 token，并写入 endpoint 文件。
- gateway API、service、connector、store、安全模块边界清晰。
- `agent_chat` 可以通过轻量 `gatewayclient` 发现并调用 gateway。
- 后续 ACP、CLI、Remote connector 能在不改 UI API 的前提下接入。

## 2. 当前基线

已有：

- `docs/acp-gateway-service-design.md`：独立 gateway 总体设计。
- `agent_chat/`：Wails 3 + Vue 桌面端骨架，当前 bridge 使用 mock 数据。
- `agent_server/`：已有 `icoo-ai serve` ACP 服务、Agent Runtime、工具、审批、Subagent、Skills、MCP、审计日志。
- `agent_gateway/`：独立 gateway Go module、服务入口、`/health`、本地 token、endpoint 文件、`/v1/` mock API。
- `agent_gateway/internal/store`：线程安全内存 store，覆盖 conversation、message、run、approval、audit 的最小安全摘要字段。
- `agent_chat/internal/gatewayclient`：读取 endpoint/token 并调用 gateway `/health` 的轻量 client。

尚未存在：

- `agent_chat/internal/bridge` 到 gatewayclient/gateway API 的真实转调。
- gateway SSE 事件流与 ApprovalBroker。
- gateway store 与 service 的真实接线和 json/jsonl 落盘。
- gateway 与 ACP server 的真实连接。

## 3. 当前进度快照

更新时间：2026-05-09

| 阶段 | 状态 | 说明 |
|---|---|---|
| P1：独立服务骨架 | 已完成 | `agent_gateway` 可独立启动，`/health` 可用，随机端口、token、endpoint 文件已落地。 |
| P2a：三 worker 并行批次 | 已完成 | Worker B/API-Service、Worker C/Store、Worker F/gatewayclient 已完成并由主线程集成。 |
| P2b：bridge 接入 gateway | 未开始 | 需要把 `agent_chat/internal/bridge` 从 mock-only 改为优先使用 gatewayclient，保留开发 fallback。 |
| P3：事件流与审批闭环 | 未开始 | 需要 SSE event bus、ApprovalBroker、bridge event subscription。 |
| P4：ACP connector | 未开始 | 需要 connector registry 和 `icoo-ai serve` stdio 接入。 |
| P5：持久化与恢复 | 未开始 | 需要 json/jsonl store 接线、重启恢复、事件补拉。 |
| P6：多 Agent 并发 | 未开始 | 需要多 connector profile、事件/审批/取消隔离验证。 |

已通过验证：

```text
cd agent_gateway && go test ./...
cd agent_chat && go test ./internal/gatewayclient
```

## 4. 并行策略

采用 6 个 worker 并行推进。每个 worker 拥有明确写入范围，主线程负责阶段计划、集成、冲突处理和最终验收。

| Worker | 名称 | 负责范围 | 不允许修改 |
|---|---|---|---|
| A | Gateway Core Worker | `agent_gateway` module、cmd、config、health、endpoint/token | `agent_chat`、ACP connector、store 业务 |
| B | API / Service Worker | HTTP routes、DTO、sessions/prompts/runs/approvals mock service | connector 具体实现、桌面 UI |
| C | Store Worker | 内存 store、json/jsonl 持久化接口和测试 | HTTP handler、connector |
| D | Event / Approval Worker | SSE 事件总线、ApprovalBroker、事件 envelope | store 落盘细节、ACP stdio |
| E | ACP Connector Worker | `icoo-ai serve` stdio connector、ACP 事件映射 | gateway HTTP client、UI |
| F | agent_chat Client Worker | `agent_chat/internal/gatewayclient`、bridge 转调 gateway | gateway 内部服务、前端大改 |

主线程集成职责：

- 维护本计划和阶段进度。
- 先落地 P1 纵向薄片。
- 每个阶段结束运行对应测试。
- 发现公共 DTO 冲突时统一裁剪字段。

## 5. 文件所有权

### Worker A：Gateway Core

负责创建或修改：

```text
agent_gateway/go.mod
agent_gateway/cmd/agent-gateway/main.go
agent_gateway/internal/config/config.go
agent_gateway/internal/api/health.go
agent_gateway/internal/security/token.go
agent_gateway/internal/runtime/endpoint.go
agent_gateway/internal/runtime/server.go
agent_gateway/README.md
```

要求：

- 默认 host 为 `127.0.0.1`。
- 默认 port 为 `0`，由系统分配随机端口。
- `/health` 返回 gateway version、status、capabilities。
- token 使用加密随机数生成。
- endpoint 文件写入用户数据目录的 `icoo-ai/gateway/endpoint.json`。

验收：

- 已完成：`go test ./...` 通过。
- 已完成：`go run ./cmd/agent-gateway -data-dir ./.tmp-gateway -once` 可验证 endpoint/token 写入。
- 已完成：`/health` 可返回 JSON。

### Worker B：API / Service

负责创建或修改：

```text
agent_gateway/internal/api/routes.go
agent_gateway/internal/api/sessions.go
agent_gateway/internal/api/approvals.go
agent_gateway/internal/service/service.go
agent_gateway/internal/service/types.go
```

要求：

- 实现第一批 HTTP JSON API：
  - `GET /v1/agents`
  - `POST /v1/sessions`
  - `GET /v1/sessions`
  - `GET /v1/sessions/{sessionId}/messages`
  - `POST /v1/sessions/{sessionId}/prompt`
  - `POST /v1/sessions/{sessionId}/cancel`
  - `GET /v1/runs`
  - `GET /v1/approvals`
  - `POST /v1/approvals/{approvalId}/decision`
- 第一阶段可以使用 mock connector 和内存 store。
- 错误响应统一包含 `code`、`message`。

验收：

- 已完成：httptest 覆盖 session 创建、prompt、cancel、approval decision。
- 已完成：runtime 层对 `/v1/` 做 bearer token 校验；session 不存在返回结构化 JSON 错误。

### Worker C：Store

负责创建或修改：

```text
agent_gateway/internal/store/store.go
agent_gateway/internal/store/memory.go
agent_gateway/internal/store/jsonl.go
agent_gateway/internal/store/types.go
```

要求：

- Store 接口先稳定，不绑定具体 HTTP handler。
- 支持 conversation、message、run、approval、audit 的最小字段。
- json/jsonl 落盘只保存安全摘要，不保存完整大工具输出。
- 写入操作要尊重 context cancellation。

验收：

- 已完成：MemoryStore 单元测试覆盖 upsert/list、message 按 session 过滤、approval 更新。
- 未开始：json/jsonl 落盘、重启读取、损坏 jsonl 行策略。

### Worker D：Event / Approval

负责创建或修改：

```text
agent_gateway/internal/events/bus.go
agent_gateway/internal/events/types.go
agent_gateway/internal/api/events.go
agent_gateway/internal/service/approval_broker.go
```

要求：

- GatewayEvent 使用统一 envelope：
  - `id`
  - `type`
  - `agentId`
  - `sessionId`
  - `runId`
  - `payload`
  - `createdAt`
- 先实现 SSE：`GET /v1/events/stream`。
- 支持 last event id 的基础补拉接口设计，第一阶段可只保留内存 ring buffer。
- ApprovalBroker 必须以 `agentId/sessionId/runId/connectorRequestId` 定位审批。

验收：

- httptest 能读取 SSE event。
- 两个 session 的审批不会串线。
- cancel 后 pending approval 能过期或拒绝。

### Worker E：ACP Connector

负责创建或修改：

```text
agent_gateway/internal/connector/connector.go
agent_gateway/internal/connector/types.go
agent_gateway/internal/connectors/acp/connector.go
agent_gateway/internal/connectors/acp/mapper.go
agent_gateway/internal/connectors/acp/process.go
```

要求：

- Gateway 核心只依赖 `AgentConnector` 接口。
- ACP connector 负责启动 `icoo-ai serve` stdio 进程。
- 支持 initialize、newSession、prompt、cancel 的最小闭环。
- 映射 ACP session update 到 GatewayEvent。
- stdout 仅用于 ACP 协议，日志必须走 stderr 或 gateway audit。

验收：

- 使用 fake ACP process 覆盖协议映射。
- 真实 `icoo-ai serve` smoke test 可手动运行。
- ACP protocol error 能变成结构化 gateway 错误。

### Worker F：agent_chat Client

负责创建或修改：

```text
agent_chat/internal/gatewayclient/client.go
agent_chat/internal/gatewayclient/discovery.go
agent_chat/internal/gatewayclient/types.go
agent_chat/internal/bridge/agent_service.go
agent_chat/internal/bridge/types.go
```

要求：

- 先保留前端现有 bridge API。
- bridge 内部通过 gatewayclient 调用 gateway。
- gateway 不可用时开发模式可以 fallback 到 mock，但生产模式应返回明确错误。
- gatewayclient 读取 endpoint/token 文件，调用 `/health` 验证版本和 capabilities。

验收：

- 已完成：`gatewayclient` 单元测试覆盖 endpoint/token 读取、Authorization header、非 2xx 错误。
- 未开始：bridge 单元测试覆盖 gateway 可用、不可用、token 错误。
- 前端 bindings 暂不强制重生成，除非 bridge DTO 变更。

## 6. 阶段拆分

### P1：独立服务骨架（已完成）

范围：

- Worker A 为主。
- Worker F 只做 discovery/client 草案，不改前端行为。

交付：

- `agent_gateway/` Go module。
- `cmd/agent-gateway`。
- `/health`。
- config 默认值。
- token 和 endpoint 文件。
- 单元测试。

验收：

- 已通过：`cd agent_gateway && go test ./...`。
- 已完成：gateway 监听地址为 `127.0.0.1:<随机端口>`。
- 已完成：endpoint.json 包含 `pid/baseUrl/tokenFile/startedAt`。

### P2：保持 UI API 不变（进行中）

范围：

- Worker B、C、F。

交付：

- 已完成：gateway sessions/messages/runs/approvals mock API。
- 已完成：`agent_gateway/internal/store` MemoryStore 基础能力。
- 已完成：`agent_chat/internal/gatewayclient` 可发现 gateway 并调用 `/health`。
- 未开始：`agent_chat` bridge 转调 gateway client。
- 未开始：事件由 bridge 映射成现有 `agent:event`。

验收：

- 未完成：`agent_chat` UI 能创建会话、发送 prompt、取消运行。
- 未完成：无 gateway 时有清晰连接状态或开发 fallback。

### P3：事件流与审批闭环

范围：

- Worker D、B、F。

交付：

- SSE 事件流。
- ApprovalBroker。
- `/v1/approvals/{id}/decision`。
- `agent_chat` 订阅 event stream 并发出 Wails event。

验收：

- prompt 能产生 user/assistant/tool/approval event。
- 审批卡片决策能回到 gateway。

### P4：接入 `icoo-ai` ACP connector

范围：

- Worker E、B、D。

交付：

- `AgentConnector` registry。
- `icoo-ai-acp` 默认 profile。
- ACP stdio process 管理。
- ACP update 到 GatewayEvent 的映射。

验收：

- gateway 能启动或连接 `icoo-ai serve`。
- prompt/cancel 能通过 ACP 跑通。
- ACP 进程退出后 run 标记 failed/cancelled。

### P5：持久化与恢复

范围：

- Worker C、D、B。

交付：

- conversations/messages/runs/audit 落盘。
- 重启恢复会话列表和历史消息。
- 事件 ring buffer 和补拉策略。

验收：

- gateway 重启后历史会话仍可查询。
- 敏感字段和完整大工具输出不落盘。

### P6：多 Agent 并发

范围：

- Worker E、B、D、F。

交付：

- `GET /v1/agents`。
- 会话支持 `agentId`、`model`。
- 至少一个 mock/CLI/remote connector spike。
- 多 Agent 同时运行的事件隔离测试。

验收：

- 两个 agent/session 同时运行时，事件、审批、取消互不串线。
- UI 会话和运行记录都带 `agentId`。

## 7. 下一批建议

下一批建议先执行 3 个 worker，继续保持文件边界清晰：

| Worker | 任务 | 写入范围 | 验收 |
|---|---|---|---|
| F1 | bridge 接入 gatewayclient | `agent_chat/internal/bridge/**`，必要时只读 `agent_chat/internal/gatewayclient/**` | bridge 优先探活 gateway；gateway 不可用时开发 fallback；原前端 API 不变。 |
| D1 | SSE event bus | `agent_gateway/internal/events/**`、`agent_gateway/internal/api/events.go` | httptest 能订阅 `/v1/events/stream` 并收到 event envelope。 |
| C1 | json/jsonl store spike | `agent_gateway/internal/store/**` | conversations/messages/runs/audit 可落盘并重启读取，测试覆盖损坏行策略。 |

主线程集成点：

1. 将 event bus 挂到 `runtime/server.go`。
2. 决定 mock service 是否直接使用 Store 接口，或保留独立 mock service 到 P3 后再替换。
3. bridge 接入后运行 `agent_chat` Go 测试，并视 DTO 变更决定是否重生成前端 bindings。

## 8. 风险与约束

- gateway 必须默认只绑定 `127.0.0.1`。
- endpoint/token 文件应尽量只允许当前用户读取。
- ACP stdio 模式下不能向 stdout 写普通日志。
- `agent_chat` 前端 API 不应在 P1/P2 大改。
- 多 worker 并行时，公共 DTO 先以最小字段为准，避免提前绑定 ACP 细节。
- 不把 gateway 构建混入 `agent_chat/frontend` 的 `npm run build` 链路。
- P2 目前 service mock 和 Store 仍是两条线，下一步需要决定是否在 P3 前统一接线。
- `/health` 当前无鉴权，`/v1/` 已有 bearer token 校验；事件流接入时必须沿用同一 token 策略。
