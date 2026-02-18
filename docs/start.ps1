# Start Windows Browser Guard
# This script starts the monitor either as a task or directly

param(
    [switch]$Direct  # Start directly instead of via task scheduler
)

$scriptDir = $PSScriptRoot
$configPath = Join-Path $scriptDir "WindowsBrowserGuard-config.json"
$taskName = "WindowsBrowserGuard"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Starting Windows Browser Guard" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Load configuration
if (Test-Path $configPath) {
    $config = Get-Content $configPath | ConvertFrom-Json
    $taskName = $config.TaskName
} else {
    Write-Host "⚠️  Configuration not found. Using defaults." -ForegroundColor Yellow
}

if ($Direct) {
    # Start directly without task scheduler
    Write-Host "Starting directly (not using task scheduler)..." -ForegroundColor Cyan
    
    $exePath = Join-Path $scriptDir "WindowsBrowserGuard.exe"
    if (-not (Test-Path $exePath)) {
        Write-Host "❌ Error: WindowsBrowserGuard.exe not found" -ForegroundColor Red
        exit 1
    }
    
    # Check if already running
    $existing = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "⚠️  Process already running (PID: $($existing.Id))" -ForegroundColor Yellow
        $response = Read-Host "Stop and restart? (Y/N)"
        if ($response -eq 'Y' -or $response -eq 'y') {
            Stop-Process -Id $existing.Id -Force
            Start-Sleep -Seconds 2
        } else {
            exit 0
        }
    }
    
    # Build command arguments
    $args = @()
    if ($config.LogPath) {
        $args += "--log-file=$($config.LogPath)"
    }
    if ($config.OTLPEndpoint) {
        $ep = $config.OTLPEndpoint
        # Migrate legacy bare host:port (no scheme) to full URL using saved protocol/insecure fields
        if ($ep -notlike "*://*") {
            $protocol = if ($config.OTLPProtocol) { $config.OTLPProtocol } else { "grpc" }
            $scheme = if ($protocol -eq "http") {
                if ($config.OTLPInsecure) { "http" } else { "https" }
            } else {
                if ($config.OTLPInsecure) { "grpc" } else { "grpcs" }
            }
            $ep = "${scheme}://$ep"
        }
        $args += "--otlp-endpoint=$ep"
    }
    
    Write-Host "Starting process..." -ForegroundColor Cyan
    if ($args.Count -gt 0) {
        Start-Process -FilePath $exePath -ArgumentList $args -WindowStyle Hidden
    } else {
        Start-Process -FilePath $exePath -WindowStyle Hidden
    }
    
    Start-Sleep -Seconds 2
    
    $process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
    if ($process) {
        Write-Host "✓ Started successfully (PID: $($process.Id))" -ForegroundColor Green
    } else {
        Write-Host "❌ Failed to start process" -ForegroundColor Red
        exit 1
    }
} else {
    # Start via task scheduler
    Write-Host "Starting task: $taskName..." -ForegroundColor Cyan
    
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    if (-not $task) {
        Write-Host "❌ Task not found: $taskName" -ForegroundColor Red
        Write-Host "Please run install-task.ps1 first" -ForegroundColor Yellow
        exit 1
    }
    
    # Check current state
    $taskInfo = Get-ScheduledTaskInfo -TaskName $taskName
    Write-Host "Current state: $($task.State)" -ForegroundColor Gray
    
    if ($task.State -eq "Running") {
        Write-Host "⚠️  Task is already running" -ForegroundColor Yellow
        $response = Read-Host "Restart it? (Y/N)"
        if ($response -eq 'Y' -or $response -eq 'y') {
            Stop-ScheduledTask -TaskName $taskName | Out-Null
            Start-Sleep -Seconds 2
        } else {
            exit 0
        }
    }
    
    # Start the task
    Start-ScheduledTask -TaskName $taskName
    Start-Sleep -Seconds 3
    
    # Verify
    $process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
    if ($process) {
        Write-Host "✓ Task started successfully (PID: $($process.Id))" -ForegroundColor Green
    } else {
        Write-Host "❌ Task started but process not found" -ForegroundColor Red
        Write-Host "Check logs: .\view-logs.ps1" -ForegroundColor Yellow
        exit 1
    }
}

Write-Host ""
Write-Host "Monitor is now running" -ForegroundColor Green
Write-Host ""
Write-Host "To view logs: .\view-logs.ps1" -ForegroundColor Yellow
Write-Host "To stop: .\stop.ps1" -ForegroundColor Yellow
Write-Host ""
