# Build SWELL Box for Windows (no console + embedded app icon)
# Usage:
#   .\scripts\build.ps1              # current arch (usually amd64)
#   .\scripts\build.ps1 -Arch arm64  # Windows ARM64
param(
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is not installed or not on PATH."
}

$rsrc = Join-Path $env:USERPROFILE "go\bin\rsrc.exe"
if (-not (Test-Path $rsrc)) {
    go install github.com/akavel/rsrc@latest
}
if (Test-Path $rsrc) {
    Remove-Item ".\cmd\swellbox\rsrc.syso" -Force -ErrorAction SilentlyContinue
    $ico = ".\internal\seed\app.ico"
    if (-not (Test-Path $ico)) { $ico = ".\internal\seed\logo.ico" }
    $manifest = ".\cmd\swellbox\app.manifest"
    # Embed icon + DPI-aware manifest (sharp tray menus on scaled displays)
    if (Test-Path $manifest) {
        & $rsrc -manifest $manifest -ico $ico -arch $Arch -o ".\cmd\swellbox\rsrc_windows_$Arch.syso"
    } else {
        & $rsrc -ico $ico -arch $Arch -o ".\cmd\swellbox\rsrc_windows_$Arch.syso"
    }
}

go mod tidy
New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = $Arch
$out = if ($Arch -eq "amd64") { "dist/SWELL-Box.exe" } else { "dist/SWELL-Box-windows-arm64.exe" }
go build -ldflags "-H=windowsgui -s -w" -o $out ./cmd/swellbox
Write-Host "OK -> $out (windows/$Arch)"
