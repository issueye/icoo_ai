# agent_chat 启动唤醒 agent_gateway 与网关对接开发计划

> 日期：2026-05-15
> 状态：`agent_gateway` 已完成 Gin + GORM + SQLite(no cgo) + WebSocket + ACP 重构；本文档聚焦 `agent_chat` 启动唤醒、连接状态治理与剩余对接收敛。

## 1. 范围与目标

本计划仅覆盖两条主线：

1. **启动链路**：`agent_chat` 在 `ServiceStartup` 阶段确保本地 gateway 可用（发现、健康检查、拉起、重试）。
2. **服务对接链路**：`agent_chat/internal/bridge` 通过 gateway REST API + WebSocket 事件通道工作，移除旧 mock/fallback 路径对主链路的影响。

不在本计划内：

- 前端 UI 视觉改版。
- 远程网关部署（仅本机 `127.0.0.1`）。
- 非 ACP connector 的新增协议实现。
- 兼容旧版 gateway API。

## 2. 当前基线（代码现状）

### 2.1 已完成能力

- `agent_gateway` 已重构为 Gin HTTP 服务，使用 GORM + SQLite(no cgo) 持久化。
- `agent_gateway` 已提供 `/health` 与 `/v1/*` 路由，并写入 `endpoint.json` + `token` 供本地发现。
- 事件通道已切换为 WebSocket：`GET /v1/events`。
- `agent_chat` 已迁移到 WebSocket 事件订阅，不再依赖 SSE。
- 会话 API 已改为 Agent-scoped：
  - `POST /v1/agents/:id/sessions`
  - `POST /v1/agents/:id/sessions/:sessionId/prompts`
  - `POST /v1/agents/:id/sessions/:sessionId/cancel`
  - `DELETE /v1/agents/:id/sessions/:sessionId`
- ACP connector 已进入 gateway 主链路，prompt/cancel/事件转发不再以旧 mock service 为主。

### 2.2 已移除或不再兼容的旧接口

以下旧接口不再作为开发或兼容目标：

- `GET /v1/events/stream` SSE 事件流。
- 全局 `/v1/sessions/*` 会话接口。
- `GET/PUT /v1/management/settings` 管理设置接口。

任何 `agent_chat` 侧调用、测试或文档引用都应改写为当前 WebSocket 与 Agent-scoped API。

### 2.3 当前缺口

- `agent_chat` **没有**“发现失败后自动拉起 `agent_gateway`”机制（当前 `loadGatewayProxy()` 只发现，不启动）。
- 启动后网关可用性缺少统一状态机（连接中/已连接/失败原因）。
- WebSocket 断线重连、鉴权失败、endpoint/token 重新发现仍需要按新协议硬化。
- 旧 mock/fallback 只能作为开发诊断路径，不能掩盖 gateway 主链路错误。

## 3. 目标架构（启动后流程）

1. `agent_chat` 启动。
2. bridge 执行 `ensureGatewayRunning()`：
   - 读取 endpoint + token；
   - 调用 `/health`；
   - 不可用则启动 `agent-gateway` 进程；
   - 等待 endpoint 文件与健康检查通过。
3. bridge 建立 gateway client，并连接 WebSocket：`GET /v1/events`。
4. `NewSession/Prompt/Cancel/DeleteSession` 统一走 Agent-scoped gateway API。
5. 当 gateway 异常退出或 WebSocket 断开时，bridge 触发重新发现、重连与可观测状态事件。

### 3.1 agent_chat 调用映射

| agent_chat 操作 | gateway API |
| --- | --- |
| 订阅事件 | `GET /v1/events` WebSocket |
| 创建会话 | `POST /v1/agents/:id/sessions` |
| 发送 prompt | `POST /v1/agents/:id/sessions/:sessionId/prompts` |
| 取消运行 | `POST /v1/agents/:id/sessions/:sessionId/cancel` |
| 删除会话 | `DELETE /v1/agents/:id/sessions/:sessionId` |

## 4. 分阶段实施计划

### P0：旧协议引用清理（已完成/持续校验）

**目标**：确保开发计划、代码注释和测试不再把旧 SSE、全局 session 或 management settings 当作目标接口；旧接口只允许出现在“不再兼容”的说明中。

任务项：

1. 移除或改写所有 `/v1/events/stream` 引用，统一为 WebSocket `GET /v1/events`。
2. 移除或改写所有 `/v1/sessions/*` 引用，统一为 `/v1/agents/:id/sessions/*`。
3. 移除或改写所有 `/v1/management/settings` 引用。

验收标准：

- 新增设计、任务项与测试仅描述当前 gateway API。
- 兼容旧接口不再作为验收要求。

### P1：启动唤醒能力（待完成）

**目标**：`agent_chat` 启动后，无需手工先启动 gateway。

改动范围：

- `agent_chat/internal/bridge/agent_service.go`
- 新增：`agent_chat/internal/bridge/gateway_bootstrap.go`
- 新增测试：`agent_chat/internal/bridge/gateway_bootstrap_test.go`

任务项：

1. 新增 `GatewayBootstrapper`：
   - `Discover()`
   - `HealthCheck()`
   - `StartGatewayProcess()`
   - `WaitUntilReady(timeout)`
2. 启动命令策略（Windows 优先）：
   - 环境变量显式指定：`ICOO_GATEWAY_BIN`
   - 回退到同目录候选：`agent-gateway.exe`
   - 最后回退：`go run ./agent_gateway/cmd/agent-gateway`（仅开发模式）
3. `AgentService.ServiceStartup` 改为：
   - 先 `ensureGatewayRunning`
   - 成功后再建立 gateway client
   - 最后连接 WebSocket `GET /v1/events`
4. 失败处理：
   - 返回结构化错误码 `gateway_bootstrap_failed`
   - 记录 stderr 日志文件路径（若有）

验收标准：

- 清空 endpoint 文件后启动 `agent_chat`，可自动拉起 gateway 并通过 `/health`。
- gateway 已运行时不重复拉起。
- 启动失败时，前端能收到明确错误而非静默 fallback。

### P2：连接状态与 WebSocket 重连治理（待完成）

**目标**：把“连接中/已连接/重连中/失败”变成可观测状态，避免隐式失败。

改动范围：

- `agent_chat/internal/bridge/agent_service.go`
- `agent_chat/internal/bridge/types.go`
- 前端桥接适配：`agent_chat/frontend/src/services/agentBridge.js`（仅状态事件透出）

任务项：

1. 新增连接状态事件：
   - `gateway_connecting`
   - `gateway_ready`
   - `gateway_reconnecting`
   - `gateway_failed`
2. WebSocket 断线策略：
   - 指数退避重连；
   - 连续失败阈值后切换为 `gateway_failed` 并停止无穷重试；
   - 重连前重新读取 endpoint + token，避免旧 token 造成循环失败。
3. 鉴权失败处理：
   - 401/403 不做无意义重试；
   - 重新发现 endpoint/token 后最多重试一次；
   - 仍失败则返回 `gateway_auth_failed`。

验收标准：

- 运行中 kill 掉 gateway，客户端进入重连状态并可在 gateway 恢复后自动恢复 WebSocket 事件通道。
- 鉴权失败时给出明确 `gateway_auth_failed`。
- WebSocket 重连不会退回旧 SSE 路径。

### P3：业务接口收敛（基本完成，保留回归项）

**目标**：`agent_chat` 的核心操作完全依赖当前 gateway 真实能力。

当前状态：

- gateway 侧主链路已由 ACP connector 承接。
- prompt/cancel/delete session 均应通过 Agent-scoped API。
- 旧 mock assistant 文案不应再作为 UI 主链路输出来源。

剩余任务项：

1. 回归 `agent_chat` 创建会话、发送 prompt、取消运行、删除会话是否全部使用 Agent-scoped API。
2. 确认 prompt HTTP 返回仅表示 run 已接收，增量内容由 WebSocket 事件推送。
3. 审批链路闭环：
   - connector -> approval broker -> approval decision API -> connector 回写。

验收标准：

- `agent_chat` 发起 prompt 后，可在 UI 收到真实 agent 增量事件。
- cancel 与 approval 在 UI、gateway、connector 三侧状态一致。
- 未发现对旧版事件流、全局会话接口或 management settings 接口的运行时依赖。

### P4：发布前硬化

**目标**：上线前稳定性与可运维性达标。

改动范围：

- `agent_chat/README.md`
- `agent_gateway/README.md`
- `scripts/build.ps1` / `scripts/build.sh`（如需）

任务项：

1. 文档补充：
   - 自动唤醒机制；
   - gateway 可执行文件定位规则；
   - WebSocket 事件订阅路径；
   - Agent-scoped 会话 API；
   - 常见错误与排查步骤。
2. E2E 验证脚本：
   - 启动 chat -> 自动拉起 gateway -> 建会话 -> prompt -> cancel -> delete session。
3. 兼容性验证：
   - Windows 首验；
   - Linux/macOS 至少保证 discovery、health、WebSocket 路径正确。

验收标准：

- 新机器按 README 启动即可自动完成网关拉起与聊天链路。
- `go test ./...`（`agent_chat`、`agent_gateway`）通过。

## 5. 任务拆分建议

1. **Worker A（chat bootstrap）**
负责 `agent_chat/internal/bridge/gateway_bootstrap*.go` 与启动时序改造。
2. **Worker B（chat websocket/state）**
负责 `AgentService` 状态机、WebSocket 重连、错误码与状态事件。
3. **Worker C（chat API convergence）**
负责清理 `agent_chat` 侧旧接口调用，确保全部使用 Agent-scoped API。
4. **Worker D（tests/e2e/docs）**
负责集成测试、回归脚本与文档补齐。

## 6. 风险与缓解

- 风险：启动命令路径不稳定。
  缓解：优先 `ICOO_GATEWAY_BIN`，并把最终命令写入诊断日志。

- 风险：重复拉起多个 gateway 进程。
  缓解：启动前先 health check；读取 endpoint PID 并做进程存活判断。

- 风险：鉴权 token 与 endpoint 不一致。
  缓解：发现后必须同时读取 endpoint + token；401/403 触发重新发现。

- 风险：WebSocket 重连使用过期 endpoint/token。
  缓解：每轮重连前重新发现；鉴权失败只允许有限重试。

- 风险：开发 fallback 掩盖真实问题。
  缓解：生产模式禁用 fallback；开发模式 fallback 默认告警并可配置关闭。

## 7. 里程碑与交付件

1. **M0（gateway 重构完成）**：Gin + GORM + SQLite(no cgo) + WebSocket + ACP 主链路就绪。
交付：当前 gateway API、WebSocket 事件通道、Agent-scoped session 接口。

2. **M1（P1 完成）**：自动唤醒可用，启动不再依赖手工先启 gateway。
交付：bootstrap 代码 + 单元测试。

3. **M2（P2 完成）**：连接状态、WebSocket 重连和错误行为稳定。
交付：状态事件与断线恢复测试。

4. **M3（P3 回归完成）**：业务主链路确认全部使用真实 gateway connector。
交付：prompt/cancel/delete/approval 端到端通路。

5. **M4（P4 完成）**：文档与回归验证完成，可进入发布候选。
交付：README、E2E 脚本、回归报告。
