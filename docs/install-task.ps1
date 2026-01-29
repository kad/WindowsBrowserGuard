# Registry Extension Monitor - Task Scheduler Installation Script
# This script creates a scheduled task that runs the monitor at user login with Administrator privileges

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "❌ This script must be run as Administrator" -ForegroundColor Red
    Write-Host "Right-click on PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    pause
    exit 1
}

# Configuration
$scriptDir = $PSScriptRoot
$exePath = Join-Path $scriptDir "printwatch.exe"
$logPath = Join-Path $scriptDir "printwatch-log.txt"
$wrapperPath = Join-Path $scriptDir "printwatch-wrapper.ps1"
$taskName = "RegistryExtensionMonitor"

# Validate executable exists
if (-not (Test-Path $exePath)) {
    Write-Host "❌ Error: printwatch.exe not found at: $exePath" -ForegroundColor Red
    Write-Host "Please ensure printwatch.exe is in the same directory as this script" -ForegroundColor Yellow
    pause
    exit 1
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Registry Extension Monitor Installer" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Executable: $exePath" -ForegroundColor Gray
Write-Host "Log file:   $logPath" -ForegroundColor Gray
Write-Host "Task name:  $taskName" -ForegroundColor Gray
Write-Host ""

# Check if task already exists
$existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host "⚠️  Task '$taskName' already exists" -ForegroundColor Yellow
    $response = Read-Host "Do you want to remove it and create a new one? (Y/N)"
    if ($response -ne 'Y' -and $response -ne 'y') {
        Write-Host "Installation cancelled" -ForegroundColor Yellow
        pause
        exit 0
    }
    Write-Host "Removing existing task..." -ForegroundColor Yellow
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    Write-Host "✓ Existing task removed" -ForegroundColor Green
}

# Create wrapper script that redirects output to log file
Write-Host "Creating wrapper script..." -ForegroundColor Cyan
$wrapperScript = @"
# Wrapper script for Registry Extension Monitor
# This script redirects program output to a log file

`$exePath = '$exePath'
`$logPath = '$logPath'

# Create log file if it doesn't exist
if (-not (Test-Path `$logPath)) {
    New-Item -Path `$logPath -ItemType File -Force | Out-Null
}

# Start the monitor and redirect output
Start-Process -FilePath `$exePath -NoNewWindow -RedirectStandardOutput `$logPath -RedirectStandardError `$logPath -Wait
"@

Set-Content -Path $wrapperPath -Value $wrapperScript -Force
Write-Host "✓ Wrapper script created: $wrapperPath" -ForegroundColor Green

# Create the scheduled task
Write-Host "Creating scheduled task..." -ForegroundColor Cyan

$action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-WindowStyle Hidden -ExecutionPolicy Bypass -File `"$wrapperPath`""
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0

try {
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description "Monitors registry for unwanted extension installations and automatically blocks them" -ErrorAction Stop | Out-Null
    Write-Host "✓ Scheduled task created successfully" -ForegroundColor Green
} catch {
    Write-Host "❌ Failed to create scheduled task: $_" -ForegroundColor Red
    pause
    exit 1
}

# Verify task was created
$verifyTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
if ($verifyTask) {
    Write-Host "✓ Task verified in Task Scheduler" -ForegroundColor Green
} else {
    Write-Host "⚠️  Task created but verification failed" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Installation Complete!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "The Registry Extension Monitor will:" -ForegroundColor White
Write-Host "  • Start automatically at your next login" -ForegroundColor Gray
Write-Host "  • Run with Administrator privileges" -ForegroundColor Gray
Write-Host "  • Monitor registry for extension installations" -ForegroundColor Gray
Write-Host "  • Write logs to: $logPath" -ForegroundColor Gray
Write-Host ""
Write-Host "To start the monitor now without waiting for login:" -ForegroundColor Yellow
Write-Host "  Start-ScheduledTask -TaskName '$taskName'" -ForegroundColor Cyan
Write-Host ""
Write-Host "To view logs:" -ForegroundColor Yellow
Write-Host "  Get-Content '$logPath' -Tail 50 -Wait" -ForegroundColor Cyan
Write-Host ""
Write-Host "To uninstall, run: uninstall-task.ps1" -ForegroundColor Yellow
Write-Host ""

# Ask if user wants to start the task now
$startNow = Read-Host "Do you want to start the monitor now? (Y/N)"
if ($startNow -eq 'Y' -or $startNow -eq 'y') {
    Write-Host "Starting task..." -ForegroundColor Cyan
    Start-ScheduledTask -TaskName $taskName
    Start-Sleep -Seconds 2
    
    # Check if process is running
    $process = Get-Process -Name "printwatch" -ErrorAction SilentlyContinue
    if ($process) {
        Write-Host "✓ Monitor is now running (PID: $($process.Id))" -ForegroundColor Green
    } else {
        Write-Host "⚠️  Task started but process not found. Check the log file for details." -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "Press any key to exit..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
