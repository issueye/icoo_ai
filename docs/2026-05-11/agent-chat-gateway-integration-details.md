# agent_chat 与 agent_gateway 对接详情梳理（2026-05-11）

## 1. 目标与结论

本文基于当前仓库代码，梳理 `agent_chat` 与 `agent_gateway` 的真实对接状态（非仅文档口径）。

结论：

- 主链路已打通：网关启动/发现、鉴权、业务接口调用、SSE 事件流接入均可运行。
- 存在对接偏移：`README` 与冒烟脚本中的 ACP 配置口径，和网关当前实际参数/配置实现不一致。
- 建议先收敛配置契约，再处理 SSE 回放边界与命名语义问题。

---

## 2. 启动与发现链路

### 2.1 `agent_chat` 启动流程

1. `ServiceStartup` 调用 `ensureGatewayRunning`，保障网关可用。  
   代码：`agent_chat/internal/bridge/agent_service.go`
2. 若已有网关实例可探活，则复用；否则由 `gatewayBootstrapper` 拉起网关并轮询健康。  
   代码：`agent_chat/internal/bridge/gateway_bootstrap.go`
3. 启动后读取 `endpoint.json + token` 生成网关代理客户端。  
   代码：`agent_chat/internal/gatewayclient/discovery.go`
4. 首次探测事件流可用性，成功置 `gateway_ready`，失败置 `gateway_reconnecting` 并后台重连。  
   代码：`agent_chat/internal/bridge/agent_service.go`

### 2.2 网关运行时行为

1. 网关启动后绑定本地地址，生成随机或指定端口。  
   代码：`agent_gateway/internal/runtime/server.go`
2. 写入运行时文件（endpoint/token）供客户端发现。  
   代码：`agent_gateway/internal/runtime/server.go`, `agent_gateway/internal/runtime/endpoint.go`
3. `/v1/*` 统一 Bearer Token 鉴权。  
   代码：`agent_gateway/internal/runtime/server.go`

---

## 3. 接口对接矩阵

`agent_chat` 侧调用（Go bridge）：

- `GET /v1/agents`
- `GET /v1/skills`
- `GET /v1/sessions`
- `POST /v1/sessions`
- `GET /v1/sessions/{id}`
- `POST /v1/sessions/{id}/prompt`
- `POST /v1/sessions/{id}/cancel`
- `GET /v1/sessions/{id}/messages`
- `GET /v1/runs`
- `GET /v1/approvals`
- `POST /v1/approvals/{id}/decision`
- `GET /v1/events/stream`

调用入口：`agent_chat/internal/bridge/agent_service.go`

网关路由已提供对应处理器：  
`agent_gateway/internal/api/routes.go`

评估：接口路径与方法当前已对齐，无明显“客户端调用存在但网关缺失”断点。

---

## 4. 事件流（SSE）对接细节

### 4.1 网关侧

- 事件流端点：`GET /v1/events/stream`
- 支持 `Last-Event-ID`（header）与 `lastEventId`（query）回放起点
- 支持 `sessionId`、`agentId` 过滤
- 15 秒 keep-alive 注释帧

代码：`agent_gateway/internal/api/events.go`

### 4.2 `agent_chat` 侧

- 使用 `StreamEventsWithFilter` 建立订阅
- 支持 `Last-Event-ID` 续传
- 失败指数退避重连
- 鉴权失败收敛为 `gateway_failed`
- 达到最大失败次数后停止重连

代码：`agent_chat/internal/gatewayclient/client.go`, `agent_chat/internal/bridge/agent_service.go`

### 4.3 事件映射

网关事件类型会归一到前端事件 Kind（`message/tool_call/tool_result/approval/run/audit/subagent`）。  
代码：`mapGatewayEventTypeToBridgeKind` in `agent_chat/internal/bridge/agent_service.go`

---

## 5. 配置契约现状（重点）

### 5.1 当前实际生效的网关启动参数

`agent_chat` 在拉起网关时目前仅传：

- `-host`
- `-port`

代码：`gatewayLaunchArgsFromSettings` in `agent_chat/internal/bridge/gateway_bootstrap.go`

网关 CLI 目前仅支持：

- `-host`
- `-port`
- `-once`

代码：`agent_gateway/cmd/agent-gateway/main.go`

### 5.2 当前实际生效的网关配置文件键

`config/agent-gateway.toml` 读取器当前仅支持：

- `host`
- `port`
- `data_dir`

代码：`agent_gateway/internal/config/file.go`

---

## 6. 对接偏移与风险

## P1：ACP 配置契约偏移（优先修复）

- `agent_chat/README.md` 仍声明 `acp_enabled/acp_command` 等配置要求。
- `scripts/smoke-gateway-chat.ps1` 仍使用 `-acp-enabled/-acp-command/-acp-args` 启动参数。
- 但网关当前 CLI/配置加载器并不支持这些参数和键。

影响：

- 按文档配置可能与真实行为不一致；
- 冒烟脚本在当前实现下存在失效风险；
- 联调与排障成本上升。

## P2：服务命名语义偏移

- 网关核心实现类型仍命名为 `MockGatewayService`，但已承载真实主链路能力。

影响：维护者容易误判代码角色，排障效率降低。

## P2：SSE 回放边界风险

- `snapshotSince` 为内存 ring 回放，断连过久且事件被覆盖时可能产生回放缺口。

代码：`agent_gateway/internal/events/bus.go`

---

## 7. 测试状态

已执行：

- `agent_gateway`: `go test ./...` 通过
- `agent_chat`: `go test ./...` 通过

说明：当前测试基线与“主链路可运行”结论一致，但不能覆盖全部契约偏移问题（尤其文档/脚本层面的漂移）。

---

## 8. 建议的收敛动作

1. 统一配置契约（优先）  
   在“README、smoke 脚本、网关 CLI、网关配置读取”中选择一个一致方案，并一次性同步。

2. 清理命名语义  
   将 `MockGatewayService` 更名为更贴近实际职责的名称（例如 `GatewayServiceImpl` / `InMemoryGatewayService`）。

3. 强化事件回放策略  
   为 SSE 回放缺口增加显式告警与补偿策略（如回退全量消息拉取或会话级重扫）。

