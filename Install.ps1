# Windows Browser Guard Installer
# This script installs WindowsBrowserGuard to a specified directory (default: C:\Program Files\WindowsBrowserGuard)

param(
    [string]$InstallPath = "C:\Program Files\WindowsBrowserGuard",
    [switch]$SkipTaskSetup,
    [switch]$Unattended
)

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "❌ This script must be run as Administrator" -ForegroundColor Red
    Write-Host "Right-click on PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    if (-not $Unattended) { pause }
    exit 1
}

$scriptDir = $PSScriptRoot
$version = "1.0.0"  # Update this with each release

Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║  Windows Browser Guard Installer v$version         ║" -ForegroundColor Cyan
Write-Host "╚═══════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""

# Verify executable exists
$exePath = Join-Path $scriptDir "WindowsBrowserGuard.exe"
if (-not (Test-Path $exePath)) {
    Write-Host "❌ Error: WindowsBrowserGuard.exe not found" -ForegroundColor Red
    Write-Host "Expected location: $exePath" -ForegroundColor Yellow
    Write-Host "Please run this script from the build directory" -ForegroundColor Yellow
    if (-not $Unattended) { pause }
    exit 1
}

# Get file version info
$exeInfo = Get-Item $exePath
$exeSize = [math]::Round($exeInfo.Length / 1MB, 2)
$exeDate = $exeInfo.LastWriteTime

Write-Host "Installation Settings:" -ForegroundColor Yellow
Write-Host "  Source:      $scriptDir" -ForegroundColor Gray
Write-Host "  Destination: $InstallPath" -ForegroundColor Gray
Write-Host "  Executable:  WindowsBrowserGuard.exe ($exeSize MB)" -ForegroundColor Gray
Write-Host "  Built:       $exeDate" -ForegroundColor Gray
Write-Host ""

# Confirm installation path
if (-not $Unattended) {
    $confirm = Read-Host "Install to '$InstallPath'? (Y/N or enter custom path)"
    if ($confirm -eq 'N' -or $confirm -eq 'n') {
        Write-Host "Installation cancelled" -ForegroundColor Yellow
        pause
        exit 0
    } elseif ($confirm -ne 'Y' -and $confirm -ne 'y' -and $confirm -ne '') {
        $InstallPath = $confirm
        Write-Host "Installing to custom path: $InstallPath" -ForegroundColor Cyan
    }
}

# Check if already installed
if (Test-Path $InstallPath) {
    Write-Host ""
    Write-Host "⚠️  Installation directory already exists" -ForegroundColor Yellow
    
    $existingExe = Join-Path $InstallPath "WindowsBrowserGuard.exe"
    if (Test-Path $existingExe) {
        $existingInfo = Get-Item $existingExe
        Write-Host "  Existing version: $($existingInfo.LastWriteTime)" -ForegroundColor Gray
        Write-Host "  New version:      $exeDate" -ForegroundColor Gray
        
        if (-not $Unattended) {
            $upgrade = Read-Host "Upgrade installation? (Y/N)"
            if ($upgrade -ne 'Y' -and $upgrade -ne 'y') {
                Write-Host "Installation cancelled" -ForegroundColor Yellow
                pause
                exit 0
            }
        }
        
        # Check if process is running
        $process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
        if ($process) {
            Write-Host ""
            Write-Host "⚠️  WindowsBrowserGuard is currently running" -ForegroundColor Yellow
            Write-Host "  PID: $($process.Id)" -ForegroundColor Gray
            
            if (-not $Unattended) {
                $stop = Read-Host "Stop it before upgrading? (Y/N)"
                if ($stop -eq 'Y' -or $stop -eq 'y') {
                    Write-Host "Stopping process..." -ForegroundColor Cyan
                    Stop-Process -Id $process.Id -Force
                    Start-Sleep -Seconds 2
                    Write-Host "✓ Process stopped" -ForegroundColor Green
                } else {
                    Write-Host "❌ Cannot upgrade while process is running" -ForegroundColor Red
                    pause
                    exit 1
                }
            } else {
                Write-Host "Stopping process (unattended mode)..." -ForegroundColor Cyan
                Stop-Process -Id $process.Id -Force
                Start-Sleep -Seconds 2
            }
        }
    }
} else {
    # Create installation directory
    Write-Host "Creating installation directory..." -ForegroundColor Cyan
    try {
        New-Item -Path $InstallPath -ItemType Directory -Force | Out-Null
        Write-Host "✓ Directory created: $InstallPath" -ForegroundColor Green
    } catch {
        Write-Host "❌ Failed to create directory: $_" -ForegroundColor Red
        if (-not $Unattended) { pause }
        exit 1
    }
}

Write-Host ""
Write-Host "Installing files..." -ForegroundColor Cyan
Write-Host "──────────────────────────────────────────────────" -ForegroundColor Gray

# Files to copy
$filesToCopy = @(
    @{ Name = "WindowsBrowserGuard.exe"; Required = $true; Description = "Main executable" }
)

# PowerShell scripts to copy
$scriptsDir = Join-Path $scriptDir "docs"
$scripts = @(
    "install-task.ps1"
    "uninstall-task.ps1"
    "start.ps1"
    "stop.ps1"
    "restart.ps1"
    "status.ps1"
    "view-logs.ps1"
    "start-monitor.ps1"
)

$copyCount = 0
$errorCount = 0

# Copy main executable
Write-Host "  Copying executable..." -NoNewline
try {
    Copy-Item -Path $exePath -Destination $InstallPath -Force -ErrorAction Stop
    Write-Host " ✓" -ForegroundColor Green
    $copyCount++
} catch {
    Write-Host " ✗" -ForegroundColor Red
    Write-Host "    Error: $_" -ForegroundColor Red
    $errorCount++
}

# Copy PowerShell scripts
Write-Host "  Copying maintenance scripts..." -NoNewline
$scriptsCopied = 0
foreach ($script in $scripts) {
    $srcPath = Join-Path $scriptsDir $script
    if (Test-Path $srcPath) {
        try {
            Copy-Item -Path $srcPath -Destination $InstallPath -Force -ErrorAction Stop
            $scriptsCopied++
        } catch {
            Write-Host ""
            Write-Host "    Warning: Failed to copy $script" -ForegroundColor Yellow
        }
    } else {
        Write-Host ""
        Write-Host "    Warning: $script not found" -ForegroundColor Yellow
    }
}
Write-Host " ✓ ($scriptsCopied scripts)" -ForegroundColor Green
$copyCount += $scriptsCopied

# Copy documentation (optional)
$docFiles = @("README.md", "PROJECT-SUMMARY.md")
Write-Host "  Copying documentation..." -NoNewline
$docsCopied = 0
foreach ($docFile in $docFiles) {
    $srcPath = Join-Path $scriptDir $docFile
    if (Test-Path $srcPath) {
        try {
            Copy-Item -Path $srcPath -Destination $InstallPath -Force -ErrorAction Stop
            $docsCopied++
        } catch {
            # Non-critical, continue
        }
    }
}
if ($docsCopied -gt 0) {
    Write-Host " ✓ ($docsCopied files)" -ForegroundColor Green
} else {
    Write-Host " - (skipped)" -ForegroundColor Gray
}

# Create docs subdirectory and copy feature documentation
$installDocsPath = Join-Path $InstallPath "docs"
if (-not (Test-Path $installDocsPath)) {
    New-Item -Path $installDocsPath -ItemType Directory -Force | Out-Null
}

$docsSourcePath = Join-Path $scriptDir "docs"
if (Test-Path $docsSourcePath) {
    # Copy guides
    $guidesSource = Join-Path $docsSourcePath "guides"
    $guidesTarget = Join-Path $installDocsPath "guides"
    if (Test-Path $guidesSource) {
        Write-Host "  Copying user guides..." -NoNewline
        try {
            Copy-Item -Path $guidesSource -Destination $installDocsPath -Recurse -Force -ErrorAction Stop
            $guideCount = (Get-ChildItem -Path $guidesTarget -File).Count
            Write-Host " ✓ ($guideCount guides)" -ForegroundColor Green
        } catch {
            Write-Host " - (skipped)" -ForegroundColor Gray
        }
    }
    
    # Copy features documentation
    $featuresSource = Join-Path $docsSourcePath "features"
    $featuresTarget = Join-Path $installDocsPath "features"
    if (Test-Path $featuresSource) {
        Write-Host "  Copying feature documentation..." -NoNewline
        try {
            Copy-Item -Path $featuresSource -Destination $installDocsPath -Recurse -Force -ErrorAction Stop
            $featureCount = (Get-ChildItem -Path $featuresTarget -File).Count
            Write-Host " ✓ ($featureCount docs)" -ForegroundColor Green
        } catch {
            Write-Host " - (skipped)" -ForegroundColor Gray
        }
    }
    
    # Copy README from docs
    $docsReadme = Join-Path $docsSourcePath "README.md"
    if (Test-Path $docsReadme) {
        Copy-Item -Path $docsReadme -Destination $installDocsPath -Force -ErrorAction SilentlyContinue
    }
}

Write-Host ""
Write-Host "──────────────────────────────────────────────────" -ForegroundColor Gray
Write-Host "✓ Installation completed: $copyCount files copied" -ForegroundColor Green

if ($errorCount -gt 0) {
    Write-Host "⚠️  $errorCount errors occurred" -ForegroundColor Yellow
}

# Add to PATH (optional)
Write-Host ""
if (-not $Unattended) {
    $addToPath = Read-Host "Add installation directory to system PATH? (Y/N)"
    if ($addToPath -eq 'Y' -or $addToPath -eq 'y') {
        Write-Host "Adding to PATH..." -ForegroundColor Cyan
        
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$InstallPath*") {
            try {
                $newPath = $currentPath + ";" + $InstallPath
                [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
                Write-Host "✓ Added to system PATH" -ForegroundColor Green
                Write-Host "  Restart your terminal to use the new PATH" -ForegroundColor Yellow
            } catch {
                Write-Host "❌ Failed to modify PATH: $_" -ForegroundColor Red
                Write-Host "  You can add it manually: $InstallPath" -ForegroundColor Yellow
            }
        } else {
            Write-Host "✓ Already in PATH" -ForegroundColor Green
        }
    }
}

# Create uninstaller
Write-Host ""
Write-Host "Creating uninstaller..." -ForegroundColor Cyan
$uninstallerPath = Join-Path $InstallPath "Uninstall.ps1"
$uninstallerScript = @"
# Windows Browser Guard Uninstaller
# Generated by installer on $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')

`$installPath = '$InstallPath'

Write-Host ""
Write-Host "Windows Browser Guard Uninstaller" -ForegroundColor Cyan
Write-Host "Installation: `$installPath" -ForegroundColor Gray
Write-Host ""

# Check if running as Administrator
`$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not `$isAdmin) {
    Write-Host "❌ This script must be run as Administrator" -ForegroundColor Red
    pause
    exit 1
}

# Stop process if running
`$process = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
if (`$process) {
    Write-Host "Stopping WindowsBrowserGuard process..." -ForegroundColor Cyan
    Stop-Process -Id `$process.Id -Force
    Start-Sleep -Seconds 2
    Write-Host "✓ Process stopped" -ForegroundColor Green
}

# Remove scheduled task if exists
`$task = Get-ScheduledTask -TaskName "WindowsBrowserGuard" -ErrorAction SilentlyContinue
if (`$task) {
    Write-Host "Removing scheduled task..." -ForegroundColor Cyan
    Unregister-ScheduledTask -TaskName "WindowsBrowserGuard" -Confirm:`$false
    Write-Host "✓ Scheduled task removed" -ForegroundColor Green
}

# Remove from PATH
Write-Host "Removing from PATH..." -ForegroundColor Cyan
`$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if (`$currentPath -like "*`$installPath*") {
    `$newPath = (`$currentPath -split ';' | Where-Object { `$_ -ne `$installPath }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", `$newPath, "Machine")
    Write-Host "✓ Removed from PATH" -ForegroundColor Green
}

# Remove installation directory
Write-Host ""
`$removeAll = Read-Host "Remove installation directory and all files? (Y/N)"
if (`$removeAll -eq 'Y' -or `$removeAll -eq 'y') {
    Write-Host "Removing installation directory..." -ForegroundColor Cyan
    try {
        Remove-Item -Path `$installPath -Recurse -Force -ErrorAction Stop
        Write-Host "✓ Installation removed" -ForegroundColor Green
        Write-Host ""
        Write-Host "Windows Browser Guard has been completely uninstalled" -ForegroundColor Green
    } catch {
        Write-Host "❌ Failed to remove directory: `$_" -ForegroundColor Red
        Write-Host "You may need to remove it manually: `$installPath" -ForegroundColor Yellow
    }
} else {
    Write-Host "Installation directory preserved: `$installPath" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Press any key to exit..."
`$null = `$Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
"@

try {
    Set-Content -Path $uninstallerPath -Value $uninstallerScript -Force
    Write-Host "✓ Uninstaller created: Uninstall.ps1" -ForegroundColor Green
} catch {
    Write-Host "⚠️  Failed to create uninstaller: $_" -ForegroundColor Yellow
}

# Setup scheduled task
if (-not $SkipTaskSetup -and -not $Unattended) {
    Write-Host ""
    Write-Host "╔═══════════════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║  Scheduled Task Setup                             ║" -ForegroundColor Cyan
    Write-Host "╚═══════════════════════════════════════════════════╝" -ForegroundColor Cyan
    Write-Host ""
    
    $setupTask = Read-Host "Set up Windows Browser Guard to start automatically at login? (Y/N)"
    if ($setupTask -eq 'Y' -or $setupTask -eq 'y') {
        $installTaskScript = Join-Path $InstallPath "install-task.ps1"
        if (Test-Path $installTaskScript) {
            Write-Host ""
            Write-Host "Running task setup script..." -ForegroundColor Cyan
            Write-Host ""
            
            # Run install-task.ps1 from installation directory
            Set-Location -Path $InstallPath
            & $installTaskScript
        } else {
            Write-Host "❌ install-task.ps1 not found in installation directory" -ForegroundColor Red
        }
    }
}

# Summary
Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║  Installation Summary                             ║" -ForegroundColor Cyan
Write-Host "╚═══════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""
Write-Host "Installation Path:     $InstallPath" -ForegroundColor White
Write-Host "Executable:            WindowsBrowserGuard.exe" -ForegroundColor White
Write-Host "Scripts:               $scriptsCopied PowerShell scripts" -ForegroundColor White
Write-Host "Documentation:         docs/" -ForegroundColor White
Write-Host "Uninstaller:           Uninstall.ps1" -ForegroundColor White
Write-Host ""

Write-Host "Quick Start:" -ForegroundColor Yellow
Write-Host "  1. Configure task:  " -NoNewline -ForegroundColor Gray
Write-Host "cd '$InstallPath' && .\install-task.ps1" -ForegroundColor Cyan
Write-Host "  2. Start monitor:   " -NoNewline -ForegroundColor Gray
Write-Host ".\start.ps1" -ForegroundColor Cyan
Write-Host "  3. Check status:    " -NoNewline -ForegroundColor Gray
Write-Host ".\status.ps1" -ForegroundColor Cyan
Write-Host "  4. View logs:       " -NoNewline -ForegroundColor Gray
Write-Host ".\view-logs.ps1" -ForegroundColor Cyan
Write-Host ""

Write-Host "Documentation:" -ForegroundColor Yellow
Write-Host "  README.md" -ForegroundColor Cyan
Write-Host "  docs\guides\MAINTENANCE-SCRIPTS.md" -ForegroundColor Cyan
Write-Host "  docs\features\" -ForegroundColor Cyan
Write-Host ""

Write-Host "To uninstall, run: " -NoNewline -ForegroundColor Gray
Write-Host ".\Uninstall.ps1" -ForegroundColor Cyan
Write-Host ""

if (-not $Unattended) {
    Write-Host "Press any key to exit..."
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
}
