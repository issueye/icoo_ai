# agent_chat × agent_gateway 对接复盘与补齐计划（更新版，2026-05-10）

> 评估时间：2026-05-10  
> 评估仓库：`E:/codes/icoo_ai`  
> 基线要求：配置文件驱动、去 mock（不兼容旧 mock 行为）、生命周期托管、UED 交互一致

## 1. 当前结论

`agent_chat` 与 `agent_gateway` 主链路已完成从“骨架对接”到“真实运行链路”的切换，P0 项基本收敛。当前可以判定：

- 接口链路可用：session / prompt / cancel / runs / approvals / skills / agents 已贯通。
- 事件链路可用：bridge 拉取 SSE 并向前端分发 `agent:event`，前端已消费并更新状态。
- 配置与运维可用：`chat.toml` 承载网关与日志配置，host/port 默认 `127.0.0.1:17889`，日志落盘。
- 生命周期可控：chat 负责拉起、重启、关闭托管网关，并在应用退出时结束托管进程。

## 2. 完成度矩阵（最新）

| 领域 | 完成度 | 现状 |
| --- | --- | --- |
| 网关生命周期托管 | ✅ | 启动确保网关、支持重启/关闭、程序退出释放托管进程 |
| Host/Port 配置 | ✅ | 设置页可配，默认 `127.0.0.1:17889`，启动时通过 CLI 参数注入 |
| slog + 配置化 + 落盘 | ✅ | `chat.toml` 管理 `log_level/log_format/log_file_path`，日志写终端+文件 |
| UED 下拉与二次确认 | ✅ | 设置页使用 UED 下拉，重启/关闭/保存后重启均二次确认 |
| 去 mock（网关运行时） | ✅ | `ACP.Enabled=false` 不再回退 mock，返回结构化配置错误 |
| ACP 启动参数闭环 | ✅ | gateway CLI 支持 `-acp-enabled/-acp-command/-acp-args`，chat 透传 |
| bridge 上下文透传 | ✅ | `workspaceId/cwd/mode/model/agentId` 已映射透传 |
| `/v1/agents` 对接 | ✅ | chat bridge 暴露 `ListAgents`，前端动态加载 agent/mode/model |
| SSE 前端消费 | ✅ | 前端订阅 `agent:event` 并做去重分发 |
| 重启后“假 ready”修复 | ✅ | 增加事件流探活门禁，探活失败保持 `gateway_reconnecting` |
| 冒烟脚本与 no-mock 一致性 | ✅ | `scripts/smoke-gateway-chat.ps1` 已支持 ACP 参数并默认 no-mock 约束 |

## 3. 已完成代码收口（关键点）

1. `agent_gateway/internal/runtime/server.go`  
   - 移除 ACP disabled 场景下的 mock fallback，改为结构化错误返回。

2. `agent_gateway/cmd/agent-gateway/main.go`  
   - 新增 ACP CLI 参数解析：`-acp-enabled`、`-acp-command`、`-acp-args`。

3. `agent_chat/internal/bridge/settings.go` + `gateway_bootstrap.go`  
   - `chat.toml` 新增 ACP/日志字段。  
   - 启动网关时拼装 host/port/acp 参数。

4. `agent_chat/internal/bridge/agent_service.go`  
   - 新增 `ListAgents`。  
   - 启动/重启后增加 SSE 探活判定，防止提前置 `gateway_ready`。

5. 前端 stores 与事件接入  
   - 订阅 `agent:event`，按类型更新 messages/runs/approvals/app。  
   - mode/model 由 `ListAgents` 结果动态驱动。

6. 文档与脚本同步  
   - `agent_chat/README.md` 与 no-mock/ACP 现状对齐。  
   - smoke 脚本支持 ACP 配置参数化。

## 4. 仍需继续优化（非阻塞）

### P1：稳定性增强

- 增加“重启后短窗口稳定性确认”集成测试（不仅探活一次，而是确认事件流持续可连）。  
- 对重连失败路径补充更细粒度错误分级（网络拒连、鉴权失败、配置错误）。

### P1：前端配置源统一

- 当前 workspace 下拉仍以内置列表为主，建议迁移为配置文件或网关返回能力，减少硬编码。

### P2：工程清理

- gateway service 内部类型名仍沿用 `MockGatewayService`，建议后续重命名为中性命名（例如 `GatewayServiceImpl`）以降低认知噪音。
- `ListAgents` 前端当前通过 `Call.ByName` 调用，可在后续统一重新生成 bindings 后切回强类型调用。

## 5. 下一阶段开发计划（建议）

1. 稳定性专项（P1）  
   - 增加网关重启/关闭/异常退出的端到端集成测试。  
   - 为 `gateway_reconnecting -> gateway_failed` 收敛路径增加可观测字段（失败阶段、重试次数、最后错误码）。

2. 配置治理（P1）  
   - 抽离 workspace 选项为配置项或后端接口。  
   - 明确 `chat.toml` 字段契约与迁移策略（向后不兼容说明已生效）。

3. 工程可维护性（P2）  
   - 完成服务命名去 mock 化与绑定生成流程收口。  
   - 补充“联调排障手册”：ACP 启动失败、事件流拒连、权限问题三类。

## 6. 验证基线（本次复核）

2026-05-10 本地复核：

- `cd agent_chat && go test ./...`：通过  
- `cd agent_gateway && go test ./...`：通过  
- `cd agent_chat/frontend && npm run build`：通过

结论：当前对接已从“可演示”进入“可联调可运行”状态，P0 目标可视为完成，后续重点转入稳定性与维护性优化。
