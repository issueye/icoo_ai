# 多 Agent 开发阶段与执行计划

## 1. 目标

本文档用于把 icoo-ai 的开发工作拆分为适合多 Agent 并行执行的阶段、任务包和交付边界。目标是让多个开发 Agent 可以在不互相覆盖代码的前提下，围绕清晰接口并行推进。

本计划基于 `docs/requirements-analysis.md`，首期核心方向如下：

- Go 实现本地 Agent Server。
- ACP 入口使用 `github.com/coder/acp-go-sdk`。
- Agent Runtime 与 ACP 解耦，后续可扩展 WebSocket、HTTP 等协议。
- LLM 首发支持 OpenAI Responses API。
- 默认权限模式为 `workspace-write`。
- 配置使用 TOML。
- 支持 Codex-style `SKILL.md` skills。
- 支持 MCP client。
- 支持团队级审计日志。
- `web_search` 首期使用 DuckDuckGo。

## 2. 多 Agent 协作原则

### 2.1 所有权边界

每个 Agent 必须只修改自己负责的包和测试文件。共享类型和接口只能由架构 Agent 或接口冻结阶段统一修改。

建议所有权：

```text
cmd/icoo-ai/                  CLI Agent
internal/protocol/            Protocol Agent
internal/agent/               Agent Runtime Agent
internal/llm/                 LLM Agent
internal/tools/               Tools Agent
internal/policy/              Security Agent
internal/hooks/               Hooks Agent
internal/skills/              Skills Agent
internal/mcp/                 MCP Agent
internal/audit/               Audit Agent
internal/config/              Config Agent
internal/session/             Session Agent
internal/workspace/           Workspace Agent
docs/                         Docs Agent
```

### 2.2 并行规则

- 阶段 0 必须先完成，冻结核心接口后再大规模并行。
- 同一阶段内的任务包可以并行执行，除非标记了依赖。
- Agent 不应重命名其他 Agent 正在使用的公共接口。
- 公共接口变更必须先更新 `internal/*/types.go` 和对应文档，再通知其他任务包同步。
- 每个任务包必须包含单元测试或可运行的集成验证。
- 每个任务包完成时必须说明修改文件、测试结果和剩余风险。

### 2.3 合并规则

- 优先合并接口层、配置层、基础测试工具。
- 再合并不依赖外部服务的本地功能。
- 最后合并 OpenAI、DuckDuckGo、MCP 等外部集成。
- 出现冲突时以接口冻结文档和 `docs/requirements-analysis.md` 为准。

## 3. 阶段总览

```text
阶段 0：项目骨架与接口冻结
阶段 1：最小 ACP Agent Server
阶段 2：Agent Loop 与工具闭环
阶段 3：安全写入、Hooks、审计日志
阶段 4：Skills、MCP、网络工具完善
阶段 5：兼容性、集成测试、发布准备
```

## 4. 阶段 0：项目骨架与接口冻结

目标：建立 Go 项目结构、核心接口、配置规范和测试基础。此阶段是后续并行开发的阻塞项。

### P0-A 架构与公共类型

负责范围：

- `go.mod`
- `cmd/icoo-ai/main.go`
- `internal/agent/types.go`
- `internal/protocol/types.go`
- `internal/llm/types.go`
- `internal/tools/types.go`
- `internal/policy/types.go`
- `pkg/api/types.go`

交付内容：

- 初始化 Go module。
- 定义 `agent.Runtime`、`agent.Loop`、`agent.Event`。
- 定义 `llm.Provider`、`CompletionRequest`、`CompletionEvent`。
- 定义 `tools.Tool`、`ToolDefinition`、`ToolResult`。
- 定义权限模式枚举，默认 `workspace-write`。
- 所有接口必须可 mock。

验收：

- `go test ./...` 可运行。
- 不依赖 OpenAI、ACP SDK、MCP SDK 的具体实现。
- 下游 Agent 可基于接口并行开发。

### P0-B 配置系统

负责范围：

- `internal/config/`

交付内容：

- TOML 配置加载。
- 配置优先级：CLI 参数、环境变量、项目 `.icoo-ai.toml`、用户 `~/.icoo-ai/config.toml`、默认值。
- 支持 `provider = "openai"`、`api = "responses"`、`approval_mode = "workspace-write"`。
- 支持 `[web_search] provider = "duckduckgo"`。
- 支持 MCP、skills、hooks、audit 基础配置结构。
- 提供 Claude Code 风格配置迁移入口，但阶段 0 可只保留接口。

验收：

- 覆盖默认配置、项目配置、用户配置、环境变量覆盖的单元测试。
- 配置错误有明确错误信息。

### P0-C 测试与 Mock 基础设施

负责范围：

- `internal/testutil/`

交付内容：

- Mock LLM Provider。
- Mock Tool。
- Mock Runtime event collector。
- 临时 workspace fixture。
- 测试用 fake shell runner。

验收：

- 后续 Agent 可以直接复用测试工具。
- 无外部网络依赖。

## 5. 阶段 1：最小 ACP Agent Server

目标：通过 ACP stdio 接收请求，创建会话，调用 Runtime，返回流式更新。

### P1-A ACP 协议适配

负责范围：

- `internal/protocol/acp/`
- `internal/protocol/runtime.go`

依赖：

- P0-A。

交付内容：

- 使用 `github.com/coder/acp-go-sdk`。
- 实现 stdio server。
- 支持 `initialize`、`session/new`、`session/prompt`、`session/cancel`。
- 将 ACP request 映射为协议无关 Runtime request。
- 将 `agent.Event` 映射为 ACP session update。

验收：

- 单元测试覆盖 mapper。
- 使用 fake Runtime 验证 ACP handler 不依赖具体 Agent Loop。

### P1-B CLI 启动入口

负责范围：

- `cmd/icoo-ai/`
- `internal/cli/`

依赖：

- P0-B。
- P1-A 的 server 接口。

交付内容：

- `icoo-ai serve` 启动 ACP stdio server。
- `icoo-ai run "prompt"` 调用本地 Runtime 调试入口。
- `icoo-ai config` 展示有效配置。
- `icoo-ai doctor` 检查配置、OpenAI Key、Git、rg。

验收：

- CLI 参数覆盖配置。
- `serve` 不输出破坏 stdio 协议的普通日志。
- `doctor` 不泄露 API Key。

### P1-C Session 存储

负责范围：

- `internal/session/`

依赖：

- P0-A。

交付内容：

- 创建、读取、更新、列出会话。
- 默认存储在 `~/.icoo-ai/sessions/`。
- JSON Lines 或 JSON 文件格式。
- 会话记录包含 messages、tool calls、events 摘要。

验收：

- 并发写入有基本保护。
- 文件损坏时返回可理解错误。

## 6. 阶段 2：Agent Loop 与工具闭环

目标：实现从 prompt 到模型、工具调用、工具结果回传、最终响应的闭环。

### P2-A Agent Runtime 与 ReAct Loop

负责范围：

- `internal/agent/`

依赖：

- P0-A。
- P1-C。

交付内容：

- 实现 `agent.Runtime`。
- 实现首个 `react_loop`。
- 支持事件流：`run_started`、`message_delta`、`tool_call_started`、`tool_call_completed`、`run_completed`、`run_failed`、`run_cancelled`。
- 支持 context cancellation。
- 支持工具调用最大轮数限制。

验收：

- 使用 Mock LLM 和 Mock Tool 验证 loop 闭环。
- 工具失败时 Agent 可以继续或终止，并输出结构化错误。

### P2-B OpenAI Responses Provider

负责范围：

- `internal/llm/openai_responses.go`

依赖：

- P0-A。

交付内容：

- 支持 OpenAI Responses API。
- 支持 streaming。
- 支持 tool calling。
- 支持 structured output 和 reasoning 参数透传。
- 屏蔽 OpenAI 私有事件结构，向上输出统一 `llm.CompletionEvent`。

验收：

- 使用 httptest mock OpenAI API。
- 覆盖流式文本、工具调用、错误响应、超时。
- 不在日志中输出 API Key。

### P2-C Workspace 与文件工具

负责范围：

- `internal/workspace/`
- `internal/tools/file.go`

依赖：

- P0-A。

交付内容：

- 工作区识别和路径规范化。
- 尊重 `.gitignore` 和 `.icooignore`。
- `list_files`、`search_files`、`read_file`、`write_file`、`apply_patch`。
- 大文件截断。
- 二进制文件跳过。

验收：

- 路径穿越被拦截。
- 不默认读取密钥文件。
- patch 应用失败时不产生半写入。

### P2-D Shell 与 Git 工具

负责范围：

- `internal/tools/shell.go`
- `internal/tools/git.go`

依赖：

- P0-A。
- P3-A 可并行开发，先使用简化风险策略。

交付内容：

- `run_shell`。
- `git_status`。
- `git_diff`。
- 命令超时、stdout/stderr、退出码、工作目录。

验收：

- Windows PowerShell 和 Unix shell 抽象可测试。
- 高风险命令返回待确认状态。

## 7. 阶段 3：安全写入、Hooks、审计日志

目标：完善权限策略、生命周期扩展点和团队级审计。

### P3-A 权限策略

负责范围：

- `internal/policy/`

依赖：

- P0-A。

交付内容：

- `readonly`、`suggest`、`workspace-write`、`full-auto`。
- 默认 `workspace-write`。
- 命令风险识别。
- 路径写入策略。
- 网络访问策略。
- MCP 调用策略。

验收：

- 覆盖危险 shell、工作区外写入、本地网段访问、密钥文件读取。

### P3-B Hooks 系统

负责范围：

- `internal/hooks/`

依赖：

- P0-A。
- P3-A。

交付内容：

- Hook dispatcher。
- 生命周期事件：BeforeRun、AfterRun、BeforeToolCall、AfterToolCall、BeforeFileWrite、AfterFileWrite、BeforeShellCommand、AfterShellCommand、OnError。
- Hook 返回 `continue`、`modify`、`block`、`request_approval`。
- 内置安全 hooks。

验收：

- Hook 无法绕过权限策略。
- Hook 错误不会导致未审计的写入或命令执行。

### P3-C 团队级审计日志

负责范围：

- `internal/audit/`

依赖：

- P0-B。

交付内容：

- JSON Lines 审计日志。
- 默认存储 `~/.icoo-ai/audit/`。
- 记录会话、工具、文件、shell、网络、MCP、权限、skill、hook 事件。
- 敏感信息脱敏。
- 预留远端 sink 接口。

验收：

- 单元测试覆盖脱敏。
- 审计事件可按 session id 过滤。
- 工具调用失败也会记录。

## 8. 阶段 4：Skills、MCP、网络工具完善

目标：补齐扩展能力和外部信息访问。

### P4-A Codex-style Skills

负责范围：

- `internal/skills/`

依赖：

- P0-B。
- P2-A。

交付内容：

- 发现内置、用户、项目 skills。
- 解析 `SKILL.md` frontmatter 的 `name` 和 `description`。
- 渐进式加载 `SKILL.md` 正文。
- 支持 `scripts/`、`references/`、`assets/` 资源索引。
- Skill 注入内容进入审计日志。

验收：

- 无 `SKILL.md` 的目录不被加载。
- 大 references 不自动读入上下文。
- skill 冲突有明确错误或优先级处理。

### P4-B MCP Client

负责范围：

- `internal/mcp/`
- `internal/tools/mcp.go`

依赖：

- P0-B。
- P3-A。
- P3-C。

交付内容：

- MCP server TOML 配置。
- 连接外部 MCP server。
- 工具发现和 schema 映射。
- resources 和 prompts 读取。
- MCP 调用转换为统一 ToolResult。

验收：

- 使用 fake MCP server 测试。
- MCP 工具调用经过权限策略。
- MCP 调用进入审计日志。

### P4-C 网络工具

负责范围：

- `internal/tools/web_search.go`
- `internal/tools/web_fetch.go`

依赖：

- P3-A。
- P3-C。

交付内容：

- `web_search` 首期使用 DuckDuckGo。
- `web_fetch` 支持 HTTP(S) 抓取、大小限制、超时、重定向限制。
- SSRF 防护：禁止本地网段、云元数据地址、非 HTTP(S) 协议。
- 返回来源 URL、抓取时间、状态码、content type。

验收：

- 使用 httptest 覆盖 fetch。
- DuckDuckGo provider 可 mock。
- 网络访问进入审计日志。

## 9. 阶段 5：兼容性、集成测试、发布准备

目标：把各模块联通，完成真实端到端验证和发布资产。

### P5-A Claude Code 兼容

负责范围：

- `internal/compat/`
- `internal/cli/`
- `internal/config/`

依赖：

- P0-B。
- P1-B。

交付内容：

- 常用命令习惯兼容。
- Claude Code 风格配置迁移到 TOML。
- 权限和工具确认体验尽量贴近 Claude Code。

验收：

- 配置迁移测试。
- 命令别名测试。

### P5-B 端到端测试

负责范围：

- `tests/e2e/`

依赖：

- 阶段 1 至阶段 4。

交付内容：

- ACP fake client 端到端测试。
- OpenAI mock server 端到端测试。
- 文件修改和测试执行场景。
- skill 触发场景。
- MCP fake server 场景。
- 网络工具 mock 场景。

验收：

- `go test ./...` 通过。
- e2e 测试不依赖真实 OpenAI、DuckDuckGo 或外部 MCP 服务。

### P5-C 发布与文档

负责范围：

- `README.md`
- `docs/`
- 构建脚本

依赖：

- 所有核心功能。

交付内容：

- 安装说明。
- 配置说明。
- ACP 客户端接入说明。
- OpenAI Responses 配置说明。
- MCP 配置说明。
- skills 编写说明。
- 审计日志说明。
- 跨平台构建脚本。

验收：

- 新用户可按 README 完成最小启动。
- 发布包不包含密钥或本地会话数据。

## 10. 推荐并行执行编组

### Wave 1

必须先执行：

- P0-A 架构与公共类型。
- P0-B 配置系统。
- P0-C 测试与 Mock 基础设施。

### Wave 2

P0 完成后可并行：

- P1-A ACP 协议适配。
- P1-B CLI 启动入口。
- P1-C Session 存储。
- P2-B OpenAI Responses Provider。
- P2-C Workspace 与文件工具。
- P3-A 权限策略。
- P3-C 团队级审计日志。

### Wave 3

依赖 Wave 2 的接口后可并行：

- P2-A Agent Runtime 与 ReAct Loop。
- P2-D Shell 与 Git 工具。
- P3-B Hooks 系统。
- P4-A Codex-style Skills。
- P4-C 网络工具。

### Wave 4

核心闭环稳定后执行：

- P4-B MCP Client。
- P5-A Claude Code 兼容。
- P5-B 端到端测试。
- P5-C 发布与文档。

## 11. 跨 Agent 集成检查清单

每次合并前检查：

- 是否只修改了自己负责的包。
- 是否新增或更新了测试。
- 是否通过 `go test ./...`。
- 是否影响公共接口。
- 是否需要更新 `docs/requirements-analysis.md` 或本文档。
- 是否记录安全、审计、配置方面的行为。
- 是否避免真实网络和真实 API 依赖进入默认测试。

## 12. 最小可运行版本定义

第一个可运行版本应满足：

- `icoo-ai serve` 可以启动 ACP stdio server。
- ACP Client 可以 initialize、新建 session、发送 prompt。
- Runtime 可以调用 OpenAI Responses mock provider。
- Agent Loop 可以完成一次文本响应。
- Agent Loop 可以完成一次文件读取工具调用。
- 默认配置为 TOML，权限模式为 `workspace-write`。
- 会话和审计日志可以落盘。
- `go test ./...` 通过。

