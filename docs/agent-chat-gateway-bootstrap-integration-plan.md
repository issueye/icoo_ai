# agent_chat 启动唤醒 agent_gateway 与网关对接开发计划

> 日期：2026-05-09  
> 目标：`agent_chat` 启动后自动唤醒 `agent_gateway`，并以 gateway 作为唯一会话/事件入口，逐步替换当前 mock/fallback 路径。

## 1. 范围与目标

本计划仅覆盖两条主线：

1. **启动链路**：`agent_chat` 在 `ServiceStartup` 阶段确保本地 gateway 可用（发现、健康检查、拉起、重试）。
2. **服务对接链路**：`agent_chat/internal/bridge` 全量通过 gateway API + 事件流工作，逐步收敛 mock fallback。

不在本计划内：

- 前端 UI 视觉改版。
- 远程网关部署（仅本机 `127.0.0.1`）。
- 非 ACP connector 的新增协议实现。

## 2. 当前基线（代码现状）

### 2.1 已具备能力

- `agent_gateway` 已可独立启动，提供 `/health` 与 `/v1/*` 路由，并写入 `endpoint.json` + `token`。
- `agent_chat/internal/gatewayclient` 已支持 endpoint/token 发现、`/health` 请求、SSE 事件订阅。
- `agent_chat/internal/bridge/agent_service.go` 已支持 gateway API 调用与 event stream 转发。

### 2.2 当前缺口

- `agent_chat` **没有**“发现失败后自动拉起 `agent_gateway`”机制（当前 `loadGatewayProxy()` 只发现，不启动）。
- 启动后网关可用性缺少统一状态机（连接中/已连接/失败原因）。
- `agent_gateway` runtime 中 ACP connector 已创建，但 service 仍以 `MockGatewayService` 为主，真实 prompt/cancel 路径未完全替换。

## 3. 目标架构（启动后流程）

1. `agent_chat` 启动。
2. bridge 执行 `ensureGatewayRunning()`：
   - 读取 endpoint + token；
   - 调用 `/health`；
   - 不可用则启动 `agent-gateway` 进程；
   - 等待 endpoint 文件与健康检查通过。
3. bridge 建立网关代理并启动 SSE 订阅。
4. `NewSession/Prompt/Cancel/List*` 统一走 gateway。
5. 当 gateway 异常退出时，bridge 触发重连与可控降级。

## 4. 分阶段实施计划

### P1：启动唤醒能力（必须先做）

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
   - 成功后再 `streamGatewayEvents`
4. 失败处理：
   - 返回结构化错误码 `gateway_bootstrap_failed`
   - 记录 stderr 日志文件路径（若有）

验收标准：

- 清空 endpoint 文件后启动 `agent_chat`，可自动拉起 gateway 并通过 `/health`。
- gateway 已运行时不重复拉起。
- 启动失败时，前端能收到明确错误而非静默 fallback。

### P2：连接状态与重连治理

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
2. SSE 断线策略：
   - 指数退避（已有）保留；
   - 连续失败阈值后切换为 `gateway_failed` 并停止无穷重试。
3. 将 `lastEventID` 持久化到内存状态边界，保证重连后可补拉。

验收标准：

- 运行中 kill 掉 gateway，客户端进入重连状态并可在 gateway 恢复后自动恢复事件流。
- 鉴权失败（401/403）不做无意义重试，直接给出 `gateway_auth_failed`。

### P3：网关业务接口收敛（从 mock 到真实）

**目标**：`agent_chat` 的核心操作完全依赖 gateway 真实能力。

改动范围：

- `agent_gateway/internal/runtime/server.go`
- `agent_gateway/internal/service/service.go`（替换 mock prompt 主链路）
- `agent_gateway/internal/connectors/acp/*`
- `agent_gateway/internal/api/*`

任务项：

1. 在 gateway runtime 中引入“真实 service”装配：
   - service 调用 connector 处理 `Prompt/Cancel/Approval`；
   - store 仅负责状态持久化与查询。
2. `/v1/sessions/{id}/prompt` 返回值与事件流语义对齐：
   - HTTP 返回 run 接收确认；
   - token-by-token 内容走 SSE。
3. 审批链路闭环：
   - connector -> approval broker -> `/v1/approvals/{id}/decision` -> connector 回写。

验收标准：

- `agent_chat` 发起 prompt 后，可在 UI 收到真实 agent 增量事件，不再依赖 mock assistant 文案。
- cancel 与 approval 在 UI、gateway、connector 三侧状态一致。

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
   - 常见错误与排查步骤。
2. E2E 验证脚本：
   - 启动 chat -> 自动拉起 gateway -> 建会话 -> prompt -> cancel。
3. 兼容性验证：
   - Windows 首验；
   - Linux/macOS 至少保证 discovery 与 health 路径正确。

验收标准：

- 新机器按 README 启动即可自动完成网关拉起与聊天链路。
- `go test ./...`（`agent_chat`、`agent_gateway`）通过。

## 5. 任务拆分建议

1. **Worker A（chat bootstrap）**  
负责 `agent_chat/internal/bridge/gateway_bootstrap*.go` 与启动时序改造。
2. **Worker B（chat stream/state）**  
负责 `AgentService` 状态机、重连、错误码与状态事件。
3. **Worker C（gateway service）**  
负责 gateway service 从 mock 到 connector 实链路。
4. **Worker D（tests/e2e/docs）**  
负责集成测试与文档补齐。

## 6. 风险与缓解

- 风险：启动命令路径不稳定。  
  缓解：优先 `ICOO_GATEWAY_BIN`，并把最终命令写入诊断日志。

- 风险：重复拉起多个 gateway 进程。  
  缓解：启动前先 health check；读取 endpoint PID 并做进程存活判断。

- 风险：鉴权 token 与 endpoint 不一致。  
  缓解：发现后必须同时读取 endpoint + token；401/403 触发重新发现。

- 风险：开发 fallback 掩盖真实问题。  
  缓解：生产模式禁用 fallback；开发模式 fallback 默认告警并可配置关闭。

## 7. 里程碑与交付件

1. **M1（P1 完成）**：自动唤醒可用，启动不再依赖手工先启 gateway。  
交付：bootstrap 代码 + 单元测试。

2. **M2（P2 完成）**：连接状态、重连和错误行为稳定。  
交付：状态事件与断线恢复测试。

3. **M3（P3 完成）**：业务主链路切到真实 gateway connector。  
交付：prompt/cancel/approval 端到端通路。

4. **M4（P4 完成）**：文档与回归验证完成，可进入发布候选。  
交付：README、E2E 脚本、回归报告。
