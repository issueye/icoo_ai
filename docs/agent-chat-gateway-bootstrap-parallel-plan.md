# agent_chat 唤醒 agent_gateway 并发协同开发计划

> 日期：2026-05-09  
> 目标：将“`agent_chat` 启动自动唤醒 `agent_gateway` + 网关对接”拆分为可并行执行的开发任务包。

## 1. 范围

本计划覆盖：

1. `agent_chat` 启动时自动发现/拉起 `agent_gateway`。
2. `agent_chat` 与 `agent_gateway` 的 API + SSE 事件流稳定对接。
3. `agent_gateway` 从 mock 主链路收敛到 ACP connector 主链路。

不覆盖：

- 前端视觉改版。
- 远程网关部署。
- 新协议 connector（CLI/Remote）新增。

## 2. 并行协作模型

- 主线程负责：计划维护、接口冻结、冲突仲裁、最终集成。
- 每个 Worker 只改自己拥有的文件集合。
- 公共 DTO/错误码先冻结，再进入并行开发。
- 每个 Wave 结束后统一跑测试再进入下一波。

## 3. Worker 分工与文件所有权

| Worker | 职责 | 可修改文件 | 禁止修改 |
|---|---|---|---|
| A（Chat Bootstrap） | 自动唤醒与进程拉起 | `agent_chat/internal/bridge/gateway_bootstrap.go` `agent_chat/internal/bridge/gateway_bootstrap_test.go` `agent_chat/internal/bridge/agent_service.go`（仅启动时序） | `agent_gateway/internal/**` |
| B（Chat Stream） | SSE 订阅、重连、状态机 | `agent_chat/internal/bridge/agent_service.go`（仅流处理） `agent_chat/internal/bridge/types.go` `agent_chat/internal/bridge/agent_service_test.go` | `agent_gateway/internal/**` |
| C（Gateway Service） | runtime 装配真实 service | `agent_gateway/internal/runtime/server.go` `agent_gateway/internal/service/service.go` `agent_gateway/internal/service/types.go` | `agent_chat/internal/**` |
| D（ACP Connector） | ACP prompt/cancel/approval 透传 | `agent_gateway/internal/connectors/acp/*` `agent_gateway/internal/connector/*` | `agent_chat/internal/**` |
| E（Events/Approval） | 事件总线与审批闭环 | `agent_gateway/internal/events/*` `agent_gateway/internal/api/events.go` `agent_gateway/internal/api/approvals.go` `agent_gateway/internal/service/approval_broker.go` | `agent_chat/internal/**` |
| F（QA/Docs） | E2E、回归、文档 | `agent_chat/README.md` `agent_gateway/README.md` `docs/*` `agent_gateway/internal/*_test.go` `agent_chat/internal/*_test.go` | 业务实现文件（非测试） |

## 4. Wave 拆分（可并发）

### Wave 0：接口冻结（主线程，阻塞项）

冻结内容：

- Gateway 启动错误码：`gateway_bootstrap_failed`、`gateway_auth_failed`、`gateway_unavailable`。
- 连接状态事件：`gateway_connecting`、`gateway_ready`、`gateway_reconnecting`、`gateway_failed`。
- `/v1/sessions/{id}/prompt` 的同步响应与异步 SSE 语义边界。

完成标准：

- 冻结清单写入本文件并确认，不再随意改字段名。

### Wave 1：双线并发（A + B）与（C + D + E）

任务包 A：

- 实现 `ensureGatewayRunning()`：发现 -> 健康检查 -> 拉起 -> 等待就绪。
- 支持 `ICOO_GATEWAY_BIN` 显式路径。

任务包 B：

- 完成 bridge 连接状态机与重连策略。
- 401/403 快速失败，不无限重试。

任务包 C：

- runtime 注入真实 service 装配点（替换 mock 主入口）。

任务包 D：

- 完成 ACP connector 的 prompt/cancel 主路径与错误映射。

任务包 E：

- 事件补拉与审批决策链路稳定化。

Wave 1 门禁：

- `cd agent_chat && go test ./internal/bridge ./internal/gatewayclient`
- `cd agent_gateway && go test ./internal/runtime ./internal/service ./internal/connectors/acp ./internal/events ./internal/api`

### Wave 2：集成并发（A+B）与（C+D+E）

任务包 AB：

- `ServiceStartup` 接入 `ensureGatewayRunning` + `streamGatewayEvents`。
- 对外透出连接状态事件。

任务包 CDE：

- prompt/cancel/approval 走“API -> service -> connector -> event -> store”完整链路。
- 修复跨 session 串线问题。

Wave 2 门禁：

- `cd agent_gateway && go test ./...`
- `cd agent_chat && go test ./...`

### Wave 3：联调与发布前收口（F 主导，A-E 支持）

任务包 F：

- 增加 E2E 用例：启动 chat 自动拉起 gateway、建会话、prompt、cancel、approval。
- 文档补充启动排障与常见错误。

Wave 3 门禁：

- 新机器按 README 可直接启动并跑通最小链路。
- 两端测试均通过，且无手工先启动 gateway 的前置步骤。

## 5. 集成顺序

1. 合并 Wave 0 冻结变更。
2. 并行合并 Wave 1 各 worker 分支（先小后大）。
3. 合并 Wave 2 联调修复。
4. 合并 Wave 3 测试与文档。

## 6. 冲突规约

- `agent_chat/internal/bridge/agent_service.go` 为高冲突文件：A/B 必须按函数分段占位提交，不同 PR 不改同一函数块。
- DTO 字段变更只能由主线程提交。
- 发现跨域改动（越权改文件）直接退回对应 PR。

## 7. 验收标准（最终）

1. `agent_chat` 启动后可自动唤醒 `agent_gateway`。
2. gateway 已存在时不会重复拉起新进程。
3. prompt/cancel/approval 可在 UI、gateway、connector 三侧一致流转。
4. SSE 断线可恢复；鉴权失败可明确报错。
5. `agent_chat` 与 `agent_gateway` 全量测试通过。

