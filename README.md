# icoo-ai

Go implementation of a local AI coding agent server inspired by Claude Code.

## Current Status

Implemented:

- ACP server scaffold using `github.com/coder/acp-go-sdk`
- Protocol-neutral `agent.Runtime`
- OpenAI Responses API provider
- ReAct-style agent loop
- TOML config loading
- Workspace file tools
- Shell and Git tools
- DuckDuckGo `web_search`
- HTTP(S) `web_fetch`
- Codex-style `SKILL.md` loader
- MCP stdio client and tool mapping scaffold
- Hooks, policy checks, session store, audit JSONL logs

## Configuration

Project config:

```text
.icoo-ai.toml
```

User config:

```text
~/.icoo-ai/config.toml
```

Example:

```toml
model = "gpt-4.1"
provider = "openai"
api = "responses"
approval_mode = "workspace-write"
agent_loop = "react"
respect_gitignore = true

[web_search]
provider = "duckduckgo"

[audit]
enabled = true
format = "jsonl"

[mcp]
enabled = false

[mcp.servers.filesystem]
enabled = true
transport = "stdio"
command = "mcp-server-filesystem"
args = ["."]
```

Set one of:

```text
OPENAI_API_KEY
ICOO_AI_OPENAI_API_KEY
```

## Commands

```powershell
go run ./cmd/icoo-ai config
go run ./cmd/icoo-ai doctor
go run ./cmd/icoo-ai run "explain this workspace"
go run ./cmd/icoo-ai serve
go run ./cmd/icoo-ai migrate-claude-config ./claude.json ./.icoo-ai.toml
```

`serve` starts the ACP stdio server. `run` executes a single prompt through the local runtime.
`migrate-claude-config` converts common Claude Code JSON config fields into icoo-ai TOML.

## Development

Run tests:

```powershell
go test ./...
go test ./... -count=1
```

The default tests use mock providers and fake clients. They do not require real OpenAI, DuckDuckGo, or MCP services.

## Docs

- [Requirements](docs/requirements-analysis.md)
- [Multi-agent Development Plan](docs/development-plan.md)
