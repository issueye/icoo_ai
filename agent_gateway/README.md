# agent_gateway

Local AI Agent gateway for `icoo-ai`.

This service has been moved to the new breaking design:

- Go + Gin HTTP API.
- GORM + SQLite through `github.com/glebarez/sqlite` with no cgo sqlite driver.
- MVC controllers under `internal/controllers`.
- Injected container under `internal/bootstrap`.
- Unified CRUD for Agents, Agent Roles, MCP Servers, Schedule Tasks, and Skills.
- ACP runtime for external Agent processes, sessions, prompts, permission approvals, and gateway extension methods.
- WebSocket event stream at `/v1/events`.

## Run

```powershell
go run ./cmd/agent-gateway
```

The service loads `config/agent-gateway.toml`, ensures `auth_token` exists, binds to loopback, and writes runtime endpoint files under `data_dir`.

One-shot startup check:

```powershell
go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once
```

## API

All `/v1/*` endpoints require:

```text
Authorization: Bearer <auth_token>
```

Core REST resources:

- `/v1/agents`
- `/v1/agent-roles`
- `/v1/mcp-servers`
- `/v1/schedule-tasks`
- `/v1/skills`
- `/v1/approvals`

Agent runtime:

- `POST /v1/agents/:id/start`
- `POST /v1/agents/:id/stop`
- `POST /v1/agents/sync-runtime`
- `GET /v1/agents/runtime-status`
- `POST /v1/agents/:id/sessions`
- `POST /v1/agents/:id/sessions/:sessionId/prompts`
- `POST /v1/agents/:id/sessions/:sessionId/cancel`
- `DELETE /v1/agents/:id/sessions/:sessionId`

Events:

- `GET /v1/events`
- Optional query filters: `agentId`, `sessionId`, `type`, `lastEventId`.

ACP gateway extension methods use `_icoo.gateway/*`, including `agent.*`, `agent-role.*`, `mcp.*`, `schedule.*`, and `skill.*`.

## Migration

Old management settings exports can be imported into the new SQLite schema:

```powershell
go run ./cmd/agent-gateway-migrate -input path\to\settings-export.data -data-dir .\.agent_gateway
```

The migration creates a timestamped backup of the input file by default, then imports agents, MCP servers, schedule tasks, and skills.

## Smoke

Fast smoke:

```powershell
.\scripts\smoke-agent-gateway.ps1
```

Full smoke plus all tests:

```powershell
.\scripts\smoke-agent-gateway.ps1 -Full
```

Manual verification:

```powershell
go test ./...
go list -deps ./... | Select-String "mattn/go-sqlite3"
```

The dependency check must print nothing.
