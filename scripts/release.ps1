param(
  [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $root

if (Test-Path "dist") {
  Remove-Item "dist" -Recurse -Force
}
New-Item -ItemType Directory -Path "dist" | Out-Null

$targets = @(
  @{ GOOS = "linux"; GOARCH = "amd64" },
  @{ GOOS = "linux"; GOARCH = "arm64" },
  @{ GOOS = "darwin"; GOARCH = "amd64" },
  @{ GOOS = "darwin"; GOARCH = "arm64" },
  @{ GOOS = "windows"; GOARCH = "amd64" },
  @{ GOOS = "windows"; GOARCH = "arm64" }
)

foreach ($target in $targets) {
  $ext = if ($target.GOOS -eq "windows") { ".exe" } else { "" }
  $out = "dist/gosearch-$($target.GOOS)-$($target.GOARCH)$ext"
  $env:GOOS = $target.GOOS
  $env:GOARCH = $target.GOARCH
  go build -ldflags "-X main.version=$Version" -o $out .
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Get-ChildItem dist | Where-Object { -not $_.PSIsContainer } | ForEach-Object {
  $hash = Get-FileHash $_.FullName -Algorithm SHA256
  "$($hash.Hash.ToLower())  $($_.Name)"
} | Set-Content dist/SHA256SUMS.txt

Write-Output "Release artifacts generated in dist/ for version: $Version"
