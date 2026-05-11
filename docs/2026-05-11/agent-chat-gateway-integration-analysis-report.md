# agent_chat 与网关对接分析报告（2026-05-11）

## 1. 结论摘要

当前 `agent_chat` 与 `agent_gateway` 已完成可运行的主链路对接，覆盖了：

- 启动托管：`agent_chat` 启动时自动发现/拉起网关。
- 鉴权访问：通过 `endpoint.json + token` 动态发现网关地址并携带 Bearer Token。
- 业务接口：会话、消息、Prompt、取消、审批、技能、Agent 列表均已贯通。
- 实时流：SSE 事件流已接入，包含重连、鉴权失败识别、失败收敛状态。
- 前端映射：网关事件已分发到消息、运行、审批、审计与应用状态 Store。

综合评估：**对接状态为“已贯通并可用”，但仍有若干中风险项建议继续收敛**（见第 6 节）。

---

## 2. 端到端调用链

1. `agent_chat` 启动 `ServiceStartup`，先置 `gateway_connecting`，调用 `ensureGatewayRunning`。  
   参考：`agent_chat/internal/bridge/agent_service.go:61,73,194`
2. `gatewayBootstrapper.EnsureRunning` 先发现已运行网关；失败后启动进程并轮询健康。  
   参考：`agent_chat/internal/bridge/gateway_bootstrap.go:64`
3. 网关启动后写运行时文件 `endpoint.json/token`，并对 `/v1/*` 开启 Bearer 鉴权。  
   参考：`agent_gateway/internal/runtime/server.go:55,70,89,200`；`agent_gateway/internal/runtime/endpoint.go:31`
4. `agent_chat` 读取 `endpoint.json/token` 建立 `gatewayProxy`。  
   参考：`agent_chat/internal/gatewayclient/discovery.go:29,46,50,57,72`
5. 启动前会探活 SSE；成功置 `gateway_ready`，失败置 `gateway_reconnecting` 并后台重连。  
   参考：`agent_chat/internal/bridge/agent_service.go:88,233,457`
6. 前端通过 Wails Bridge 调用 Go 服务；Go 服务通过 HTTP 调用网关。  
   参考：`agent_chat/frontend/src/services/agentBridge.js:1`；`agent_chat/internal/bridge/agent_service.go:1139`

---

## 3. 接口对接匹配情况

`agent_chat` 当前调用的网关接口：

- `GET /v1/agents`：`ListAgents`  
  `agent_chat/internal/bridge/agent_service.go:428`
- `GET/POST /v1/sessions`、`GET /v1/sessions/{id}`  
  `agent_chat/internal/bridge/agent_service.go:321,336,346`
- `POST /v1/sessions/{id}/prompt`  
  `agent_chat/internal/bridge/agent_service.go:352,362`
- `POST /v1/sessions/{id}/cancel`  
  `agent_chat/internal/bridge/agent_service.go:374,376`
- `GET /v1/sessions/{id}/messages`、`GET /v1/runs`、`GET /v1/approvals`、`POST /v1/approvals/{id}/decision`、`GET /v1/skills`

网关路由均已提供对应实现：  
`agent_gateway/internal/api/routes.go:24,30,32,33,37`，`agent_gateway/internal/api/sessions.go:72,163,177`

结论：**接口路径与方法已对齐，无明显“调用端存在、服务端缺失”的断链点。**

---

## 4. 事件流与状态机对接情况

- 网关提供 `/v1/events/stream`（SSE），支持 `sessionId/agentId` 过滤与 `Last-Event-ID`。  
  `agent_gateway/internal/api/events.go:15`
- `agent_chat` 事件流处理具备：
  - 指数退避重连；
  - 鉴权错误直接失败收敛；
  - 最大失败次数阈值（8 次）后置 `gateway_failed`。  
  `agent_chat/internal/bridge/agent_service.go:457`
- 事件类型会映射到前端统一 Kind（message/tool_call/tool_result/approval/run/audit 等）。  
  `agent_chat/internal/bridge/agent_service.go`（`mapGatewayEventTypeToBridgeKind`）
- 前端 `agentEvents` 已对接全局事件总线并分发至多 Store。  
  `agent_chat/frontend/src/services/agentEvents.js:28`

结论：**状态机与事件流闭环完整，具备基础容错能力。**

---

## 5. 配置与启动参数对接

- `chat.toml` 由 `agent_chat` 读写，覆盖网关地址、二进制路径、ACP 参数、日志参数。  
  `agent_chat/internal/bridge/settings.go`
- 启动网关时会把上述配置映射为启动参数：
  - `-host/-port`
  - `-acp-enabled/-acp-command/-acp-args`  
  `agent_chat/internal/bridge/gateway_bootstrap.go:319`
- 网关侧校验：
  - host 必须 loopback；
  - `acp.enabled=true` 时必须提供 `acp.command`。  
  `agent_gateway/internal/config/config.go:38`

结论：**配置项在 chat 与 gateway 之间已形成明确契约，且有基本参数校验。**

---

## 6. 风险与改进建议（按优先级）

### P1（建议尽快）

1. SSE 重放语义存在边界偏差风险  
   `events.Bus.snapshotSince` 在命中 `lastEventID` 后会把该事件本身也包含进回放，依赖前端去重兜底。跨客户端/多消费者场景可能出现重复消费。  
   参考：`agent_gateway/internal/events/bus.go:92`

2. 网关服务实现命名仍为 `MockGatewayService`  
   当前已承载真实 ACP 主链路，命名会误导维护与排障。  
   参考：`agent_gateway/internal/service/service.go`

### P2（中期优化）

1. `agent_chat` 事件订阅默认按“当前会话”过滤；多会话并发时可观测性依赖 `activeSessions` 推断，建议显式策略化（全量/当前会话/会话集合）。  
   参考：`agent_chat/internal/bridge/agent_service.go`（`streamSubscriptionState`）

2. 网关鉴权 token 每次启动重置，桌面端通过文件发现可恢复，但外部调试工具连接会断，建议补充运维说明与可选固定 token 策略。

---

## 7. 验证结果

本次在本地代码执行了双侧单测回归：

- `agent_chat`: `go test ./...` 通过（`internal/bridge`、`internal/gatewayclient` 通过）
- `agent_gateway`: `go test ./...` 通过（`api/runtime/service/connectors` 等通过）

判定：**当前对接实现与测试基线一致，未发现阻断级问题。**

---

## 8. 最终评估

`agent_chat` 与网关当前已经具备生产前联调能力，核心链路（启动托管、鉴权访问、业务接口、SSE 实时事件、前端状态同步）均已完成。下一阶段建议优先处理 SSE 回放边界和服务命名语义问题，再补多会话事件订阅策略与运维可观测性细节。
