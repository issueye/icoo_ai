# agent_gateway 与 agent_server(icoo_server) 对接评估与开发落地报告

日期：2026-05-10

## 1. 目标与结论

本次工作目标：

1. 分析 `agent_gateway` 与 `agent_server` 的 ACP 对接状态并形成文档。  
2. 在 `agent_gateway` 增加 ACP 服务端池管理能力。  
3. 完成网关与 `agent_server` 的真实链路联通测试。

结论：

- 已完成 ACP 池化能力落地（配置、CLI、运行时装配、路由与单测）。
- 已修复网关 ACP method 映射为 ACP 标准 method（`session/new`、`session/list`、`session/resume`、`session/set_mode`、`session/set_config_option`、`session/prompt`、`session/cancel`、`session/close`），不再兼容旧 mock method（`newSession`/`prompt`/`cancel`）。
- 已完成真实环境联通验证，网关通过 HTTP 接口可完成会话创建、查询、恢复、模式切换、配置更新、prompt、取消与关闭，链路可用。

---

## 2. 当前对接架构

链路：

`agent_chat / 其他客户端 -> agent_gateway(HTTP/SSE) -> ACP Connector(stdio) -> agent_server(icoo-ai serve)`

网关 northbound 能力：

- `GET /v1/agents`
- `GET /v1/skills`
- `POST /v1/sessions`、`GET /v1/sessions`
- `GET /v1/sessions/{id}`
- `POST /v1/sessions/{id}/prompt`
- `POST /v1/sessions/{id}/cancel`
- `GET /v1/sessions/{id}/messages`
- `GET /v1/runs`
- `GET /v1/approvals`、`POST /v1/approvals/{id}/decision`
- `GET /v1/events/stream`

网关 southbound（当前直连 ACP）：

- `initialize`
- `session/new`
- `session/list`
- `session/resume`
- `session/set_mode`
- `session/set_config_option`
- `session/prompt`
- `session/cancel`
- `session/close`

---

## 3. 本次已落地改造

## 3.1 ACP 服务端池管理（gateway）

新增配置与参数：

- `acp.pool_size`（默认 `1`）
- CLI 参数 `-acp-pool-size`

新增池化连接器：

- 新增 `agent_gateway/internal/connectors/acp/pool.go`
- `Pool` 实现 `connector.AgentConnector`
- `NewSession` 轮询选择 backend，并记录 `sessionID -> backendIndex`
- `Prompt/Cancel` 按 session 绑定路由
- `Initialize` 对所有 backend 广播初始化（幂等缓存）
- `Close` 全量回收 backend，错误聚合返回

运行时装配升级：

- `runtime.Server` 从单 `*acp.Connector` 升级为 `connector.AgentConnector`
- 按 `acp.pool_size` 创建多个 backend 并封装为 pool
- `Shutdown` 时关闭整个 pool

## 3.2 ACP method 映射修正（不兼容旧 mock 协议）

原因：

- `agent_server` 基于 `acp-go-sdk` 的 method 名称是 `session/new` 等标准路径。
- 旧网关 connector 调的是 `newSession/prompt/cancel`，会返回 `Method not found`。

修正后：

- `NewSession` -> `session/new`
- `Prompt` -> `session/prompt`
- `Cancel` -> `session/cancel`
- 参数结构调整为 ACP 标准字段（如 `prompt` block、`mcpServers`、`cwd` 等）
- `initialize` 响应兼容 `agentInfo.name/version` 映射
- `session/update` 事件兼容 `session/update` 与 `session.update` 表达形式

说明：

- 本次按需求“不兼容之前”执行，旧 mock method 不再保留兼容分支。

---

## 4. 能力矩阵（agent_server vs gateway）

| ACP 能力 | agent_server | gateway 接入 |
| --- | --- | --- |
| initialize | 已实现 | 已接入 |
| session/new | 已实现 | 已接入 |
| session/prompt | 已实现 | 已接入 |
| session/cancel | 已实现 | 已接入 |
| session/close | 已实现 | 已接入 |
| session/list | 已实现 | 已接入 |
| session/resume | 已实现 | 已接入 |
| session/set_mode | 已实现 | 已接入 |
| session/set_config_option | 已实现 | 已接入 |

当前定位：

- 主链路（new/prompt/cancel）已可用。
- 会话管理增强链路（close/list/resume/mode/config）已完成 southbound 接入并通过实测。

---

## 5. 真实联通测试结果

## 5.1 agent_server ACP 完整能力自测（对照组）

命令：

```powershell
go -C agent_server run ./cmd/acp-real-client -llm-info ../docs/llm_info.txt -workspace E:/codes/icoo_ai -timeout 300
```

结果：通过。

关键输出：

- `initialize` 成功
- `newSession/listSessions/resumeSession/setSessionMode/setSessionConfigOption/prompt/closeSession` 全部成功
- `prompt stopReason=end_turn`

## 5.2 gateway connector real smoke（网关直连 agent_server）

命令（节选）：

```powershell
$env:ACP_SMOKE_TEST='1'
$env:ACP_SMOKE_COMMAND='go'
$env:ACP_SMOKE_ARGS='-C ../../../../agent_server run ./cmd/icoo-ai serve'
go test ./internal/connectors/acp -run TestRealProcessSmoke -count=1 -v
```

结果：通过。

- `Initialize/NewSession/Prompt/Cancel` 成功返回

## 5.3 gateway HTTP 端到端联通（pool_size=2）

测试动作：

- 启动网关（`-acp-enabled -acp-pool-size 2`）
- 调用 `GET /v1/agents`
- 连续创建 2 个会话 `POST /v1/sessions`
- 分别调用 `POST /v1/sessions/{id}/prompt`
- 分别调用 `POST /v1/sessions/{id}/cancel`

结果摘要：

```json
{
  "baseURL": "http://127.0.0.1:50736",
  "agentsCount": 1,
  "sessionIDs": [
    "sess_1778419583068954800",
    "sess_1778419583072601500"
  ],
  "promptRunIDs": [
    "run_000004",
    "run_000007"
  ],
  "cancelStatuses": [
    "cancelled",
    "cancelled"
  ]
}
```

结论：`gateway <-> agent_server` 实际可联通，主链路通过。

## 5.4 gateway HTTP 会话生命周期增强链路联通（pool_size=2）

测试动作：

- 启动网关（`-acp-enabled -acp-pool-size 2`，`acp-command` 使用预编译 `icoo-ai` 二进制）。
- 调用 `POST /v1/sessions`
- 调用 `GET /v1/sessions`
- 调用 `POST /v1/sessions/{id}/resume`
- 调用 `POST /v1/sessions/{id}/mode`
- 调用 `POST /v1/sessions/{id}/config`（`approval_mode=workspace-write`）
- 调用 `POST /v1/sessions/{id}/config`（`emit_plan_updates=true`）
- 调用 `POST /v1/sessions/{id}/prompt`
- 调用 `POST /v1/sessions/{id}/close`

结果摘要：

```json
{
  "baseURL": "http://127.0.0.1:52724",
  "agentsCount": 1,
  "sessionID": "sess_1778422867055573700",
  "listCountBefore": 9,
  "resumedStatus": "active",
  "modeAfterUpdate": "agent",
  "configValueCallStatus": "active",
  "configBoolCallStatus": "active",
  "promptRunID": "run_000003",
  "promptRunStatus": "completed",
  "promptMessageCount": 1,
  "closeStatus": "closed",
  "listCountAfter": 9
}
```

结论：会话生命周期增强链路已实测通过。

---

## 6. 变更文件（本次）

主要代码：

- `agent_gateway/internal/connector/types.go`
- `agent_gateway/internal/connector/connector.go`
- `agent_gateway/internal/connectors/acp/pool.go`
- `agent_gateway/internal/connectors/acp/pool_test.go`
- `agent_gateway/internal/runtime/server.go`
- `agent_gateway/internal/connectors/acp/connector.go`
- `agent_gateway/internal/connectors/acp/mapper.go`
- `agent_gateway/internal/service/types.go`
- `agent_gateway/internal/service/service.go`
- `agent_gateway/internal/service/service_test.go`
- `agent_gateway/internal/service/approval_broker_test.go`
- `agent_gateway/internal/api/routes.go`
- `agent_gateway/internal/api/sessions.go`
- `agent_gateway/internal/api/routes_test.go`
- `agent_gateway/internal/connectors/acp/connector_smoke_test.go`

文档：

- `docs/2026-05-10/agent-gateway-agent-server-integration-status-acp-pool-test-plan.md`

---

## 7. 剩余风险与后续计划

1. pool 启动方式约束（开发态）：
   - 在 `pool_size > 1` 时直接使用 `go run ./cmd/icoo-ai serve` 作为 `acp-command`，可能出现子进程快速退出；
   - 建议在池化场景下使用预编译 `icoo-ai` 二进制作为 `acp-command`（本次实测通过方式）。

2. 会话状态一致性增强：
   - 当前 `GET /v1/sessions` 已增加 connector 侧同步；
   - 后续可补充“关闭后清理策略/冷启动重建策略”的可配置开关。

3. 事件语义增强：
   - 进一步细化 `session/update` 到网关事件模型的映射（message/run/approval/tool_call）。

4. 池化观测性增强：
   - 增加 backend 维度日志和指标（`backend_index`、失败率、会话分布）。
