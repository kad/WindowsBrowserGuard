# Registry Extension Monitor - Log Viewer Script
# This script provides an easy way to view monitor logs

# Configuration
$scriptDir = $PSScriptRoot
$logPath = Join-Path $scriptDir "printwatch-log.txt"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Registry Extension Monitor - Log Viewer" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if log file exists
if (-not (Test-Path $logPath)) {
    Write-Host "❌ Log file not found: $logPath" -ForegroundColor Red
    Write-Host ""
    Write-Host "The log file will be created when the monitor starts running" -ForegroundColor Gray
    Write-Host "Make sure you installed the monitor using install-task.ps1" -ForegroundColor Gray
    Write-Host ""
    pause
    exit 1
}

# Get file info
$fileInfo = Get-Item $logPath
$fileSize = if ($fileInfo.Length -lt 1KB) { 
    "$($fileInfo.Length) bytes" 
} elseif ($fileInfo.Length -lt 1MB) { 
    "{0:N2} KB" -f ($fileInfo.Length / 1KB)
} else { 
    "{0:N2} MB" -f ($fileInfo.Length / 1MB)
}

Write-Host "Log file: $logPath" -ForegroundColor Gray
Write-Host "Size:     $fileSize" -ForegroundColor Gray
Write-Host "Modified: $($fileInfo.LastWriteTime)" -ForegroundColor Gray
Write-Host ""

# Check if process is running
$process = Get-Process -Name "printwatch" -ErrorAction SilentlyContinue
if ($process) {
    Write-Host "✓ Monitor is running (PID: $($process.Id))" -ForegroundColor Green
} else {
    Write-Host "⚠️  Monitor is not currently running" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Choose viewing option:" -ForegroundColor Yellow
Write-Host "1. View last 50 lines" -ForegroundColor White
Write-Host "2. View last 100 lines" -ForegroundColor White
Write-Host "3. View in real-time (tail -f mode)" -ForegroundColor White
Write-Host "4. Search for 'DETECTED' events" -ForegroundColor White
Write-Host "5. Search for 'ExtensionInstallForcelist'" -ForegroundColor White
Write-Host "6. Search for 'ExtensionSettings' (Firefox)" -ForegroundColor White
Write-Host "7. View all logs" -ForegroundColor White
Write-Host "8. Open log file in Notepad" -ForegroundColor White
Write-Host ""
$choice = Read-Host "Enter choice (1-8)"

Write-Host ""

switch ($choice) {
    "1" {
        Write-Host "Last 50 lines:" -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        Get-Content $logPath -Tail 50
    }
    "2" {
        Write-Host "Last 100 lines:" -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        Get-Content $logPath -Tail 100
    }
    "3" {
        Write-Host "Viewing logs in real-time (Press Ctrl+C to stop)..." -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        Get-Content $logPath -Tail 50 -Wait
    }
    "4" {
        Write-Host "Searching for 'DETECTED' events:" -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        $results = Get-Content $logPath | Select-String "DETECTED"
        if ($results) {
            $results
            Write-Host ""
            Write-Host "Found $($results.Count) matching lines" -ForegroundColor Green
        } else {
            Write-Host "No 'DETECTED' events found" -ForegroundColor Yellow
        }
    }
    "5" {
        Write-Host "Searching for 'ExtensionInstallForcelist' (Chrome):" -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        $results = Get-Content $logPath | Select-String "ExtensionInstallForcelist"
        if ($results) {
            $results
            Write-Host ""
            Write-Host "Found $($results.Count) matching lines" -ForegroundColor Green
        } else {
            Write-Host "No 'ExtensionInstallForcelist' events found" -ForegroundColor Yellow
        }
    }
    "6" {
        Write-Host "Searching for 'ExtensionSettings' (Firefox):" -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        $results = Get-Content $logPath | Select-String "ExtensionSettings"
        if ($results) {
            $results
            Write-Host ""
            Write-Host "Found $($results.Count) matching lines" -ForegroundColor Green
        } else {
            Write-Host "No 'ExtensionSettings' events found" -ForegroundColor Yellow
        }
    }
    "7" {
        Write-Host "All logs:" -ForegroundColor Cyan
        Write-Host "----------------------------------------" -ForegroundColor Gray
        Get-Content $logPath
    }
    "8" {
        Write-Host "Opening log file in Notepad..." -ForegroundColor Cyan
        notepad.exe $logPath
        exit 0
    }
    default {
        Write-Host "❌ Invalid choice" -ForegroundColor Red
        pause
        exit 1
    }
}

Write-Host ""
Write-Host "----------------------------------------" -ForegroundColor Gray
Write-Host ""
pause
