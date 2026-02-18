# Windows Browser Guard — Self-Elevating Installer
# Installs to C:\Program Files\WindowsBrowserGuard (default), creates a
# scheduled task that runs as SYSTEM at startup, and registers in Add/Remove Programs.

param(
    [string]$InstallPath   = "C:\Program Files\WindowsBrowserGuard",
    [switch]$SkipTaskSetup,
    [switch]$Unattended
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ── Self-elevation ───────────────────────────────────────────────────────────
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "Requesting administrator privileges..." -ForegroundColor Yellow
    $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$PSCommandPath`"",
                 '-InstallPath', "`"$InstallPath`"")
    if ($SkipTaskSetup) { $argList += '-SkipTaskSetup' }
    if ($Unattended)    { $argList += '-Unattended' }
    $proc = Start-Process -FilePath 'powershell.exe' -ArgumentList $argList `
                -Verb RunAs -WindowStyle Normal -Wait -PassThru
    exit $proc.ExitCode
}

# ── Constants ────────────────────────────────────────────────────────────────
$version      = "1.0.0"
$taskName     = "WindowsBrowserGuard"
$arpKey       = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\WindowsBrowserGuard"
$logDir       = "C:\ProgramData\WindowsBrowserGuard"
$logFile      = Join-Path $logDir "monitor.log"
$configFile   = Join-Path $InstallPath "config.json"
$wrapperFile  = Join-Path $InstallPath "run-wrapper.ps1"
$uninstFile   = Join-Path $InstallPath "Uninstall.ps1"
$scriptDir    = $PSScriptRoot
$srcExe       = Join-Path $scriptDir "WindowsBrowserGuard.exe"

# ── Banner ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "╔══════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║   Windows Browser Guard Installer v$version       ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""

# ── Verify source ────────────────────────────────────────────────────────────
if (-not (Test-Path $srcExe)) {
    Write-Host "❌ WindowsBrowserGuard.exe not found in: $scriptDir" -ForegroundColor Red
    Write-Host "   Run this script from the directory that contains the executable." -ForegroundColor Yellow
    if (-not $Unattended) { pause }
    exit 1
}
$exeItem = Get-Item $srcExe
$exeSizeKB = [math]::Round($exeItem.Length / 1KB)

Write-Host "  Source:  $scriptDir" -ForegroundColor Gray
Write-Host "  Dest:    $InstallPath" -ForegroundColor Gray
Write-Host "  Binary:  WindowsBrowserGuard.exe  ($exeSizeKB KB, $($exeItem.LastWriteTime.ToString('yyyy-MM-dd HH:mm')))" -ForegroundColor Gray
Write-Host "  Task:    $taskName (SYSTEM, runs at startup)" -ForegroundColor Gray
Write-Host "  Logs:    $logFile" -ForegroundColor Gray
Write-Host ""

if (-not $Unattended) {
    $ans = Read-Host "Install to '$InstallPath'? [Y/n/custom path]"
    if ($ans -eq 'n' -or $ans -eq 'N') { Write-Host "Cancelled." -ForegroundColor Yellow; exit 0 }
    if ($ans -ne '' -and $ans -ne 'y' -and $ans -ne 'Y') {
        $InstallPath = $ans.Trim('"').Trim("'")
        $configFile  = Join-Path $InstallPath "config.json"
        $wrapperFile = Join-Path $InstallPath "run-wrapper.ps1"
        $uninstFile  = Join-Path $InstallPath "Uninstall.ps1"
        Write-Host "  Using custom path: $InstallPath" -ForegroundColor Cyan
    }
}

# ── Detect existing installation ─────────────────────────────────────────────
$isUpgrade = Test-Path (Join-Path $InstallPath "WindowsBrowserGuard.exe")
$prevOTLP  = ""

if ($isUpgrade) {
    Write-Host "⚙️  Existing installation detected — upgrading..." -ForegroundColor Yellow

    # Load previous OTLP setting to preserve it
    if (Test-Path $configFile) {
        try {
            $prev = Get-Content $configFile -Raw | ConvertFrom-Json
            if ($prev.OTLPEndpoint) { $prevOTLP = $prev.OTLPEndpoint }
        } catch { }
    }

    if (-not $Unattended) {
        $ans = Read-Host "Upgrade existing installation? [Y/n]"
        if ($ans -eq 'n' -or $ans -eq 'N') { Write-Host "Cancelled." -ForegroundColor Yellow; exit 0 }
    }

    # Stop and deregister scheduled task so the exe can be replaced
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    if ($task) {
        Write-Host "  Stopping scheduled task '$taskName'..." -NoNewline
        Stop-ScheduledTask   -TaskName $taskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
        Write-Host " ✓" -ForegroundColor Green
    }

    # Stop process if still running
    $proc = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
    if ($proc) {
        Write-Host "  Stopping process (PID $($proc.Id))..." -NoNewline
        Stop-Process -Id $proc.Id -Force
        Start-Sleep -Seconds 2
        Write-Host " ✓" -ForegroundColor Green
    }
}

# ── Create install directory ──────────────────────────────────────────────────
if (-not (Test-Path $InstallPath)) {
    New-Item -Path $InstallPath -ItemType Directory -Force | Out-Null
    Write-Host "✓ Created: $InstallPath" -ForegroundColor Green
}
if (-not (Test-Path $logDir)) {
    New-Item -Path $logDir -ItemType Directory -Force | Out-Null
}

# ── Copy files ────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "Installing files..." -ForegroundColor Cyan

Copy-Item $srcExe -Destination $InstallPath -Force
Write-Host "  ✓ WindowsBrowserGuard.exe" -ForegroundColor Green

$docsDir = Join-Path $scriptDir "docs"
$scripts = @('start.ps1','stop.ps1','restart.ps1','status.ps1','view-logs.ps1','start-monitor.ps1','uninstall-task.ps1')
$copied = 0
foreach ($s in $scripts) {
    $src = Join-Path $docsDir $s
    if (Test-Path $src) {
        Copy-Item $src -Destination $InstallPath -Force
        $copied++
    }
}
if ($copied) { Write-Host "  ✓ $copied maintenance scripts" -ForegroundColor Green }

foreach ($md in @('README.md','PROJECT-SUMMARY.md')) {
    $src = Join-Path $scriptDir $md
    if (Test-Path $src) { Copy-Item $src -Destination $InstallPath -Force }
}

# ── OTLP configuration ────────────────────────────────────────────────────────
Write-Host ""
Write-Host "OpenTelemetry / OTLP Configuration" -ForegroundColor Cyan
Write-Host "───────────────────────────────────" -ForegroundColor Gray

$otlpEndpoint = $prevOTLP

if (-not $Unattended) {
    if ($otlpEndpoint) {
        Write-Host "  Existing endpoint: $otlpEndpoint" -ForegroundColor Gray
        $ans = Read-Host "  Keep it? [Y/n/new-url]"
        if ($ans -eq 'n' -or $ans -eq 'N') {
            $otlpEndpoint = ""
        } elseif ($ans -ne '' -and $ans -ne 'y' -and $ans -ne 'Y') {
            $otlpEndpoint = $ans.Trim()
        }
    } else {
        Write-Host "  Examples:" -ForegroundColor Gray
        Write-Host "    grpc://localhost:4317    (gRPC, local, no TLS)" -ForegroundColor DarkGray
        Write-Host "    grpcs://collector:443    (gRPC, TLS)" -ForegroundColor DarkGray
        Write-Host "    http://localhost:4318    (HTTP, local, no TLS)" -ForegroundColor DarkGray
        Write-Host "    https://otlp.corp.com    (HTTP, TLS, port 443)" -ForegroundColor DarkGray
        $ans = Read-Host "  OTLP endpoint URL (leave blank to skip)"
        if ($ans.Trim() -ne '') { $otlpEndpoint = $ans.Trim() }
    }
}

if ($otlpEndpoint) {
    Write-Host "  ✓ OTLP endpoint: $otlpEndpoint" -ForegroundColor Green
} else {
    Write-Host "  — OTLP disabled (stdout/log-file only)" -ForegroundColor Gray
}

# ── Save config ───────────────────────────────────────────────────────────────
@{
    InstallPath  = $InstallPath
    Version      = $version
    OTLPEndpoint = $otlpEndpoint
    LogPath      = $logFile
    TaskName     = $taskName
} | ConvertTo-Json | Set-Content $configFile -Force
Write-Host "  ✓ Config saved: $configFile" -ForegroundColor Green

# ── Generate run-wrapper.ps1 ──────────────────────────────────────────────────
$wrapperContent = @"
# Auto-generated by installer — do not edit by hand.
`$exePath    = '$($InstallPath -replace "'","''")\WindowsBrowserGuard.exe'
`$logPath    = '$($logFile -replace "'","''")'
`$configPath = '$($configFile -replace "'","''")'

# Ensure log directory exists
`$logDir = Split-Path -Parent `$logPath
if (-not (Test-Path `$logDir)) { New-Item -Path `$logDir -ItemType Directory -Force | Out-Null }

`$argList = @("--log-file=`$logPath")
if (Test-Path `$configPath) {
    `$cfg = Get-Content `$configPath -Raw | ConvertFrom-Json
    if (`$cfg.OTLPEndpoint) { `$argList += "--otlp-endpoint=`$(`$cfg.OTLPEndpoint)" }
}

& `$exePath @argList
"@
Set-Content -Path $wrapperFile -Value $wrapperContent -Force
Write-Host "  ✓ Wrapper script: $wrapperFile" -ForegroundColor Green

# ── Scheduled task ────────────────────────────────────────────────────────────
if (-not $SkipTaskSetup) {
    Write-Host ""
    Write-Host "Creating scheduled task '$taskName'..." -ForegroundColor Cyan

    $action    = New-ScheduledTaskAction -Execute 'powershell.exe' `
                     -Argument "-NonInteractive -NoProfile -ExecutionPolicy Bypass -File `"$wrapperFile`""
    $trigger   = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    $settings  = New-ScheduledTaskSettingsSet `
                     -ExecutionTimeLimit 0 `
                     -RestartCount 3 `
                     -RestartInterval (New-TimeSpan -Minutes 1) `
                     -StartWhenAvailable `
                     -AllowStartIfOnBatteries `
                     -DontStopIfGoingOnBatteries

    Register-ScheduledTask -TaskName $taskName `
        -Action $action -Trigger $trigger -Principal $principal -Settings $settings `
        -Description "Monitors Windows Registry for forced browser extension installs and blocks them." `
        -Force | Out-Null

    Write-Host "  ✓ Task registered (SYSTEM, runs at startup)" -ForegroundColor Green
}

# ── Add/Remove Programs ───────────────────────────────────────────────────────
Write-Host ""
Write-Host "Registering in Add/Remove Programs..." -ForegroundColor Cyan
if (-not (Test-Path $arpKey)) { New-Item -Path $arpKey -Force | Out-Null }
$uninstCmd = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$uninstFile`""
@{
    DisplayName          = 'Windows Browser Guard'
    DisplayVersion       = $version
    Publisher            = 'WindowsBrowserGuard'
    InstallLocation      = $InstallPath
    UninstallString      = $uninstCmd
    QuietUninstallString = "$uninstCmd -Unattended"
    DisplayIcon          = (Join-Path $InstallPath 'WindowsBrowserGuard.exe')
    NoModify             = 1
    NoRepair             = 1
    EstimatedSize        = $exeSizeKB
}.GetEnumerator() | ForEach-Object {
    $type = if ($_.Value -is [int]) { 'DWord' } else { 'String' }
    Set-ItemProperty -Path $arpKey -Name $_.Key -Value $_.Value -Type $type
}
Write-Host "  ✓ Registered in Add/Remove Programs" -ForegroundColor Green

# ── Generate self-elevating Uninstall.ps1 ─────────────────────────────────────
Write-Host ""
Write-Host "Writing uninstaller..." -ForegroundColor Cyan
$uninstContent = @"
# Windows Browser Guard — Uninstaller (auto-generated)
param([switch]`$Unattended)

`$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not `$isAdmin) {
    `$argList = @('-NoProfile','-ExecutionPolicy','Bypass','-File',"`"`$PSCommandPath`"")
    if (`$Unattended) { `$argList += '-Unattended' }
    `$proc = Start-Process powershell.exe -ArgumentList `$argList `
                -Verb RunAs -WindowStyle Normal -Wait -PassThru
    exit `$proc.ExitCode
}

`$installPath = '$($InstallPath -replace "'","''")'
`$taskName    = '$taskName'
`$arpKey      = '$arpKey'

Write-Host ""
Write-Host "Windows Browser Guard Uninstaller" -ForegroundColor Cyan
Write-Host "Install location: `$installPath" -ForegroundColor Gray
Write-Host ""

if (-not `$Unattended) {
    `$ans = Read-Host "Remove Windows Browser Guard? [Y/n]"
    if (`$ans -eq 'n' -or `$ans -eq 'N') { Write-Host "Cancelled." -ForegroundColor Yellow; exit 0 }
}

# Stop and deregister task
`$task = Get-ScheduledTask -TaskName `$taskName -ErrorAction SilentlyContinue
if (`$task) {
    Stop-ScheduledTask   -TaskName `$taskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName `$taskName -Confirm:`$false
    Write-Host "✓ Scheduled task removed" -ForegroundColor Green
}

# Stop process
`$proc = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
if (`$proc) { Stop-Process -Id `$proc.Id -Force; Start-Sleep -Seconds 1 }

# Remove from Add/Remove Programs
if (Test-Path `$arpKey) { Remove-Item `$arpKey -Recurse -Force; Write-Host "✓ Removed from Add/Remove Programs" -ForegroundColor Green }

# Remove from PATH
`$syspath = [Environment]::GetEnvironmentVariable('Path','Machine')
if (`$syspath -like "*`$installPath*") {
    `$newPath = (`$syspath -split ';' | Where-Object { `$_ -ne `$installPath }) -join ';'
    [Environment]::SetEnvironmentVariable('Path', `$newPath, 'Machine')
    Write-Host "✓ Removed from system PATH" -ForegroundColor Green
}

# Remove files
try {
    Remove-Item -Path `$installPath -Recurse -Force
    Write-Host "✓ Files removed" -ForegroundColor Green
} catch {
    Write-Host "⚠️  Could not remove all files: `$_" -ForegroundColor Yellow
    Write-Host "   Manually delete: `$installPath" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "✓ Windows Browser Guard uninstalled." -ForegroundColor Green
if (-not `$Unattended) { pause }
"@
Set-Content -Path $uninstFile -Value $uninstContent -Force
Write-Host "  ✓ Uninstaller: $uninstFile" -ForegroundColor Green

# ── Optional: add to system PATH ─────────────────────────────────────────────
if (-not $Unattended) {
    Write-Host ""
    $ans = Read-Host "Add '$InstallPath' to system PATH? [y/N]"
    if ($ans -eq 'y' -or $ans -eq 'Y') {
        $cur = [Environment]::GetEnvironmentVariable('Path','Machine')
        if ($cur -notlike "*$InstallPath*") {
            [Environment]::SetEnvironmentVariable('Path', "$cur;$InstallPath", 'Machine')
            Write-Host "  ✓ Added to system PATH (restart terminal to take effect)" -ForegroundColor Green
        } else {
            Write-Host "  ✓ Already in PATH" -ForegroundColor Green
        }
    }
}

# ── Start now? ───────────────────────────────────────────────────────────────
if (-not $SkipTaskSetup -and -not $Unattended) {
    Write-Host ""
    $ans = Read-Host "Start the monitor now? [Y/n]"
    if ($ans -ne 'n' -and $ans -ne 'N') {
        Start-ScheduledTask -TaskName $taskName
        Start-Sleep -Seconds 2
        $proc = Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
        if ($proc) {
            Write-Host "  ✓ Monitor started (PID $($proc.Id))" -ForegroundColor Green
        } else {
            Write-Host "  ⚠️  Task launched but process not detected yet — check logs" -ForegroundColor Yellow
        }
    }
}

# ── Summary ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "╔══════════════════════════════════════════════════╗" -ForegroundColor Green
Write-Host "║   Installation complete!                         ║" -ForegroundColor Green
Write-Host "╚══════════════════════════════════════════════════╝" -ForegroundColor Green
Write-Host ""
Write-Host "  Installed to: $InstallPath" -ForegroundColor White
if ($otlpEndpoint) {
    Write-Host "  OTLP:         $otlpEndpoint" -ForegroundColor White
}
Write-Host "  Logs:         $logFile" -ForegroundColor White
Write-Host "  Task:         $taskName  (starts at system boot)" -ForegroundColor White
Write-Host ""
Write-Host "Useful commands (run from $InstallPath):" -ForegroundColor Yellow
Write-Host "  .\status.ps1     — show running state, recent logs" -ForegroundColor Gray
Write-Host "  .\stop.ps1       — stop the monitor" -ForegroundColor Gray
Write-Host "  .\restart.ps1    — restart the monitor" -ForegroundColor Gray
Write-Host "  .\view-logs.ps1  — tail the log file" -ForegroundColor Gray
Write-Host "  .\Uninstall.ps1  — remove everything" -ForegroundColor Gray
Write-Host ""
if (-not $Unattended) { pause }


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
