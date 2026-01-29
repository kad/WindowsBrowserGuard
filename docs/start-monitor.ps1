# Registry Extension Monitor - Manual Start Script
# This script starts the monitor manually (useful for testing)

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "❌ This script must be run as Administrator" -ForegroundColor Red
    Write-Host "Attempting to elevate privileges..." -ForegroundColor Yellow
    
    # Re-launch as administrator
    Start-Process powershell.exe -ArgumentList "-ExecutionPolicy Bypass -File `"$PSCommandPath`"" -Verb RunAs
    exit
}

# Configuration
$scriptDir = $PSScriptRoot
$exePath = Join-Path $scriptDir "WindowsBrowserGuard.exe"
$logPath = Join-Path $scriptDir "WindowsBrowserGuard-log.txt"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Windows Browser Guard - Manual Start" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Validate executable exists
if (-not (Test-Path $exePath)) {
    Write-Host "❌ Error: WindowsBrowserGuard.exe not found at: $exePath" -ForegroundColor Red
    Write-Host "Please ensure WindowsBrowserGuard.exe is in the same directory as this script" -ForegroundColor Yellow
    pause
    exit 1
}

# Check if already running
$existingProcess = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
if ($existingProcess) {
    Write-Host "⚠️  Monitor is already running (PID: $($existingProcess.Id))" -ForegroundColor Yellow
    $response = Read-Host "Do you want to stop it and restart? (Y/N)"
    if ($response -eq 'Y' -or $response -eq 'y') {
        Write-Host "Stopping existing process..." -ForegroundColor Cyan
        Stop-Process -Name "WindowsBrowserGuard" -Force
        Start-Sleep -Seconds 2
        Write-Host "✓ Existing process stopped" -ForegroundColor Green
    } else {
        Write-Host "Keeping existing process running" -ForegroundColor Gray
        pause
        exit 0
    }
}

Write-Host "Starting monitor..." -ForegroundColor Cyan
Write-Host "Executable: $exePath" -ForegroundColor Gray
Write-Host "Log file:   $logPath" -ForegroundColor Gray
Write-Host ""

# Ask user how they want to run it
Write-Host "Choose how to run the monitor:" -ForegroundColor Yellow
Write-Host "1. Console window (see output in real-time)" -ForegroundColor White
Write-Host "2. Hidden background process (output to log file)" -ForegroundColor White
Write-Host ""
$choice = Read-Host "Enter choice (1 or 2)"

if ($choice -eq "1") {
    # Run in console window
    Write-Host "Starting in console window..." -ForegroundColor Cyan
    Write-Host "Press Ctrl+C to stop the monitor" -ForegroundColor Yellow
    Write-Host ""
    Start-Sleep -Seconds 2
    & $exePath
} elseif ($choice -eq "2") {
    # Run hidden with log file
    Write-Host "Starting as hidden background process..." -ForegroundColor Cyan
    
    # Create a simple wrapper to redirect output
    Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -Command `"& '$exePath' *>&1 | Tee-Object -FilePath '$logPath'`"" -WindowStyle Hidden
    
    Start-Sleep -Seconds 2
    
    # Verify it started
    $runningProcess = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
    if ($runningProcess) {
        Write-Host "✓ Monitor started successfully (PID: $($runningProcess.Id))" -ForegroundColor Green
        Write-Host ""
        Write-Host "To view logs in real-time:" -ForegroundColor Yellow
        Write-Host "  Get-Content '$logPath' -Tail 50 -Wait" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "To stop the monitor:" -ForegroundColor Yellow
        Write-Host "  Stop-Process -Name WindowsBrowserGuard -Force" -ForegroundColor Cyan
    } else {
        Write-Host "❌ Failed to start monitor. Check the log file for errors." -ForegroundColor Red
        Write-Host "Log file: $logPath" -ForegroundColor Gray
    }
    Write-Host ""
    pause
} else {
    Write-Host "❌ Invalid choice" -ForegroundColor Red
    pause
    exit 1
}
