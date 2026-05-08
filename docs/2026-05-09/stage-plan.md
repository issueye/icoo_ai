# 2026-05-09 阶段计划

## 1. 目标

本计划用于安排 2026-05-09 的开发工作。当前项目已经具备可运行 MVP：ACP 服务、OpenAI Responses、ReAct Loop、基础工具、权限审批、Skills、Subagent、MCP、审计日志、构建脚本均已落地。

明天的重点不是继续堆新功能，而是把已经设计好的扩展点贯穿到运行链路中，并补齐真实使用前必须具备的稳定性能力。

优先目标：

1. Hooks 接入 Runtime、Agent Loop 和工具生命周期。
2. 强化会话记录和事件持久化。
3. 提升 Skills 自动选择和执行体验。
4. 增强 MCP、Web 工具的稳定性和可审计性。
5. 完善发布前文档和验收测试。

## 2. 当前基线

已完成：

- Go CLI：`config`、`doctor`、`version`、`run`、`serve`、`migrate-claude-config`。
- ACP stdio server 基础实现。
- OpenAI Responses Provider：
  - streaming
  - tool calling
  - function call context 回传
  - API key 支持环境变量和 TOML 配置
  - 请求重试
- Agent ReAct Loop：
  - 连续工具调用
  - 权限申请确认
  - 工具事件流
- 工具：
  - file
  - shell
  - git
  - web_search
  - web_fetch
  - MCP tools
  - subagent_run
  - skill_list / skill_get / skill_add / skill_delete / skill_execute
- Skills：
  - Codex-style `SKILL.md`
  - 发现、加载、资源索引
  - skill 执行通过 subagent
- 审计日志：
  - 使用 `slog` JSON 输出
  - 敏感信息脱敏
  - 日志按大小轮转
- 构建：
  - PowerShell 和 shell 构建脚本
  - 构建产物包含 `config.example.toml`
- 默认测试：
  - `go test ./... -count=1` 通过

## 3. 明日总体验收标准

当天结束前至少满足：

- `go test ./... -count=1` 通过。
- Hooks 至少贯穿 tool call、shell command、file write 三条关键链路。
- 新增或更新测试覆盖 Hooks 阻断、修改、审批或错误路径。
- 会话或审计中能够追踪 hooks、tool、approval、subagent、skill 的关键事件。
- README 或 docs 至少补充 Hooks、Skills 或发布相关说明之一。

## 4. 任务编组

### A 组：Hooks Runtime 集成

负责范围：

- `internal/hooks/`
- `internal/agent/`
- `internal/tools/`
- `internal/app/`

目标：

- 将现有 hooks dispatcher 接入运行时。
- 在 Agent Loop 中支持：
  - BeforeRun
  - AfterRun
  - BeforeToolCall
  - AfterToolCall
  - OnError
- 在文件写入链路支持：
  - BeforeFileWrite
  - AfterFileWrite
- 在 shell 执行链路支持：
  - BeforeShellCommand
  - AfterShellCommand

建议实现：

- 在 `agent.RunOptions` 或 `RuntimeOptions` 中注入 hook dispatcher。
- 工具执行前后由 Agent Loop 统一触发通用 tool hooks。
- 文件和 shell 的专用 hooks 可以由对应工具内部触发，或通过工具 metadata 由统一 hook 层分派。
- Hook 返回 `block` 时转换为结构化 `ToolResult`。
- Hook 返回 `request_approval` 时复用现有审批流程。

验收：

- 单元测试覆盖 hook allow、block、error。
- `run_shell` 被 hook 阻断时不执行命令。
- `write_file` 被 hook 阻断时不写入文件。
- hook 事件进入审计日志。

### B 组：Session 和事件持久化增强

负责范围：

- `internal/session/`
- `internal/agent/runtime.go`
- `tests/e2e/`

目标：

- 保存更完整的会话运行记录。
- 改善多轮 prompt 的上下文恢复。
- 记录工具调用结果摘要。

建议实现：

- Session 增加 event summary 或 run history。
- 保存 tool call、tool result、approval decision 的必要元信息。
- 避免把大文件内容、API key、完整 shell 输出无限写入 session。
- 增加 session 文件损坏、并发更新、恢复后的 prompt 测试。

验收：

- 新会话、继续会话、读取历史均有测试。
- 运行一次工具调用后，session 文件中能看到工具名和状态。
- 敏感字段不落盘。

### C 组：Skills 自动选择与命令体验

负责范围：

- `internal/skills/`
- `internal/skilltools/`
- `internal/agent/`
- `cmd/icoo-ai/`

目标：

- 降低手动调用 `skill_execute` 的成本。
- 支持更接近 Claude Code / Codex 的技能使用习惯。

建议实现：

- 增加 skill resolver：
  - 根据显式名称选择。
  - 根据 description 简单匹配。
  - 保留后续向量检索或 LLM 选择接口。
- 支持 prompt 中显式引用技能，例如：
  - `/skill go-review ...`
  - `@skill:go-review ...`
- `skill_execute` 的 subagent prompt 更结构化：
  - skill instructions
  - resources index
  - delegated task
  - expected output
- 增加 skill 执行审计中的 session id 传递。

验收：

- 能通过 CLI 显式执行某个 skill。
- skill 未找到时返回清晰错误。
- 自动选择逻辑有单元测试，不依赖真实 LLM。

### D 组：MCP 与网络工具稳定性

负责范围：

- `internal/mcp/`
- `internal/tools/mcp.go`
- `internal/tools/web_fetch.go`
- `internal/tools/web_search.go`

目标：

- 将重试策略扩展到 web 和 MCP 外部调用。
- 强化 MCP server 生命周期和错误诊断。

建议实现：

- 为 `web_fetch` 增加 retry：
  - 网络错误
  - `429`
  - `5xx`
- 为 DuckDuckGo search client 增加 retry。
- MCP 工具调用增加：
  - timeout
  - retry policy
  - 更清晰的错误码
- 审计中记录 retry attempt 数量。

验收：

- httptest 覆盖 web 429 后成功。
- web 400 不重试。
- MCP fake client 覆盖 timeout / retry / failure。

### E 组：发布和文档完善

负责范围：

- `README.md`
- `docs/`
- `scripts/`

目标：

- 让新用户可以按文档完成最小启动。
- 为后续发布做准备。

建议实现：

- 新增或更新：
  - Hooks 说明
  - Skills 编写指南
  - MCP 配置说明
  - 权限模式说明
  - 审计日志说明
- 构建脚本增加：
  - checksum 生成
  - release 目录结构
  - 可选压缩包
- `doctor` 文档补充 API key、MCP、rg、git 检查说明。

验收：

- README 中包含最小启动路径。
- `dist/` 产物结构说明清晰。
- 发布产物不包含真实密钥、session、audit 日志。

## 5. 推荐执行顺序

上午：

1. A 组先冻结 Hooks 接口接入方案。
2. B 组同步确认 Session 事件结构是否需要新增字段。
3. D 组抽出可复用 retry helper，避免各包重复实现。

中午前检查点：

- Hooks 的核心接口是否需要修改公共类型。
- Retry helper 是否会影响 OpenAI 已有实现。
- Session 结构变更是否会破坏现有测试。

下午：

1. A 组完成 tool / shell / file write hooks 接入。
2. B 组完成 session 持久化增强。
3. C 组实现显式 skill 引用或 skill resolver 的第一版。
4. D 组完成 web retry。
5. E 组同步文档。

收尾：

1. 运行 `go test ./... -count=1`。
2. 运行 `.\scripts\build.ps1 -SkipTests -Clean`。
3. 检查 `dist/config.example.toml`。
4. 汇总剩余风险。

## 6. 风险与注意事项

- Hooks 接入不能绕过权限系统。Hook 只能增强或阻断，不能直接放行高风险操作。
- Session 不应保存完整敏感内容或无限增长的大输出。
- Retry 不能重试认证错误，避免浪费配额或掩盖配置问题。
- Web 和 MCP 的 retry 必须尊重 context cancellation。
- Skill 自动选择必须可解释，避免 Agent 随机注入不相关 skill。
- ACP stdio 模式下不能向 stdout 打普通日志，避免破坏协议。

## 7. 明日建议完成范围

优先完成：

- Hooks 贯穿关键链路。
- web_fetch / web_search retry。
- Session 工具事件摘要。
- README 增加 Hooks 和 Skills 使用说明。

可以顺延：

- MCP resources / prompts 深度支持。
- release zip / checksum。
- 自动 skill 选择的 LLM 版。
- 完整 ACP 真实客户端矩阵测试。
