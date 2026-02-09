# Registry Extension Monitor - Task Scheduler Uninstallation Script
# This script removes the scheduled task and optionally stops the running process

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "❌ This script must be run as Administrator" -ForegroundColor Red
    Write-Host "Right-click on PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    pause
    exit 1
}

# Configuration
$taskName = "WindowsBrowserGuard"
$scriptDir = $PSScriptRoot
$wrapperPath = Join-Path $scriptDir "WindowsBrowserGuard-wrapper.ps1"
$configPath = Join-Path $scriptDir "WindowsBrowserGuard-config.json"
$logPath = Join-Path $scriptDir "WindowsBrowserGuard-log.txt"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Windows Browser Guard Uninstaller" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if task exists
$existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if (-not $existingTask) {
    Write-Host "ℹ️  Task '$taskName' not found" -ForegroundColor Yellow
    Write-Host "The scheduled task may have already been removed" -ForegroundColor Gray
} else {
    Write-Host "Found scheduled task: $taskName" -ForegroundColor Gray
    
    # Check if task is running
    $taskInfo = Get-ScheduledTaskInfo -TaskName $taskName -ErrorAction SilentlyContinue
    if ($taskInfo.LastTaskResult -eq 267009) {
        Write-Host "Task is currently running" -ForegroundColor Yellow
    }
    
    # Remove the scheduled task
    Write-Host "Removing scheduled task..." -ForegroundColor Cyan
    try {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction Stop
        Write-Host "✓ Scheduled task removed" -ForegroundColor Green
    } catch {
        Write-Host "❌ Failed to remove scheduled task: $_" -ForegroundColor Red
    }
}

# Check if process is running
$process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
if ($process) {
    Write-Host ""
    Write-Host "⚠️  Monitor process is still running (PID: $($process.Id))" -ForegroundColor Yellow
    $stopProcess = Read-Host "Do you want to stop the running process? (Y/N)"
    if ($stopProcess -eq 'Y' -or $stopProcess -eq 'y') {
        Write-Host "Stopping process..." -ForegroundColor Cyan
        try {
            Stop-Process -Name "WindowsBrowserGuard" -Force -ErrorAction Stop
            Write-Host "✓ Process stopped" -ForegroundColor Green
        } catch {
            Write-Host "❌ Failed to stop process: $_" -ForegroundColor Red
        }
    }
} else {
    Write-Host "ℹ️  Monitor process is not running" -ForegroundColor Gray
}

# Ask about wrapper script
if (Test-Path $wrapperPath) {
    Write-Host ""
    $removeWrapper = Read-Host "Do you want to remove the wrapper script? (Y/N)"
    if ($removeWrapper -eq 'Y' -or $removeWrapper -eq 'y') {
        try {
            Remove-Item -Path $wrapperPath -Force -ErrorAction Stop
            Write-Host "✓ Wrapper script removed" -ForegroundColor Green
        } catch {
            Write-Host "❌ Failed to remove wrapper script: $_" -ForegroundColor Red
        }
    }
}

# Ask about config file
if (Test-Path $configPath) {
    Write-Host ""
    $removeConfig = Read-Host "Do you want to remove the configuration file? (Y/N)"
    if ($removeConfig -eq 'Y' -or $removeConfig -eq 'y') {
        try {
            Remove-Item -Path $configPath -Force -ErrorAction Stop
            Write-Host "✓ Configuration file removed" -ForegroundColor Green
        } catch {
            Write-Host "❌ Failed to remove configuration file: $_" -ForegroundColor Red
        }
    }
}

# Ask about log file
if (Test-Path $logPath) {
    Write-Host ""
    $removeLog = Read-Host "Do you want to remove the log file? (Y/N)"
    if ($removeLog -eq 'Y' -or $removeLog -eq 'y') {
        try {
            Remove-Item -Path $logPath -Force -ErrorAction Stop
            Write-Host "✓ Log file removed" -ForegroundColor Green
        } catch {
            Write-Host "❌ Failed to remove log file: $_" -ForegroundColor Red
        }
    } else {
        Write-Host "ℹ️  Log file preserved at: $logPath" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Uninstallation Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Windows Browser Guard has been removed from startup" -ForegroundColor White
Write-Host "The executable (WindowsBrowserGuard.exe) has been kept and can be deleted manually" -ForegroundColor Gray
Write-Host ""

Write-Host "Press any key to exit..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
