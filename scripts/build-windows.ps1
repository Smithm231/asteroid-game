# Builds Drifthold.exe on Windows. Needs Go 1.25 or newer (https://go.dev/dl/).
#
#   powershell -ExecutionPolicy Bypass -File scripts\build-windows.ps1
param([string]$Version = "1.0.0")
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
New-Item -ItemType Directory -Force dist | Out-Null
go run ./tools/winres -out cmd/drifthold/rsrc_windows_amd64.syso -version $Version
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-H=windowsgui -s -w" -o dist\Drifthold.exe ./cmd/drifthold
Write-Host "built dist\Drifthold.exe"
