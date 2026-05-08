# Go AI Agent CLI 工具需求分析与总体设计

## 1. 项目背景

本项目目标是使用 Go 开发一个类似 Claude Code 的命令行 AI Agent 工具。工具面向开发者在本地代码仓库中完成代码理解、编辑、命令执行、测试验证、文档生成、重构建议等任务。

该工具不是单纯的聊天客户端，而是一个具备本地上下文感知、工具调用、任务规划、文件编辑和执行反馈闭环的开发代理。

## 2. 产品目标

### 2.1 核心目标

- 提供 ACP 协议入口，支持开发者通过编辑器、IDE 或其他客户端用自然语言描述任务。
- 能读取和理解当前工作区代码、配置、文档和 Git 状态。
- 能基于模型输出调用本地工具，例如搜索文件、读取文件、修改文件、运行命令、运行测试。
- 能在执行高风险操作前进行权限确认或策略拦截。
- 能保留会话上下文，支持多轮任务推进。
- 以 Go 实现稳定、跨平台、易分发的本地 Agent Server。

### 2.2 非目标

- 初期不实现完整 IDE 图形界面。
- 初期不实现云端多人协作平台。
- 初期不托管用户代码。
- 初期不实现自研大模型，只集成外部 LLM Provider。
- 初期不追求完全自动化的大规模代码改写，优先保证可控性和可审计性。

## 3. 目标用户

- 后端、前端、全栈开发者。
- DevOps / SRE 工程师。
- 技术负责人和代码审查人员。
- 希望在本地仓库中使用 AI 辅助编码的团队。

## 4. 典型使用场景

### 4.1 代码理解

用户输入：

```text
解释一下这个项目的启动流程
```

Agent 行为：

- 扫描项目结构。
- 查找入口文件、配置文件、README、脚本。
- 总结启动链路和关键模块。

### 4.2 Bug 修复

用户输入：

```text
修复登录接口偶发 500 的问题，并跑相关测试
```

Agent 行为：

- 搜索登录接口实现和测试。
- 阅读错误处理、数据库访问、日志相关代码。
- 制定修改计划。
- 修改文件。
- 运行测试。
- 汇报变更和验证结果。

### 4.3 重构

用户输入：

```text
把用户权限校验逻辑抽成独立模块
```

Agent 行为：

- 分析现有重复逻辑。
- 给出重构范围。
- 修改相关代码。
- 保持兼容接口。
- 运行测试并汇报风险。

### 4.4 文档生成

用户输入：

```text
根据当前代码生成 API 文档
```

Agent 行为：

- 解析路由、handler、请求响应结构。
- 生成 Markdown 文档。
- 可选写入 `docs/`。

### 4.5 命令执行辅助

用户输入：

```text
检查为什么测试失败
```

Agent 行为：

- 运行测试命令。
- 分析失败日志。
- 定位相关代码。
- 给出修复建议或直接修复。

## 5. 功能需求

### 5.1 ACP 协议与 CLI 入口

初期暂不考虑 TUI。产品形态以 ACP Agent Server 为主，CLI 只负责启动、配置、诊断和调试。

ACP 按 Agent Client Protocol 设计，使用 JSON-RPC 2.0 消息模型。客户端可以是编辑器插件、IDE、终端适配器或其他支持 ACP 的 Agent Client。Go 实现必须优先使用 `github.com/coder/acp-go-sdk`，避免手写协议类型和底层 JSON-RPC 分发逻辑。

工具应支持以下命令：

```text
icoo-ai
icoo-ai serve
icoo-ai run "修复 lint 问题"
icoo-ai init
icoo-ai config
icoo-ai doctor
```

基础能力：

- `serve` 以 stdio 方式启动 ACP JSON-RPC 服务。
- 支持 ACP 初始化、能力协商、会话创建、会话提示、会话取消、会话更新。
- 支持单次任务执行模式，作为本地调试和自动化脚本入口。
- 支持流式 session update。
- 支持任务取消和会话恢复。
- CLI 不直接承担复杂 TUI 展示，展示职责交给 ACP Client。

ACP 第一阶段最低支持：

- `initialize`
- `session/new`
- `session/prompt`
- `session/cancel`
- `session/update`

后续可扩展：

- `session/load`
- `session/list`
- `terminal/create`
- `terminal/output`
- MCP capability 协商。

ACP SDK 使用要求：

- 使用 `github.com/coder/acp-go-sdk` 中的协议类型和 Agent 连接封装。
- ACP 层只负责协议编解码、能力声明和事件映射。
- 不允许业务逻辑直接依赖 ACP handler，业务逻辑通过协议无关的 Agent 接口调用。
- SDK 升级影响应限制在 `internal/protocol/acp` 包内。

### 5.2 工作区上下文

Agent 应能收集以下上下文：

- 当前目录。
- Git 根目录。
- Git 分支、状态、diff。
- 文件树。
- README、配置文件、依赖声明。
- 用户显式指定的文件。
- 最近读取或修改的文件。

上下文收集应遵守限制：

- 不默认读取大文件、二进制文件、密钥文件。
- 有 token 预算管理。
- 支持 `.icooignore` 排除规则。
- 尊重 `.gitignore`。

### 5.3 文件操作

必须支持：

- 搜索文件。
- 搜索文本。
- 读取文件。
- 创建文件。
- 修改文件。
- 删除文件时需要显式确认。
- 展示 diff。

文件修改策略：

- 优先使用 patch 形式。
- 修改前确认文件是否存在用户未提交改动。
- 不覆盖无关变更。
- 修改后可展示变更摘要。

### 5.4 Shell 命令执行

必须支持：

- 执行只读命令，例如 `ls`、`rg`、`go test`、`npm test`。
- 捕获 stdout、stderr、退出码、耗时。
- 支持超时。
- 支持工作目录设置。
- 支持命令风险分级。

高风险命令包括：

- 删除文件或目录。
- 重置 Git 状态。
- 修改系统配置。
- 安装全局依赖。
- 网络上传本地文件。

高风险命令必须经过策略判断或用户确认。

### 5.5 LLM Provider 集成

首个版本优先支持 OpenAI Responses API。Provider 层仍需保持抽象，避免 Agent Loop 直接依赖 OpenAI SDK 或 Responses API 私有结构。

Provider 能力：

- Responses API。
- Streaming。
- Tool calling / function calling。
- Structured output。
- Reasoning 参数透传。
- 模型列表配置。
- 超时和重试。
- API Key 从环境变量或配置文件读取。

建议预留 Provider 接口，便于扩展：

- OpenAI。
- Anthropic。
- Gemini。
- OpenAI compatible endpoint。

暂不支持：

- 离线本地模型。
- Ollama 等本地模型运行时。

### 5.6 Claude Code 兼容性

工具应兼容 Claude Code 的常用命令习惯和配置体验，降低用户迁移成本。

兼容范围：

- 命令命名和行为尽量贴近 Claude Code 的交互直觉。
- 支持项目级和用户级配置。
- 支持类似权限模式、工作区写入、计划展示、工具调用确认等使用习惯。
- 支持从 Claude Code 风格配置迁移到本项目 TOML 配置。

约束：

- 配置文件主格式仍使用 TOML。
- 不复制 Claude Code 的内部实现或私有格式。
- 与本项目架构冲突时，以协议解耦、可审计和安全策略为准。

### 5.7 Agent Loop 接口

Agent Loop 必须抽象成独立接口，与具体 LLM Provider 解耦。Provider 只负责模型通信，Agent Loop 负责状态机、工具调用、上下文追加、步骤推进、终止判断和事件输出。

核心要求：

- Agent Loop 不直接依赖 OpenAI、Anthropic 等 Provider SDK。
- Agent Loop 通过统一 `llm.Provider` 接口发送请求、接收流式事件和工具调用。
- Agent Loop 通过 `tools.Registry` 查找和执行工具。
- Agent Loop 通过 `hooks.Dispatcher` 暴露生命周期事件。
- Agent Loop 的输入输出应是领域模型，不使用 Provider 私有结构。
- 支持不同 loop 实现，例如 `react_loop`、`plan_act_loop`、`single_shot_loop`。

建议接口：

```go
type Loop interface {
    Run(ctx context.Context, req RunRequest) (<-chan Event, error)
}

type Provider interface {
    Stream(ctx context.Context, req CompletionRequest) (<-chan CompletionEvent, error)
}
```

Loop 事件至少包括：

- `run_started`
- `message_delta`
- `tool_call_started`
- `tool_call_completed`
- `approval_requested`
- `plan_updated`
- `run_completed`
- `run_failed`
- `run_cancelled`

### 5.8 Agent 协议适配接口

Agent 核心能力必须与 ACP 解耦。ACP、WebSocket、HTTP、CLI 调试入口都应通过统一 Agent Runtime 接口交互。

核心要求：

- `internal/agent` 不引用 `github.com/coder/acp-go-sdk`。
- ACP 类型只出现在协议适配层。
- 协议适配层负责把 ACP request 转换为领域请求，把 Agent event 转换为 ACP session update。
- 后续新增 WebSocket 协议时，不修改 Agent Loop、Tool、Skill、Hook 核心实现。
- 协议适配层需要处理认证、会话绑定、取消信号和流式事件桥接。

建议接口：

```go
type Runtime interface {
    NewSession(ctx context.Context, req NewSessionRequest) (Session, error)
    Prompt(ctx context.Context, req PromptRequest) (<-chan Event, error)
    Cancel(ctx context.Context, sessionID string) error
    LoadSession(ctx context.Context, sessionID string) (Session, error)
}
```

协议适配包建议：

```text
internal/protocol/
  runtime.go

internal/protocol/acp/
  server.go
  mapper.go
  capabilities.go

internal/protocol/websocket/
  server.go
  mapper.go
```

### 5.9 工具调用系统

Agent 应内置工具：

- `list_files`
- `search_files`
- `read_file`
- `write_file`
- `apply_patch`
- `run_shell`
- `git_status`
- `git_diff`
- `ask_user`
- `web_search`
- `web_fetch`

工具系统要求：

- 每个工具有 schema。
- 每次调用有日志。
- 每次调用有权限判断。
- 工具结果可被模型继续消费。
- 工具错误应结构化返回。

网络访问工具要求：

- `web_search` 用于按查询词检索网页结果，返回标题、摘要、URL、时间等结构化结果。
- `web_search` 首期默认使用 DuckDuckGo 作为搜索后端。
- `web_fetch` 用于抓取指定 URL 内容，返回正文摘要、标题、状态码、content type、最终 URL。
- 默认限制抓取大小、超时、重定向次数和可访问协议。
- 禁止访问本地网段、云元数据地址和 `file://` 等非预期协议，除非用户显式授权。
- 网络工具结果必须标记来源 URL 和抓取时间。
- 高成本或可能泄露本地信息的请求需要经过权限策略。

### 5.10 Hooks 系统

Hooks 用于在 Agent 运行生命周期中插入可配置逻辑。它不应破坏 Agent Loop 与 LLM Provider 的解耦关系，也不应直接修改模型原始消息。

Hooks 触发点：

- `BeforeRun`
- `AfterRun`
- `BeforeModelRequest`
- `AfterModelResponse`
- `BeforeToolCall`
- `AfterToolCall`
- `BeforeFileWrite`
- `AfterFileWrite`
- `BeforeShellCommand`
- `AfterShellCommand`
- `OnApprovalRequested`
- `OnError`

Hooks 能力：

- 记录审计日志。
- 注入项目约束或上下文。
- 对命令和路径做二次安全检查。
- 格式化或压缩工具结果。
- 在文件写入前运行自定义校验。
- 在任务结束后自动运行测试或格式化。

Hooks 配置示例：

```yaml
hooks:
  before_shell_command:
    - name: block-dangerous-rm
      type: builtin
  after_file_write:
    - name: gofmt
      type: command
      command: gofmt -w {{files}}
```

Hook 结果类型：

- `continue`：继续执行。
- `modify`：修改上下文或参数后继续。
- `block`：阻止本次行为并返回原因。
- `request_approval`：要求用户确认。

### 5.11 Skills 系统

Skills 参考 Codex 的 skills 机制设计。Skill 是一个自包含目录，通过必需的 `SKILL.md` 提供元数据和主指令，通过可选资源目录提供脚本、参考资料和资产。它用于扩展项目知识、工具说明、工作流、提示词片段和专用 hooks。

Skill 来源：

- 内置 skills。
- 用户目录 `~/.icoo-ai/skills/`。
- 项目目录 `.icoo-ai/skills/`。
- 后续可支持 Git URL 或插件市场。

Skill 目录结构：

```text
my-skill/
  SKILL.md
  agents/
    openai.yaml
  scripts/
  references/
  assets/
```

`SKILL.md` 必须包含 YAML frontmatter 和 Markdown 指令正文：

```markdown
---
name: go-code-review
description: Go 代码审查与测试建议
---

# Go Code Review

当用户要求审查 Go 代码、分析测试失败或改进 Go 项目质量时使用。
```

Skills 能力要求：

- 支持按名称显式启用。
- 通过 `SKILL.md` frontmatter 中的 `name` 和 `description` 进行发现与触发判断。
- 采用渐进式加载：先加载元数据，触发后加载 `SKILL.md` 正文，需要时再读取 `references/`、执行 `scripts/` 或使用 `assets/`。
- 不要求每个 skill 提供额外 README，避免上下文污染。
- 支持优先级和冲突检测。
- Skill 注入内容必须可审计。
- Skill 可以注册 hooks，但不应绕过权限系统。
- Skill 可以扩展工具，但工具仍需经过统一工具注册和权限策略。

### 5.12 任务规划

Agent 应支持简单计划能力：

- 对复杂任务生成步骤。
- 标记步骤状态。
- 根据工具结果调整计划。
- 在关键变更前向用户说明意图。

计划不是强制用户确认的阻塞流程，除非涉及高风险操作。

### 5.13 会话管理

会话应包括：

- 用户输入。
- 模型回复。
- 工具调用记录。
- 读取文件摘要。
- 修改文件列表。
- 任务计划状态。

建议存储位置：

```text
~/.icoo-ai/sessions/
```

支持：

- 列出历史会话。
- 恢复会话。
- 删除会话。
- 导出会话 Markdown。

### 5.14 配置管理

配置来源优先级：

1. 命令行参数。
2. 环境变量。
3. 项目配置 `.icoo-ai.toml`。
4. 用户配置 `~/.icoo-ai/config.toml`。
5. 默认值。

配置项示例：

```toml
model = "gpt-4.1"
provider = "openai"
base_url = "https://api.openai.com/v1"
api = "responses"
approval_mode = "workspace-write"
max_context_tokens = 120000
shell_timeout_seconds = 120
respect_gitignore = true
agent_loop = "react"
claude_code_compat = true

[web_search]
provider = "duckduckgo"

[skills]
enabled = ["go-code-review"]

[hooks]
enabled = true
```

### 5.15 MCP 支持

需要支持 MCP，用于接入外部工具、资源和提示能力。MCP 能力必须通过统一工具注册、权限策略和审计日志，不允许绕过 Agent 的安全边界。

MCP 支持范围：

- MCP client：连接外部 MCP server。
- 工具发现和 schema 映射。
- 工具调用结果转换为 Agent ToolResult。
- MCP resources 读取。
- MCP prompts 作为可选上下文或 skill 辅助材料。

约束：

- MCP 工具调用必须经过权限检查。
- MCP server 配置使用 TOML。
- MCP 调用必须进入团队级审计日志。
- MCP 与内置 tools 使用同一个工具命名和冲突处理策略。

### 5.16 团队级审计日志

需要支持团队级审计日志，用于记录 Agent 在开发工作区内的关键行为，便于安全审查、问题追溯和合规管理。

审计日志至少记录：

- 会话创建、结束和操作者标识。
- 用户 prompt 摘要。
- 模型 Provider、模型名和调用元信息。
- 工具调用名称、参数摘要、结果状态、耗时。
- 文件创建、修改、删除和 diff 摘要。
- Shell 命令、退出码、工作目录、风险等级。
- 网络访问 URL、搜索关键词、来源和时间。
- MCP server、tool/resource/prompt 调用记录。
- 权限确认、拒绝和策略拦截。
- Skill 和 hook 的启用、注入、阻断记录。

审计日志要求：

- 默认写入本地结构化日志。
- 支持 JSON Lines 格式，便于后续接入团队平台。
- 对 API Key、token、密钥文件内容做脱敏。
- 支持按项目、会话、用户维度过滤。
- 支持后续扩展到远端审计 sink。

建议存储位置：

```text
~/.icoo-ai/audit/
```

### 5.17 权限与安全

权限模式：

- `readonly`：只能读取文件和执行只读命令。
- `suggest`：可生成 patch，但修改前需要确认。
- `workspace-write`：默认模式，可修改工作区文件，高风险操作确认。
- `full-auto`：自动执行，但仍拦截危险命令。

安全要求：

- 默认禁止读取常见密钥文件，例如 `.env`、`id_rsa`、`*.pem`。
- 命令执行前做风险分析。
- 对 shell 命令参数做展示和审计。
- 对文件写入限制在工作区内，除非用户授权。
- 日志中避免输出完整密钥。

## 6. 非功能需求

### 6.1 性能

- CLI 启动时间目标小于 300ms。
- 文件搜索优先调用 `rg`，不可用时降级为 Go 内置遍历。
- 大文件读取应支持截断和分页。
- 流式输出首 token 延迟由模型决定，客户端应即时渲染。

### 6.2 可用性

- 错误信息必须明确说明失败原因和下一步建议。
- 对用户可见的变更应提供文件路径和摘要。
- 支持 Ctrl+C 中断。
- 支持 Windows、macOS、Linux。

### 6.3 可测试性

- Agent 核心逻辑与 CLI 展示分离。
- Tool 调用可 mock。
- LLM Provider 可 mock。
- Shell runner 可注入。
- 文件系统使用接口封装，便于测试。

### 6.4 可扩展性

- Provider 插件化。
- Tool 注册表插件化。
- Prompt 模板可版本化。
- MCP server 接入插件化。
- 协议适配层可扩展到 WebSocket、HTTP 等入口。

## 7. 总体架构设计

### 7.1 架构分层

```text
Protocol Layer
  - Protocol-neutral runtime interface
  - ACP adapter based on github.com/coder/acp-go-sdk
  - Event mapper
  - Future WebSocket adapter

CLI Layer
  - Cobra command
  - serve
  - run
  - config
  - doctor

Application Layer
  - Session manager
  - Agent loop
  - Planner
  - Approval manager
  - Hook dispatcher
  - Skill manager
  - Audit logger

Domain Layer
  - Message model
  - Tool schema
  - Agent loop events
  - Hook events
  - Skill manifest
  - Audit event
  - Permission policy
  - Workspace model

Infrastructure Layer
  - LLM providers
  - File system
  - Shell runner
  - Git adapter
  - Web search client
  - Web fetch client
  - MCP client
  - Config store
  - Session store
  - Audit store
  - Skill store
```

### 7.2 核心流程

```text
User Input
  -> ACP Client
  -> ACP Adapter
  -> Protocol-neutral Runtime
  -> Session Manager
  -> Skill Resolver
  -> Context Builder
  -> Agent Loop
  -> Hook Dispatcher
  -> LLM Provider Interface
  -> Tool Call?
      -> Hook Dispatcher
      -> Permission Policy
      -> Tool Executor
      -> Tool Result
      -> Agent Loop
  -> Loop Event Stream
  -> Runtime Event Stream
  -> ACP session/update
  -> Session Persist
```

### 7.3 推荐 Go 包结构

```text
cmd/icoo-ai/
  main.go

internal/cli/
  root.go
  serve.go
  run.go
  init.go
  config.go
  doctor.go

internal/protocol/
  runtime.go
  types.go

internal/protocol/acp/
  server.go
  mapper.go
  capabilities.go
  stdio.go

internal/protocol/websocket/
  server.go
  mapper.go

internal/agent/
  loop.go
  react_loop.go
  events.go
  state.go
  planner.go
  prompts.go

internal/llm/
  provider.go
  openai_responses.go
  types.go

internal/tools/
  registry.go
  file.go
  shell.go
  git.go
  user.go
  web_search.go
  web_fetch.go
  mcp.go

internal/mcp/
  client.go
  config.go
  mapper.go

internal/audit/
  logger.go
  event.go
  redactor.go
  store.go

internal/hooks/
  dispatcher.go
  hook.go
  builtin.go
  command.go

internal/skills/
  manifest.go
  loader.go
  resolver.go
  registry.go

internal/workspace/
  context.go
  ignore.go
  scanner.go

internal/session/
  store.go
  transcript.go

internal/config/
  config.go
  loader.go

internal/policy/
  approval.go
  command_risk.go
  path_policy.go

internal/ui/
  output.go

pkg/api/
  types.go
```

## 8. 核心数据模型

### 8.1 Message

```go
type Message struct {
    Role      string         `json:"role"`
    Content   string         `json:"content,omitempty"`
    ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}
```

### 8.2 Tool

```go
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
```

### 8.3 Agent Loop

```go
type Loop interface {
    Name() string
    Run(ctx context.Context, req RunRequest) (<-chan Event, error)
}

type RunRequest struct {
    SessionID string
    CWD       string
    Messages  []Message
    Context   WorkspaceContext
    Skills    []Skill
    Options   RunOptions
}

type Event struct {
    Type      string         `json:"type"`
    SessionID string         `json:"session_id"`
    Content   string         `json:"content,omitempty"`
    Data      map[string]any `json:"data,omitempty"`
    Error     string         `json:"error,omitempty"`
}
```

### 8.4 Agent Runtime

```go
type Runtime interface {
    NewSession(ctx context.Context, req NewSessionRequest) (Session, error)
    Prompt(ctx context.Context, req PromptRequest) (<-chan Event, error)
    Cancel(ctx context.Context, sessionID string) error
    LoadSession(ctx context.Context, sessionID string) (Session, error)
}

type PromptRequest struct {
    SessionID string
    Prompt    string
    CWD       string
    Metadata  map[string]any
}
```

ACP、WebSocket、HTTP 或 CLI 调试入口只能依赖 `Runtime`，不能直接调用具体 Agent Loop。

### 8.5 LLM Provider

```go
type Provider interface {
    Name() string
    Stream(ctx context.Context, req CompletionRequest) (<-chan CompletionEvent, error)
}

type CompletionRequest struct {
    Model    string
    Messages []Message
    Tools    []ToolDefinition
    Options  CompletionOptions
}
```

### 8.6 Hook

```go
type Hook interface {
    Name() string
    Match(event HookEvent) bool
    Execute(ctx context.Context, event HookEvent) (HookResult, error)
}

type HookResult struct {
    Action  string         `json:"action"`
    Reason  string         `json:"reason,omitempty"`
    Patches map[string]any `json:"patches,omitempty"`
}
```

### 8.7 Skill

```go
type Skill struct {
    Name        string
    Description string
    Path        string
    Body        string
    Resources   SkillResources
    Metadata    map[string]any
}

type SkillResources struct {
    Scripts    []string
    References []string
    Assets     []string
}
```

Skill 的 `Name` 和 `Description` 来自 `SKILL.md` frontmatter，`Body` 来自 `SKILL.md` Markdown 正文。`scripts/`、`references/`、`assets/` 通过渐进式加载使用。

### 8.8 ToolResult

```go
type ToolResult struct {
    OK       bool           `json:"ok"`
    Content  string         `json:"content"`
    Data     map[string]any `json:"data,omitempty"`
    Error    string         `json:"error,omitempty"`
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

### 8.9 Session

```go
type Session struct {
    ID        string    `json:"id"`
    CWD       string    `json:"cwd"`
    Model     string    `json:"model"`
    Messages  []Message `json:"messages"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

## 9. Agent 行为原则

系统提示词应约束 Agent：

- 先理解任务，再行动。
- 读代码后再修改。
- 不猜测不存在的文件内容。
- 对风险操作请求确认。
- 不覆盖用户未要求的变更。
- 修改后尽量运行相关测试。
- 最终回复必须包含变更摘要、验证结果、剩余风险。

## 10. 里程碑规划

### M1：最小可用 ACP Agent Server

- `icoo-ai serve`
- 基于 `github.com/coder/acp-go-sdk` 实现 ACP stdio 服务。
- `initialize`、`session/new`、`session/prompt`、`session/update`。
- 协议无关 `agent.Runtime` 接口。
- Agent Loop 接口与 LLM Provider 接口解耦。
- OpenAI Responses API provider。
- 流式回复。
- 基础配置加载。
- TOML 配置。
- 默认 `workspace-write` 权限模式。
- 会话保存。

### M2：本地工具调用与 Agent Loop 闭环

- 文件搜索、读取。
- 基于 DuckDuckGo 的 `web_search`。
- `web_fetch` 网络抓取工具。
- Shell 执行。
- Git status / diff。
- Tool calling 闭环。
- Loop 事件流映射到 ACP session update。
- 基础团队审计日志。

### M3：安全文件编辑与 Hooks

- Patch 应用。
- Diff 展示。
- 权限模式。
- 高风险命令拦截。
- Hooks 生命周期事件。
- 内置安全 hooks。
- Claude Code 命令习惯和配置迁移兼容。

### M4：Skills 与项目能力包

- 任务计划。
- 历史会话恢复。
- `.icooignore`。
- Codex-style `SKILL.md` 发现、加载、启用。
- Skill 正文注入。
- `scripts/`、`references/`、`assets/` 渐进式加载。
- MCP client、工具发现和调用。
- `doctor` 诊断命令。

### M5：扩展能力

- 多 Provider。
- 插件化工具。
- 项目级 Prompt 配置。
- ACP 能力扩展。
- WebSocket 协议适配器。
- 远端审计 sink。

## 11. 验收标准

### 11.1 基础验收

- 用户可以在任意 Git 仓库运行 `icoo-ai serve`。
- ACP Client 可以完成初始化、创建会话、发送 prompt、接收流式更新。
- ACP 实现基于 `github.com/coder/acp-go-sdk`，核心 Agent 不直接依赖 ACP SDK 类型。
- 首发 LLM 调用通过 OpenAI Responses API 完成。
- Agent 能搜索并读取相关文件。
- Agent 能在权限允许时调用 DuckDuckGo `web_search` 和 `web_fetch` 并返回来源 URL。
- Agent 能调用测试命令并解释失败。
- 会话记录可以恢复。
- Agent Loop 可以在不修改 loop 代码的情况下切换 LLM Provider。
- 默认权限模式为 `workspace-write`。
- 团队级审计日志能记录关键工具调用和文件修改。

### 11.2 编辑验收

- Agent 能创建或修改文件。
- 修改前后可展示 diff。
- 不会无提示删除文件。
- 不会覆盖用户未要求的改动。
- 修改后能运行相关验证命令。

### 11.3 安全验收

- 默认不读取密钥文件。
- 危险 shell 命令被拦截或请求确认。
- 工作区外写入需要确认。
- 日志不泄露 API Key。
- Hooks 无法绕过权限策略。
- Skills 注册的工具仍经过统一权限检查。
- 网络工具默认禁止访问本地网段、云元数据地址和非 HTTP(S) 协议。
- MCP 工具调用必须经过统一权限策略和审计日志。

## 12. 主要风险

- LLM 工具调用可能产生错误命令，需要严格策略层。
- 上下文过大导致成本和延迟上升，需要预算和摘要机制。
- 文件修改可能与用户并行改动冲突，需要变更检测。
- Windows shell、Unix shell 差异会影响命令执行抽象。
- 不同模型的 tool calling 格式差异需要 Provider 层屏蔽。
- ACP 协议仍在演进，需隔离协议层和 Agent 核心层，降低后续升级成本。
- Skills 和 hooks 提供扩展能力，也会引入供应链和执行安全风险。
- 网络工具会引入隐私、SSRF、提示注入和结果可信度风险，需要默认限制和来源标注。
- DuckDuckGo 非官方抓取方式可能受页面变化、限流和地区差异影响，需要隔离 search provider 接口。
- 团队级审计日志会引入隐私和存储合规要求，需要脱敏和保留策略。

## 13. 待确认问题

- ACP 的目标兼容版本和测试客户端是什么。
- Skills 是否允许执行外部命令型 hooks。
- Claude Code 配置兼容需要覆盖哪些具体字段和命令别名。
- MCP 首期只做 client 还是同时内置 server。
- 团队级审计日志首期是否需要远端上传，还是只保留本地 JSONL。
- DuckDuckGo 使用 HTML 抓取、lite endpoint 还是第三方封装库。
