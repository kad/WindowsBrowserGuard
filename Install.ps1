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

# ── Scheduled task ────────────────────────────────────────────────────────────
if (-not $SkipTaskSetup) {
    Write-Host ""
    Write-Host "Creating scheduled task '$taskName'..." -ForegroundColor Cyan

    $exePath   = Join-Path $InstallPath "WindowsBrowserGuard.exe"
    $action    = New-ScheduledTaskAction -Execute $exePath `
                     -Argument "--config=`"$configFile`""
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
