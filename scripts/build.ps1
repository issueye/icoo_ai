param(
    [ValidateSet("all", "chat", "gateway", "server")]
    [string]$Target = "all",
    [switch]$Clean,
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..")

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )
    Write-Host "==> $Name"
    & $Action
}

function Build-AgentChat {
    $args = @()
    if ($Clean) { $args += "-Clean" }
    if (-not $SkipTests) { $args += "-RunTests" }
    & (Join-Path $RepoRoot "agent_chat\scripts\build.ps1") @args
}

function Build-AgentGateway {
    Push-Location (Join-Path $RepoRoot "agent_gateway")
    try {
        if (-not $SkipTests) {
            go test ./... -count=1
        }
        if ($Clean) {
            Remove-Item -LiteralPath "dist" -Recurse -Force -ErrorAction SilentlyContinue
        }
        New-Item -ItemType Directory -Force -Path "dist" | Out-Null
        $out = Join-Path (Get-Location) "dist\agent-gateway.exe"
        go build -trimpath -o $out ./cmd/agent-gateway
        Write-Host "Build complete: $out"
    }
    finally {
        Pop-Location
    }
}

function Build-AgentServer {
    $args = @()
    if ($Clean) { $args += "-Clean" }
    if ($SkipTests) { $args += "-SkipTests" }
    & (Join-Path $RepoRoot "agent_server\scripts\build.ps1") @args
}

Set-Location $RepoRoot

switch ($Target) {
    "all" {
        Invoke-Step "Build agent_chat" { Build-AgentChat }
        Invoke-Step "Build agent_gateway" { Build-AgentGateway }
        Invoke-Step "Build agent_server" { Build-AgentServer }
    }
    "chat"    { Invoke-Step "Build agent_chat" { Build-AgentChat } }
    "gateway" { Invoke-Step "Build agent_gateway" { Build-AgentGateway } }
    "server"  { Invoke-Step "Build agent_server" { Build-AgentServer } }
}
