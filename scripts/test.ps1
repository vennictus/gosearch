param(
    [switch]$IncludeFuzz,
    [string]$Version = "dev"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")

Push-Location $repoRoot
try {
    Write-Host "==> Running unit/integration tests (no cache)..." -ForegroundColor Cyan
    go test -count=1 ./...

    Write-Host "==> Running race detector..." -ForegroundColor Cyan
    go test -count=1 -race ./...

    Write-Host "==> Building all packages..." -ForegroundColor Cyan
    go build ./...

    Write-Host "==> Building versioned binary..." -ForegroundColor Cyan
    go build -ldflags "-X main.version=$Version" -o gosearch.exe .

    Write-Host "==> Verifying version output..." -ForegroundColor Cyan
    .\gosearch.exe --version

    if ($IncludeFuzz) {
        Write-Host "==> Running fuzz tests (manual stop recommended)..." -ForegroundColor Cyan
        go test -fuzz=Fuzz -run=^$ ./...
    }

    Write-Host "All validation checks passed." -ForegroundColor Green
}
finally {
    Pop-Location
}
