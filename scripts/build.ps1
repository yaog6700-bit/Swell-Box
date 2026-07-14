# Build SWELL Box for Windows (no console + embedded app icon)
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
    & $rsrc -ico $ico -arch amd64 -o ".\cmd\swellbox\rsrc_windows_amd64.syso"
}

go mod tidy
New-Item -ItemType Directory -Force -Path dist | Out-Null
go build -ldflags "-H=windowsgui -s -w" -o dist/swellbox.exe ./cmd/swellbox
Write-Host "OK -> dist/swellbox.exe (GUI, colorful icon)"
