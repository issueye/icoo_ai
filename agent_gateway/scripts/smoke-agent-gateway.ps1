param(
    [switch]$Full
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repo = Split-Path -Parent $PSScriptRoot
Push-Location $repo
try {
    go test ./internal/smoke

    if ($Full) {
        go test ./...
    }

    $sqliteCgo = go list -deps ./... | Select-String "mattn/go-sqlite3"
    if ($sqliteCgo) {
        throw "cgo sqlite dependency detected: mattn/go-sqlite3"
    }

    go run ./cmd/agent-gateway -host 127.0.0.1 -port 0 -once
}
finally {
    Pop-Location
}
