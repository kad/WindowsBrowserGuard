# Stop Windows Browser Guard
# This script stops the monitor process and/or task

$scriptDir = $PSScriptRoot
$configPath = Join-Path $scriptDir "WindowsBrowserGuard-config.json"
$taskName = "WindowsBrowserGuard"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Stopping Windows Browser Guard" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Load configuration
if (Test-Path $configPath) {
    $config = Get-Content $configPath | ConvertFrom-Json
    $taskName = $config.TaskName
}

# Check if process is running
$process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue

if ($process) {
    Write-Host "Found running process (PID: $($process.Id))" -ForegroundColor Yellow
    Write-Host "Stopping process..." -ForegroundColor Cyan
    
    Stop-Process -Id $process.Id -Force
    Start-Sleep -Seconds 2
    
    # Verify stopped
    $stillRunning = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
    if ($stillRunning) {
        Write-Host "❌ Failed to stop process" -ForegroundColor Red
        exit 1
    } else {
        Write-Host "✓ Process stopped" -ForegroundColor Green
    }
} else {
    Write-Host "No running process found" -ForegroundColor Gray
}

# Check task scheduler
$task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($task) {
    if ($task.State -eq "Running") {
        Write-Host "Stopping scheduled task..." -ForegroundColor Cyan
        Stop-ScheduledTask -TaskName $taskName | Out-Null
        Start-Sleep -Seconds 1
        Write-Host "✓ Task stopped" -ForegroundColor Green
    } else {
        Write-Host "Task is not running (State: $($task.State))" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "Monitor stopped" -ForegroundColor Green
Write-Host ""
Write-Host "To start again: .\start.ps1" -ForegroundColor Yellow
Write-Host ""
