# icoo-ai

`icoo-ai` 是一个使用 Go 开发的本地 AI 编程 Agent 工具，目标是提供类似 Claude Code 的工作流能力，同时保持 Agent、协议、LLM、工具和权限系统之间的解耦。

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
- Hooks、权限策略、会话存储、团队审计 JSONL 日志。
- 权限申请确认流程：执行一次、总是执行、拒绝。

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

审计日志使用 Go `slog` JSON 输出，默认写入用户目录下的 `.icoo-ai/audit/audit.jsonl`。当日志超过 `max_size_mb` 后会自动轮转，最多保留 `max_backups` 个历史文件。

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

## 常用命令

```powershell
go run ./cmd/icoo-ai config
go run ./cmd/icoo-ai doctor
go run ./cmd/icoo-ai version
go run ./cmd/icoo-ai run "解释这个工作区"
go run ./cmd/icoo-ai serve
go run ./cmd/icoo-ai migrate-claude-config ./claude.json ./.icoo-ai.toml
```

命令说明：

- `config`：打印当前关键配置。
- `doctor`：检查配置、Provider、API、权限模式和密钥环境变量。
- `version`：打印版本、提交号、构建时间、Go 版本和平台信息。
- `run`：通过本地 Runtime 执行一次 Prompt。
- `serve`：启动 ACP stdio 服务。
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
