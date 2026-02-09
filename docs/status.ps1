# Check Windows Browser Guard Status
# This script shows the current status of the monitor

$scriptDir = $PSScriptRoot
$configPath = Join-Path $scriptDir "WindowsBrowserGuard-config.json"
$logPath = Join-Path $scriptDir "WindowsBrowserGuard-log.txt"
$taskName = "WindowsBrowserGuard"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Windows Browser Guard Status" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Load configuration
$config = $null
if (Test-Path $configPath) {
    $config = Get-Content $configPath | ConvertFrom-Json
    $taskName = $config.TaskName
    Write-Host "Configuration:" -ForegroundColor Yellow
    Write-Host "  Location: $configPath" -ForegroundColor Gray
    if ($config.OTLPEndpoint) {
        Write-Host "  OTLP: $($config.OTLPProtocol)://$($config.OTLPEndpoint)" -ForegroundColor Gray
    } else {
        Write-Host "  OTLP: Not configured" -ForegroundColor Gray
    }
    Write-Host ""
}

# Check process
Write-Host "Process Status:" -ForegroundColor Yellow
$process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
if ($process) {
    Write-Host "  Status: " -NoNewline -ForegroundColor Gray
    Write-Host "RUNNING" -ForegroundColor Green
    Write-Host "  PID: $($process.Id)" -ForegroundColor Gray
    Write-Host "  CPU: $([math]::Round($process.CPU, 2))s" -ForegroundColor Gray
    Write-Host "  Memory: $([math]::Round($process.WorkingSet64 / 1MB, 2)) MB" -ForegroundColor Gray
    Write-Host "  Start Time: $($process.StartTime)" -ForegroundColor Gray
    
    # Calculate uptime
    $uptime = (Get-Date) - $process.StartTime
    Write-Host "  Uptime: $($uptime.Days)d $($uptime.Hours)h $($uptime.Minutes)m" -ForegroundColor Gray
} else {
    Write-Host "  Status: " -NoNewline -ForegroundColor Gray
    Write-Host "NOT RUNNING" -ForegroundColor Red
}
Write-Host ""

# Check scheduled task
Write-Host "Scheduled Task:" -ForegroundColor Yellow
$task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($task) {
    Write-Host "  Name: $taskName" -ForegroundColor Gray
    Write-Host "  State: $($task.State)" -ForegroundColor Gray
    
    $taskInfo = Get-ScheduledTaskInfo -TaskName $taskName
    Write-Host "  Last Run: $($taskInfo.LastRunTime)" -ForegroundColor Gray
    Write-Host "  Last Result: $($taskInfo.LastTaskResult) " -NoNewline -ForegroundColor Gray
    if ($taskInfo.LastTaskResult -eq 0) {
        Write-Host "(Success)" -ForegroundColor Green
    } else {
        Write-Host "(Error)" -ForegroundColor Red
    }
    Write-Host "  Next Run: $($taskInfo.NextRunTime)" -ForegroundColor Gray
} else {
    Write-Host "  Status: Not installed" -ForegroundColor Red
    Write-Host "  Run: .\install-task.ps1" -ForegroundColor Yellow
}
Write-Host ""

# Check log file
Write-Host "Log File:" -ForegroundColor Yellow
if (Test-Path $logPath) {
    $logInfo = Get-Item $logPath
    Write-Host "  Location: $logPath" -ForegroundColor Gray
    Write-Host "  Size: $([math]::Round($logInfo.Length / 1KB, 2)) KB" -ForegroundColor Gray
    Write-Host "  Modified: $($logInfo.LastWriteTime)" -ForegroundColor Gray
    
    # Show last few lines
    Write-Host ""
    Write-Host "Last 5 log entries:" -ForegroundColor Yellow
    Get-Content $logPath -Tail 5 | ForEach-Object {
        Write-Host "  $_" -ForegroundColor Gray
    }
} else {
    Write-Host "  Status: Not found" -ForegroundColor Red
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Quick Actions:" -ForegroundColor Yellow
Write-Host "  Start:   .\start.ps1" -ForegroundColor Gray
Write-Host "  Stop:    .\stop.ps1" -ForegroundColor Gray
Write-Host "  Restart: .\restart.ps1" -ForegroundColor Gray
Write-Host "  Logs:    .\view-logs.ps1" -ForegroundColor Gray
Write-Host ""
