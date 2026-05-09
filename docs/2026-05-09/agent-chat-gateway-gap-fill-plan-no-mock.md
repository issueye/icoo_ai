# agent_chat × agent_gateway 对接缺口补齐开发计划（No-Mock 不兼容版）

> 日期：2026-05-09  
> 范围：仅覆盖 `agent_chat` 与 `agent_gateway` 对接缺口补齐。  
> 原则：**删除全部 mock 数据与 mock 业务回退，不兼容旧行为**。

## 1. 目标

1. `agent_chat` 前后端仅允许真实 gateway 链路，不再提供本地 mock 数据、mock 会话、mock 提示词响应。  
2. 补齐已识别接口缺口：`prompt` 字段、`LoadSession` 路由、`skills` 路由、`runs` 模型映射。  
3. 联调失败必须显式报错，不再静默 fallback。

## 2. 不兼容变更声明

以下变更将直接破坏旧调用或旧联调习惯：

1. 移除 `agent_chat/frontend/src/services/mockData.js`。  
2. 移除 `agent_chat/frontend/src/services/agentBridge.js` 中 `callOrMock` 机制。  
3. 移除 `agent_chat/internal/bridge` 的开发态 mock fallback（含会话/消息/审批/运行假数据）。  
4. `PromptRequest` 从 `prompt` 切为 `content`（桥接与前端统一）。  
5. `agent_gateway` 不再产出 mock prompt 响应；未配置可用 connector 时返回结构化错误。

## 3. 分阶段计划

### P0（协议与路由闭环）

1. `Prompt` 字段统一为 `content`。  
2. `agent_gateway` 新增 `GET /v1/sessions/{id}`。  
3. `agent_gateway` 新增 `GET /v1/skills`（可返回空数组）。  
4. `agent_chat` bridge 增加 gateway DTO -> 前端 DTO 映射，处理 `runs` 字段差异。

交付标准：

- `NewSession -> LoadSession -> Prompt -> ListMessages -> Cancel -> ListRuns` 全链路可执行。  
- 不再出现 `LoadSession` / `ListSkills` 的 404。

### P1（彻底移除 mock）

1. 删除前端 mock 数据文件和 mock 状态同步逻辑。  
2. 删除 Go bridge mock 初始数据与 `shouldFallback/devFallback`。  
3. 网关不可用、鉴权失败、路由失败时直接报错并展示失败状态。

交付标准：

- 浏览器或桌面态不再自动伪造会话/消息。  
- 任一桥接调用失败时，调用方拿到真实错误。

### P2（测试与脚本）

1. 更新 `agent_chat` / `agent_gateway` 单测，去除 mock fallback 假设。  
2. 修复 `scripts/smoke-gateway-chat.ps1`（stdout/stderr 重定向冲突）。  
3. 在 README 标注 no-mock 模式与必需前置条件。

交付标准：

- `go test ./...` 在 `agent_chat`、`agent_gateway` 通过。  
- smoke 脚本可跑通或明确失败原因（无 connector 时应返回预期错误）。

## 4. 风险与约束

1. 无 connector 的网关在 no-mock 模式下将无法执行 `Prompt`，这是预期行为。  
2. 前端去掉 mock 后，若未运行 Wails host 或 gateway 未就绪，页面将显示真实错误。  
3. 旧 demo 截图/演示脚本（依赖 mock）需要同步废弃。

## 5. 验收清单

1. 代码中不再存在 `agent_chat` 运行态 mock 会话/消息数据源。  
2. 代码中不再存在桥接静默 fallback 到 mock 的逻辑。  
3. `Prompt` 请求体、路由、返回模型在 chat/gateway 双侧一致。  
4. 关键错误路径（gateway unavailable/auth failed/connector unavailable）可被前端明确识别。  

