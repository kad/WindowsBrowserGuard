# Restart Windows Browser Guard
# This script stops and then starts the monitor

param(
    [switch]$Direct  # Use direct start instead of task scheduler
)

$scriptDir = $PSScriptRoot

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Restarting Windows Browser Guard" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Stop
Write-Host "[1/2] Stopping..." -ForegroundColor Cyan
& (Join-Path $scriptDir "stop.ps1")

Write-Host ""
Write-Host "Waiting 2 seconds..." -ForegroundColor Gray
Start-Sleep -Seconds 2

# Start
Write-Host ""
Write-Host "[2/2] Starting..." -ForegroundColor Cyan
if ($Direct) {
    & (Join-Path $scriptDir "start.ps1") -Direct
} else {
    & (Join-Path $scriptDir "start.ps1")
}

Write-Host ""
Write-Host "✓ Restart complete" -ForegroundColor Green
Write-Host ""
