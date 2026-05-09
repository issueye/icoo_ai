param(
    [switch]$Clean,
    [switch]$SkipTests,
    [switch]$SkipChat,
    [switch]$SkipGateway,
    [switch]$SkipServer
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..")
$DistDir = Join-Path $RepoRoot "dist"

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )
    Write-Host "==> $Name"
    & $Action
}

function Build-AgentChat-ToDist {
    $args = @()
    if ($Clean) { $args += "-Clean" }
    if (-not $SkipTests) { $args += "-RunTests" }

    & (Join-Path $RepoRoot "agent_chat\scripts\build.ps1") @args

    $chatBin = Join-Path $RepoRoot "agent_chat\bin\agent_chat.exe"
    if (-not (Test-Path $chatBin)) {
        throw "agent_chat build output not found: $chatBin"
    }
    Copy-Item -LiteralPath $chatBin -Destination (Join-Path $DistDir "agent_chat.exe") -Force
}

function Build-AgentGateway-ToDist {
    Push-Location (Join-Path $RepoRoot "agent_gateway")
    try {
        if (-not $SkipTests) {
            go test ./... -count=1
        }
        $out = Join-Path $DistDir "agent-gateway.exe"
        go build -trimpath -o $out ./cmd/agent-gateway
    }
    finally {
        Pop-Location
    }
}

function Build-AgentServer-ToDist {
    Push-Location (Join-Path $RepoRoot "agent_server")
    try {
        if (-not $SkipTests) {
            go test ./... -count=1
        }
        $out = Join-Path $DistDir "icoo-ai.exe"
        go build -trimpath -o $out ./cmd/icoo-ai

        $example = Join-Path (Get-Location) "configs\config.example.toml"
        if (Test-Path $example) {
            Copy-Item -LiteralPath $example -Destination (Join-Path $DistDir "config.example.toml") -Force
        }
    }
    finally {
        Pop-Location
    }
}

if ($Clean -and (Test-Path $DistDir)) {
    Remove-Item -LiteralPath $DistDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

Set-Location $RepoRoot

if (-not $SkipChat) {
    Invoke-Step "Build agent_chat -> dist/agent_chat.exe" { Build-AgentChat-ToDist }
}
if (-not $SkipGateway) {
    Invoke-Step "Build agent_gateway -> dist/runtime/gateway/agent-gateway.exe" { Build-AgentGateway-ToDist }
}
if (-not $SkipServer) {
    Invoke-Step "Build agent_server -> dist/runtime/agent/icoo-ai.exe" { Build-AgentServer-ToDist }
}

Write-Host "Done. Artifacts in: $DistDir"
