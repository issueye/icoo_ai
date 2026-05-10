# icoo-ai

`icoo-ai` 是一个使用 Go 开发的本地 AI 编程 Agent 工具，目标是提供类似 Claude Code / Codex CLI 的工作流能力，同时保持 Agent、协议、LLM、工具和权限系统之间的解耦。

## 当前状态

已实现能力：

- 基于 `github.com/coder/acp-go-sdk` 的 ACP stdio 服务。
- 协议无关的 `agent.Runtime` 抽象，后续可扩展到 WebSocket 等协议。
- OpenAI Responses API Provider。
- ReAct 风格 Agent Loop，并与 LLM Provider 解耦。
- TOML 配置加载。
- 工作区文件工具，包括文件列表、搜索、读取、写入和补丁应用。
- Shell 和 Git 工具。
- DuckDuckGo `web_search`。
- HTTP(S) `web_fetch`。
- Codex 风格 `SKILL.md` 技能加载。
- 技能管理工具：`skill_list`、`skill_get`、`skill_add`、`skill_delete`、`skill_execute`。
- Subagent 工具：`subagent_run`。
- `skill_execute` 通过 subagent 执行。
- MCP 工具接入，使用 `github.com/mark3labs/mcp-go`。
- Hooks、权限策略、会话存储、JSONL 审计日志。
- 权限申请确认流程：执行一次、总是执行、拒绝。

## 快速开始

1. 设置任意一个 API Key 环境变量：`OPENAI_API_KEY`、`ICOO_AI_OPENAI_API_KEY` 或 `ICOO_AI_API_KEY`。
2. 如需项目级配置，将 `configs/config.example.toml` 复制为 `.icoo-ai.toml`。
3. 运行 `go run ./cmd/icoo-ai doctor` 检查配置是否可用。
4. 运行 `go run ./cmd/icoo-ai run "解释这个工作区"` 执行一次本地任务。
5. 运行 `go run ./cmd/icoo-ai serve` 启动 ACP stdio 服务。

## 配置

项目级配置文件：

```text
.icoo-ai.toml
```

用户级配置文件：

```text
~/.icoo-ai/config.toml
```

示例配置文件位于：

```text
configs/config.example.toml
```

示例：

```toml
model = "gpt-5.4"
provider = "openai"
# 推荐优先通过环境变量配置密钥；确实需要时也可以在配置文件中设置：
# api_key = "sk-..."
base_url = "https://api.openai.com/v1"
api = "responses"
approval_mode = "workspace-write"
agent_loop = "react"
respect_gitignore = true
shell_timeout_seconds = 60
max_context_tokens = 128000
claude_code_compat = true

[web_search]
provider = "duckduckgo"

[network]
# 未配置时默认读取标准环境代理变量。
# http_proxy = "http://127.0.0.1:7890"
# https_proxy = "http://127.0.0.1:7890"
# no_proxy = "localhost,127.0.0.1,.local"

[retry]
max_attempts = 3
initial_delay_millis = 500
max_delay_millis = 5000

[skills]
enabled = []
disabled = []
paths = []

[audit]
enabled = true
format = "jsonl"
max_size_mb = 100
max_backups = 5

[mcp]
enabled = false

[mcp.servers.filesystem]
enabled = false
transport = "stdio"
command = "mcp-server-filesystem"
args = ["."]
```

## API Key

需要设置以下任意一个环境变量：

```text
OPENAI_API_KEY
ICOO_AI_OPENAI_API_KEY
ICOO_AI_API_KEY
```

也可以在配置文件中设置：

```toml
api_key = "sk-..."
```

优先级为：`OPENAI_API_KEY`、`ICOO_AI_OPENAI_API_KEY`、`ICOO_AI_API_KEY`、配置文件 `api_key`。出于安全考虑，命令输出和审计日志不会直接打印密钥。

## 网络代理

网络代理可以通过 `[network]` 配置，也可以使用标准环境变量。显式配置 `[network]` 后，会优先使用配置文件中的代理设置：

```toml
[network]
http_proxy = "http://127.0.0.1:7890"
https_proxy = "http://127.0.0.1:7890"
no_proxy = "localhost,127.0.0.1,.local"
```

也可以只给某类网络调用配置代理：

```toml
[network.llm]
https_proxy = "http://127.0.0.1:7890"

[network.duckduckgo]
http_proxy = "http://127.0.0.1:7891"
```

对应环境变量为：`ICOO_AI_LLM_HTTP_PROXY`、`ICOO_AI_LLM_HTTPS_PROXY`、`ICOO_AI_LLM_NO_PROXY`，以及 `ICOO_AI_DUCKDUCKGO_HTTP_PROXY`、`ICOO_AI_DUCKDUCKGO_HTTPS_PROXY`、`ICOO_AI_DUCKDUCKGO_NO_PROXY`。

## Hooks 运行时

Hooks 可以通过 `app.BuildOptions.Hooks` 注入，并贯穿 Runtime、ReAct Loop、Subagent、Shell 工具和文件工具。

当前支持的运行时事件包括：

- `before_run` / `after_run` / `on_error`
- `before_tool_call` / `after_tool_call`
- `before_file_write` / `after_file_write`
- `before_shell_command` / `after_shell_command`

Hook 可以继续执行、修改事件数据、阻断执行或请求审批。`run_shell` 或 `write_file` 被 hook 阻断时，底层命令或文件写入不会执行。开启审计日志后，hook 调度会以 `hook_use` 事件写入审计日志。

## Skills 使用

Skill 使用 Codex 风格目录结构，每个 skill 目录包含一个 `SKILL.md` 文件。系统会从内置、用户、项目和自定义 skill 路径中发现技能。

可用的 skill 工具包括：

- `skill_list`：列出已发现的技能。
- `skill_get`：加载单个技能及其资源索引。
- `skill_add`：在项目、用户或自定义 scope 中创建可写技能。
- `skill_delete`：删除可写技能，必要时走权限审批。
- `skill_execute`：将任务和技能说明交给 subagent 执行。

CLI 支持显式执行技能：

```powershell
go run ./cmd/icoo-ai run "/skill go-review review internal/agent"
```

## MCP

MCP 配置位于 `[mcp]` 和 `[mcp.servers.<name>]`。启用后，MCP server 暴露的工具会映射为 `mcp__<server>__<tool>` 形式的本地工具名。

MCP tool call 使用有界超时，并会重试临时 `net.Error` 错误。MCP 审计事件会记录 `retry_attempts`，便于诊断外部服务不稳定问题。

## 稳定性说明

- OpenAI Responses 请求会按 `[retry]` 配置重试。
- `web_fetch` 和 DuckDuckGo `web_search` 会重试临时网络错误、`429` 和 `5xx`。
- `web_fetch` 和 `web_search` 不会重试 `400`、`401`、`403` 等配置或认证错误。
- Session 文件只持久化有限的运行摘要，用于追踪 tool、approval 和 run completion 等关键事件，不保存完整的大型工具输出。
- 审计日志使用 Go `slog` JSON 输出，默认写入用户目录下的 `.icoo-ai/audit/audit.jsonl`，超过 `max_size_mb` 后会自动轮转。

## 常用命令

```powershell
go run ./cmd/icoo-ai config
go run ./cmd/icoo-ai doctor
go run ./cmd/icoo-ai version
go run ./cmd/icoo-ai run "解释这个工作区"
go run ./cmd/icoo-ai serve
go run ./cmd/acp-real-client -llm-info ../docs/llm_info.txt -workspace ..
go run ./cmd/icoo-ai migrate-claude-config ./claude.json ./.icoo-ai.toml
```

命令说明：

- `config`：打印当前关键配置。
- `doctor`：检查配置、Provider、API、权限模式和密钥环境变量。
- `version`：打印版本、提交号、构建时间、Go 版本和平台信息。
- `run`：通过本地 Runtime 执行一次 Prompt。
- `serve`：启动 ACP stdio 服务。
- `acp-real-client`：按 `docs/llm_info.txt` 中的 `api_key/base_url/model` 拉起真实 ACP 客户端联调（覆盖 `initialize/new/list/resume/setMode/setConfig/prompt/close`）。
- `migrate-claude-config`：将常见 Claude Code JSON 配置字段迁移为 icoo-ai TOML 配置。

## 构建

Windows PowerShell：

```powershell
.\scripts\build.ps1
.\scripts\build.ps1 -All -Clean
```

Unix shell：

```sh
./scripts/build.sh
./scripts/build.sh --all --clean
```

构建产物会写入 `dist/`。每次构建也会复制：

```text
configs/config.example.toml -> dist/config.example.toml
```

## 开发

运行测试：

```powershell
go test ./...
go test ./... -count=1
```

默认测试使用 mock provider 和 fake client，不依赖真实 OpenAI、DuckDuckGo 或 MCP 服务。

## 文档

- [需求分析](docs/requirements-analysis.md)
- [多 Agent 开发计划](docs/development-plan.md)
