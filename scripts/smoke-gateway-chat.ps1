param(
    [switch]$UseGoRun,
    [int]$TimeoutSeconds = 20
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..")
$GatewayDir = Join-Path $RepoRoot "agent_gateway"
$GatewayExe = Join-Path $GatewayDir "dist\agent-gateway.exe"
$TempDir = Join-Path $RepoRoot ".tmp\smoke-gateway"
$DataDir = Join-Path $TempDir "data"
$LogPath = Join-Path $TempDir "gateway.log"

New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

if (Test-Path $LogPath) {
    Remove-Item -LiteralPath $LogPath -Force
}

function Read-EndpointInfo {
    param([string]$Dir)
    $endpointPath = Join-Path $Dir "endpoint.json"
    if (-not (Test-Path $endpointPath)) {
        return $null
    }
    $endpoint = Get-Content -Path $endpointPath -Raw | ConvertFrom-Json
    if (-not $endpoint.baseUrl) {
        return $null
    }
    $tokenFile = $endpoint.tokenFile
    if (-not $tokenFile) {
        $tokenFile = Join-Path $Dir "token"
    }
    if (-not (Test-Path $tokenFile)) {
        return $null
    }
    $token = (Get-Content -Path $tokenFile -Raw).Trim()
    return @{
        BaseUrl = [string]$endpoint.baseUrl
        Token   = [string]$token
    }
}

function Wait-EndpointInfo {
    param(
        [string]$Dir,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $info = Read-EndpointInfo -Dir $Dir
        if ($null -ne $info) {
            return $info
        }
        Start-Sleep -Milliseconds 200
    }
    throw "gateway endpoint.json/token not ready within ${TimeoutSeconds}s"
}

function Invoke-Gateway {
    param(
        [string]$BaseUrl,
        [string]$Token,
        [string]$Method,
        [string]$Path,
        $Body = $null
    )
    $headers = @{ Authorization = "Bearer $Token" }
    $url = "$BaseUrl$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $url -Headers $headers -TimeoutSec 10
    }
    $json = $Body | ConvertTo-Json -Depth 10
    return Invoke-RestMethod -Method $Method -Uri $url -Headers $headers -ContentType "application/json" -Body $json -TimeoutSec 10
}

$proc = $null
try {
    if ($UseGoRun) {
        $args = @("run", "./cmd/agent-gateway", "-data-dir", $DataDir)
        $proc = Start-Process -FilePath "go" -ArgumentList $args -WorkingDirectory $GatewayDir -PassThru -WindowStyle Hidden -RedirectStandardError $LogPath -RedirectStandardOutput $LogPath
    } else {
        if (-not (Test-Path $GatewayExe)) {
            throw "gateway binary not found: $GatewayExe. Run scripts/build.ps1 -Target gateway first, or pass -UseGoRun."
        }
        $args = @("-data-dir", $DataDir)
        $proc = Start-Process -FilePath $GatewayExe -ArgumentList $args -WorkingDirectory $GatewayDir -PassThru -WindowStyle Hidden -RedirectStandardError $LogPath -RedirectStandardOutput $LogPath
    }

    $info = Wait-EndpointInfo -Dir $DataDir -TimeoutSeconds $TimeoutSeconds
    $baseUrl = $info.BaseUrl.TrimEnd("/")
    $token = $info.Token

    $health = Invoke-RestMethod -Method Get -Uri "$baseUrl/health" -TimeoutSec 10
    if ($health.status -ne "ok") {
        throw "health status is not ok: $($health | ConvertTo-Json -Compress)"
    }

    $session = Invoke-Gateway -BaseUrl $baseUrl -Token $token -Method Post -Path "/v1/sessions" -Body @{
        title   = "smoke session"
        agentId = "icoo-ai-acp"
    }
    if (-not $session.id) {
        throw "session id is empty"
    }

    $prompt = Invoke-Gateway -BaseUrl $baseUrl -Token $token -Method Post -Path "/v1/sessions/$($session.id)/prompt" -Body @{
        content = "smoke prompt"
    }
    if (-not $prompt.run.id) {
        throw "prompt run id is empty"
    }

    $messages = Invoke-Gateway -BaseUrl $baseUrl -Token $token -Method Get -Path "/v1/sessions/$($session.id)/messages"
    if ($messages.Count -lt 1) {
        throw "messages should not be empty"
    }

    $cancel = Invoke-Gateway -BaseUrl $baseUrl -Token $token -Method Post -Path "/v1/sessions/$($session.id)/cancel"
    if (-not $cancel.id) {
        throw "cancel run id is empty"
    }

    Write-Host "Smoke passed"
    Write-Host "  baseUrl: $baseUrl"
    Write-Host "  session: $($session.id)"
    Write-Host "  prompt run: $($prompt.run.id)"
    Write-Host "  cancel run: $($cancel.id)"
}
finally {
    if ($null -ne $proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force
    }
}

