# agent_gateway

Independent local gateway service for `icoo-ai` agent clients.

Current scope:

- bind to `127.0.0.1` by default
- choose a random port by default
- expose `GET /health`
- generate a local bearer token
- write `endpoint.json` and `token` under the gateway data directory
- expose `/v1/sessions|runs|approvals|events/stream`
- support ACP connector-backed prompt/cancel path when ACP is enabled
- project async gateway events to local store for history query/recovery

Run:

```powershell
go run ./cmd/agent-gateway
```

CLI flags are intentionally limited to `-host` / `-port` (plus `-once` for one-shot tests).

Gateway configuration template:

- `config/agent-gateway.example.json`
- Includes ACP runtime settings and management settings (`agents`, `mcpServers`, `scheduleTasks`).
- Current binary still accepts only `-host`/`-port` CLI overrides; this template is the standard config baseline for gateway-managed settings.

Smoke check:

```powershell
go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once
```

Build binary:

```powershell
..\scripts\build.ps1 -Target gateway
```

End-to-end smoke:

```powershell
..\scripts\smoke-gateway-chat.ps1
```

Use go-run fallback in smoke:

```powershell
..\scripts\smoke-gateway-chat.ps1 -UseGoRun
```
