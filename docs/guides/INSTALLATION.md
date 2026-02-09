# Windows Browser Guard - Installation Guide

## Installation Methods

Windows Browser Guard can be installed in two ways:

### Method 1: Using the Installer Script (Recommended)

The installer script copies all necessary files to a centralized location and sets up the environment.

**Quick Start:**
```powershell
# Run as Administrator
.\Install.ps1
```

### Method 2: Manual Installation

Copy the executable and scripts to your preferred location and configure manually.

---

## Installer Script Usage

### Interactive Installation (Default)

```powershell
# Run the installer as Administrator
.\Install.ps1
```

**What happens:**
1. Prompts for installation path (default: `C:\Program Files\WindowsBrowserGuard`)
2. Copies executable and maintenance scripts
3. Copies documentation
4. Offers to add to system PATH
5. Creates uninstaller
6. Offers to set up scheduled task with OTLP configuration

### Command-Line Options

```powershell
# Install to custom directory
.\Install.ps1 -InstallPath "C:\Tools\WindowsBrowserGuard"

# Unattended installation (no prompts)
.\Install.ps1 -Unattended

# Skip task scheduler setup
.\Install.ps1 -SkipTaskSetup

# Combined options
.\Install.ps1 -InstallPath "C:\MyApps\WBG" -SkipTaskSetup
```

### Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `-InstallPath` | String | Installation directory | `C:\Program Files\WindowsBrowserGuard` |
| `-SkipTaskSetup` | Switch | Skip automatic task setup | `$false` |
| `-Unattended` | Switch | No interactive prompts | `$false` |

---

## What Gets Installed

### Files Copied

```
C:\Program Files\WindowsBrowserGuard\
├── WindowsBrowserGuard.exe          # Main executable
├── install-task.ps1                 # Task scheduler setup
├── uninstall-task.ps1              # Task removal
├── start.ps1                        # Start monitor
├── stop.ps1                         # Stop monitor
├── restart.ps1                      # Restart monitor
├── status.ps1                       # Show status
├── view-logs.ps1                    # View logs
├── start-monitor.ps1               # Legacy start script
├── README.md                        # Main documentation
├── PROJECT-SUMMARY.md              # Project overview
├── Uninstall.ps1                   # Generated uninstaller
└── docs\
    ├── README.md                    # User guide
    ├── guides\
    │   └── MAINTENANCE-SCRIPTS.md   # Complete script guide
    └── features\
        ├── OPENTELEMETRY.md
        ├── OPENTELEMETRY-LOGGING.md
        ├── OPENTELEMETRY-METRICS.md
        ├── OTLP-ENDPOINTS.md
        └── DRY-RUN-MODE.md
```

### Generated Files

- **Uninstall.ps1** - Custom uninstaller for this installation
- **WindowsBrowserGuard-config.json** - Created by `install-task.ps1`
- **WindowsBrowserGuard-wrapper.ps1** - Created by `install-task.ps1`
- **WindowsBrowserGuard-log.txt** - Created when monitor runs

---

## Installation Scenarios

### Scenario 1: Fresh Installation

```powershell
# 1. Run installer
PS> .\Install.ps1

# Prompts:
Install to 'C:\Program Files\WindowsBrowserGuard'? Y
Add installation directory to system PATH? Y
Set up Windows Browser Guard to start automatically at login? Y

# Configure OTLP when install-task.ps1 runs
Configure OTLP? Y
Endpoint: localhost:4317
Protocol: (1) gRPC
Disable TLS? Y

# Result:
# ✓ Files installed
# ✓ Added to PATH
# ✓ Scheduled task configured with OTLP
# ✓ Ready to use
```

### Scenario 2: Custom Location

```powershell
# Install to custom directory
PS> .\Install.ps1 -InstallPath "C:\MyApps\WindowsBrowserGuard"

# Or provide path interactively:
PS> .\Install.ps1
Install to 'C:\Program Files\WindowsBrowserGuard'? C:\MyApps\WindowsBrowserGuard
```

### Scenario 3: Upgrade Existing Installation

```powershell
# Run installer over existing installation
PS> .\Install.ps1

# Prompts:
⚠️  Installation directory already exists
  Existing version: 2026-02-08 10:30:15
  New version:      2026-02-09 14:25:30
Upgrade installation? Y

⚠️  WindowsBrowserGuard is currently running
  PID: 12345
Stop it before upgrading? Y

# Result:
# ✓ Process stopped
# ✓ Files upgraded
# ✓ Configuration preserved
```

### Scenario 4: Unattended/Automated Installation

```powershell
# Silent installation for deployment
PS> .\Install.ps1 -InstallPath "C:\Program Files\WindowsBrowserGuard" -Unattended -SkipTaskSetup

# Result:
# ✓ Installed without prompts
# ✓ Task setup skipped (configure later)
# ✓ Not added to PATH (do manually if needed)
```

### Scenario 5: Development/Testing

```powershell
# Install to local directory for testing
PS> .\Install.ps1 -InstallPath "C:\Dev\WBG-Test" -SkipTaskSetup

# Then test manually:
PS> cd "C:\Dev\WBG-Test"
PS> .\WindowsBrowserGuard.exe --dry-run
```

---

## Post-Installation Setup

After running the installer:

### Step 1: Configure Scheduled Task (if skipped)

```powershell
cd "C:\Program Files\WindowsBrowserGuard"
.\install-task.ps1
```

This will:
- Prompt for OTLP configuration
- Create scheduled task
- Set up automatic startup at login

### Step 2: Verify Installation

```powershell
# Check status
.\status.ps1

# Should show:
# - Installation path
# - OTLP configuration (if configured)
# - Scheduled task information
```

### Step 3: Start Monitor

```powershell
# Start via scheduled task
.\start.ps1

# Or start directly for testing
.\start.ps1 -Direct
```

### Step 4: Monitor Logs

```powershell
# View recent logs
.\view-logs.ps1

# Follow logs in real-time
.\view-logs.ps1 -Follow
```

---

## Upgrading

### Automatic Upgrade

```powershell
# Run installer again with same path
.\Install.ps1

# Installer will:
# 1. Detect existing installation
# 2. Offer to upgrade
# 3. Stop running process if needed
# 4. Replace files
# 5. Preserve configuration
```

### Manual Upgrade

```powershell
# 1. Stop the monitor
cd "C:\Program Files\WindowsBrowserGuard"
.\stop.ps1

# 2. Backup configuration (optional)
copy WindowsBrowserGuard-config.json WindowsBrowserGuard-config.json.backup

# 3. Copy new executable
copy "C:\Downloads\WindowsBrowserGuard.exe" .

# 4. Restart
.\start.ps1
```

---

## Uninstallation

### Using Generated Uninstaller

```powershell
cd "C:\Program Files\WindowsBrowserGuard"
.\Uninstall.ps1
```

**What it does:**
1. Stops running process
2. Removes scheduled task
3. Removes from system PATH
4. Optionally removes installation directory

### Manual Uninstallation

```powershell
# 1. Remove scheduled task
.\uninstall-task.ps1

# 2. Stop process
.\stop.ps1

# 3. Remove from PATH (optional)
# - Open System Properties → Environment Variables
# - Remove WindowsBrowserGuard path from System PATH

# 4. Delete installation directory
Remove-Item "C:\Program Files\WindowsBrowserGuard" -Recurse -Force
```

---

## System PATH Integration

Adding the installation directory to PATH allows you to run commands from anywhere:

```powershell
# With PATH configured:
PS C:\> WindowsBrowserGuard.exe --dry-run
PS C:\> status.ps1
PS C:\> view-logs.ps1

# Without PATH:
PS C:\> "C:\Program Files\WindowsBrowserGuard\WindowsBrowserGuard.exe" --dry-run
```

The installer offers to add to PATH automatically. You can also:

### Add to PATH Manually

```powershell
# Current user only
$path = [Environment]::GetEnvironmentVariable("Path", "User")
[Environment]::SetEnvironmentVariable("Path", "$path;C:\Program Files\WindowsBrowserGuard", "User")

# System-wide (requires admin)
$path = [Environment]::GetEnvironmentVariable("Path", "Machine")
[Environment]::SetEnvironmentVariable("Path", "$path;C:\Program Files\WindowsBrowserGuard", "Machine")
```

### Remove from PATH

```powershell
# System-wide (requires admin)
$path = [Environment]::GetEnvironmentVariable("Path", "Machine")
$newPath = ($path -split ';' | Where-Object { $_ -ne "C:\Program Files\WindowsBrowserGuard" }) -join ';'
[Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
```

---

## Troubleshooting

### Installer Won't Run

**Problem:** "This script must be run as Administrator"

**Solution:**
```powershell
# Right-click PowerShell → Run as Administrator
# Then run installer
.\Install.ps1
```

### Installation Directory Access Denied

**Problem:** Cannot create directory in Program Files

**Solution:**
```powershell
# Use different location
.\Install.ps1 -InstallPath "C:\Users\$env:USERNAME\AppData\Local\WindowsBrowserGuard"
```

### Process Won't Stop During Upgrade

**Problem:** Process is locked or unresponsive

**Solution:**
```powershell
# Force kill by PID
$pid = (Get-Process -Name "WindowsBrowserGuard").Id
Stop-Process -Id $pid -Force

# Then run installer again
.\Install.ps1
```

### Scheduled Task Setup Fails

**Problem:** install-task.ps1 errors during installation

**Solution:**
```powershell
# Skip task setup during install
.\Install.ps1 -SkipTaskSetup

# Configure manually later
cd "C:\Program Files\WindowsBrowserGuard"
.\install-task.ps1
```

### Files Not Copied

**Problem:** Some scripts missing after installation

**Solution:**
```powershell
# Verify source files exist
Get-ChildItem .\docs\*.ps1

# Run installer with verbose output
.\Install.ps1 -Verbose

# Copy missing files manually if needed
copy docs\*.ps1 "C:\Program Files\WindowsBrowserGuard\"
```

---

## Advanced Installation

### Silent Deployment via GPO

```powershell
# Create deployment script
$deployScript = @'
# Deploy WindowsBrowserGuard via GPO
$source = "\\fileserver\software\WindowsBrowserGuard"
$installer = Join-Path $source "Install.ps1"

if (Test-Path $installer) {
    & $installer -Unattended -InstallPath "C:\Program Files\WindowsBrowserGuard"
}
'@

# Deploy via Group Policy → Computer Configuration → Scripts → Startup
```

### Multi-Machine Deployment

```powershell
# Deploy to remote machines
$computers = "PC001", "PC002", "PC003"
$source = "\\fileserver\WindowsBrowserGuard"

foreach ($computer in $computers) {
    Write-Host "Deploying to $computer..."
    
    # Copy installer package
    $dest = "\\$computer\C$\Temp\WBG-Install"
    Copy-Item -Path $source -Destination $dest -Recurse -Force
    
    # Run installer remotely
    Invoke-Command -ComputerName $computer -ScriptBlock {
        Set-Location "C:\Temp\WBG-Install"
        .\Install.ps1 -Unattended
    }
}
```

### Docker/Container Installation

```dockerfile
# Not officially supported, but possible:
FROM mcr.microsoft.com/windows/servercore:ltsc2022
COPY WindowsBrowserGuard.exe C:/Program Files/WindowsBrowserGuard/
COPY docs/*.ps1 C:/Program Files/WindowsBrowserGuard/
WORKDIR "C:/Program Files/WindowsBrowserGuard"
CMD ["WindowsBrowserGuard.exe", "--otlp-endpoint=otel-collector:4317"]
```

---

## Verification

After installation, verify everything works:

```powershell
# 1. Check executable
cd "C:\Program Files\WindowsBrowserGuard"
.\WindowsBrowserGuard.exe --help

# 2. Check scripts
Get-ChildItem *.ps1

# 3. Test dry-run mode
.\WindowsBrowserGuard.exe --dry-run

# 4. Check status
.\status.ps1

# 5. Verify scheduled task
Get-ScheduledTask -TaskName "WindowsBrowserGuard"

# 6. Check PATH (if added)
$env:Path -split ';' | Select-String "WindowsBrowserGuard"
```

---

## Best Practices

1. **Use Default Location:** Install to `C:\Program Files\WindowsBrowserGuard` for consistency
2. **Add to PATH:** Makes maintenance scripts accessible from anywhere
3. **Configure OTLP:** Set up observability during installation
4. **Keep Uninstaller:** Don't delete `Uninstall.ps1` for clean removal later
5. **Backup Config:** Save `WindowsBrowserGuard-config.json` before upgrades
6. **Document Custom Paths:** If using non-standard location, document it
7. **Test Before Deploy:** Use `-SkipTaskSetup` to test in isolation first

---

## Quick Reference

```powershell
# Install
.\Install.ps1

# Install to custom location
.\Install.ps1 -InstallPath "C:\MyPath"

# Unattended install
.\Install.ps1 -Unattended

# Upgrade existing
.\Install.ps1  # Same path as before

# Uninstall
cd "C:\Program Files\WindowsBrowserGuard"
.\Uninstall.ps1

# Post-install setup
cd "C:\Program Files\WindowsBrowserGuard"
.\install-task.ps1  # Configure task
.\start.ps1         # Start monitor
.\status.ps1        # Check status
```

---

**See also:**
- [MAINTENANCE-SCRIPTS.md](docs/guides/MAINTENANCE-SCRIPTS.md) - Complete script guide
- [README.md](README.md) - User documentation
- [OTLP-ENDPOINTS.md](docs/features/OTLP-ENDPOINTS.md) - OTLP configuration
