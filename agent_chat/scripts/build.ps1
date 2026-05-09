param(
    [switch]$Clean,
    [switch]$RunTests,
    [switch]$NoColour
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Resolve-Path (Join-Path $ScriptDir "..")

function Require-Command {
    param([string]$Name)

    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' was not found on PATH."
    }
}

Set-Location $ProjectRoot

Require-Command "wails3"
Require-Command "npm"

if ($Clean) {
    foreach ($path in @("frontend\dist", "bin")) {
        $target = Join-Path $ProjectRoot $path
        if (Test-Path $target) {
            Remove-Item -LiteralPath $target -Recurse -Force
        }
    }
}

if ($RunTests) {
    Require-Command "go"
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

$wailsArgs = @("build")
if ($NoColour) {
    $wailsArgs += "-nocolour"
}

wails3 @wailsArgs
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$output = Join-Path $ProjectRoot "bin\agent_chat.exe"
if (Test-Path $output) {
    Write-Host "Build complete: $output"
} else {
    Write-Host "Build complete."
}
