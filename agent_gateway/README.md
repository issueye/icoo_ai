# agent_gateway

Independent local gateway service for `icoo-ai` agent clients.

Current P1 scope:

- bind to `127.0.0.1` by default
- choose a random port by default
- expose `GET /health`
- generate a local bearer token
- write `endpoint.json` and `token` under the gateway data directory

Run:

```powershell
go run ./cmd/agent-gateway
```

Smoke check with a temporary data directory:

```powershell
go run ./cmd/agent-gateway -data-dir ./.tmp-gateway -once
```
