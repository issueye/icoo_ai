param(
    [string]$Version = "",
    [string]$OutDir = "dist",
    [string]$GOOS = "",
    [string]$GOARCH = "",
    [switch]$All,
    [switch]$SkipTests,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

if ($Clean -and (Test-Path $OutDir)) {
    Remove-Item -LiteralPath $OutDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$ExampleConfig = Join-Path $RepoRoot "configs\config.example.toml"
if (Test-Path $ExampleConfig) {
    Copy-Item -LiteralPath $ExampleConfig -Destination (Join-Path $OutDir "config.example.toml") -Force
}

if ([string]::IsNullOrWhiteSpace($Version)) {
    $tag = git describe --tags --always --dirty 2>$null
    if ([string]::IsNullOrWhiteSpace($tag)) {
        $Version = "dev"
    } else {
        $Version = $tag.Trim()
    }
}

$Commit = git rev-parse --short HEAD 2>$null
if ([string]::IsNullOrWhiteSpace($Commit)) {
    $Commit = "unknown"
} else {
    $Commit = $Commit.Trim()
}
$BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$Ldflags = "-s -w -X main.version=$Version -X main.commit=$Commit -X main.date=$BuildDate"

if (-not $SkipTests) {
    go test ./... -count=1
}

$Targets = @()
if ($All) {
    $Targets = @(
        @{ GOOS = "windows"; GOARCH = "amd64" },
        @{ GOOS = "windows"; GOARCH = "arm64" },
        @{ GOOS = "linux"; GOARCH = "amd64" },
        @{ GOOS = "linux"; GOARCH = "arm64" },
        @{ GOOS = "darwin"; GOARCH = "amd64" },
        @{ GOOS = "darwin"; GOARCH = "arm64" }
    )
} else {
    if ([string]::IsNullOrWhiteSpace($GOOS)) {
        $GOOS = go env GOOS
    }
    if ([string]::IsNullOrWhiteSpace($GOARCH)) {
        $GOARCH = go env GOARCH
    }
    $Targets = @(@{ GOOS = $GOOS.Trim(); GOARCH = $GOARCH.Trim() })
}

foreach ($Target in $Targets) {
    $targetOS = $Target.GOOS
    $targetArch = $Target.GOARCH
    $binary = "icoo-ai-$targetOS-$targetArch"
    if ($targetOS -eq "windows") {
        $binary += ".exe"
    }
    $outPath = Join-Path $OutDir $binary
    Write-Host "building $outPath"
    $env:GOOS = $targetOS
    $env:GOARCH = $targetArch
    go build -trimpath -ldflags $Ldflags -o $outPath ./cmd/icoo-ai
}

Write-Host "build artifacts written to $OutDir"
