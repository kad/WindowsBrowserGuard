# Windows Browser Guard

A Windows registry monitoring tool that automatically detects and blocks unwanted browser extension installations via Group Policy.

## Features

- **Real-time Registry Monitoring**: Watches `HKLM\SOFTWARE\Policies` for changes recursively
- **Chrome Extension Protection**: Detects `ExtensionInstallForcelist` policies, extracts extension IDs, adds them to `ExtensionInstallBlocklist`, then removes the forcelist entry
- **Firefox Extension Protection**: Detects Firefox `ExtensionSettings` force install policies, creates blocklist entries with `installation_mode = "blocked"`, then removes the install policy
- **Detailed Change Tracking**: Shows exactly what changed (added/modified/removed keys and values)
- **Automatic Privilege Elevation**: Requests Administrator access if not running elevated

## Requirements

- Windows OS
- **Administrator privileges** (required for registry deletion)
- Go 1.16+ (for building from source)

## Quick Start

The easiest way to use Windows Browser Guard is with the included helper scripts:

```powershell
cd docs

# Install as scheduled task (runs at login)
.\install-task.ps1

# Or start manually
.\start-monitor.ps1

# View logs
.\view-logs.ps1

# Uninstall
.\uninstall-task.ps1
```

## Setup: Auto-Start at User Login

### Option 1: Use install-task.ps1 Script (Recommended)

The simplest method - just run the installer:

```powershell
cd docs
.\install-task.ps1
```

This creates a scheduled task with:
- Automatic start at user login
- Administrator privileges
- Output logging to WindowsBrowserGuard-log.txt
- Hidden background execution

### Option 2: Manual Task Scheduler Setup

Create a scheduled task manually:

```powershell
# Set the path to your executable (update this path to match your installation)
$exePath = "C:\Path\To\WindowsBrowserGuard.exe"
$logPath = "C:\Path\To\WindowsBrowserGuard-log.txt"
$taskName = "WindowsBrowserGuard"

# Create a wrapper script that redirects output to a log file
$wrapperScript = @"
Start-Process -FilePath '$exePath' -NoNewWindow -RedirectStandardOutput '$logPath' -RedirectStandardError '$logPath' -Wait
"@
$wrapperPath = "C:\Path\To\WindowsBrowserGuard-wrapper.ps1"
Set-Content -Path $wrapperPath -Value $wrapperScript

# Create the scheduled task
$action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-WindowStyle Hidden -ExecutionPolicy Bypass -File `"$wrapperPath`""
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description "Monitors registry for unwanted extension installations"

Write-Host "✓ Scheduled task created: $taskName"
Write-Host "✓ Logs will be written to: $logPath"
Write-Host "The program will start automatically at next login with Administrator privileges"
```

**Viewing Task Scheduler Logs:**

When running as a scheduled task, program output is redirected to the log file specified above. To view logs:

```powershell
# View the log file in real-time
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 50 -Wait

# View recent logs
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 100

# Search for specific events
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" | Select-String "ExtensionInstallForcelist"
```

**Check Task Scheduler History:**

Task execution history can be viewed in Task Scheduler:

```powershell
# Open Task Scheduler GUI
taskschd.msc

# Or check task status via PowerShell
Get-ScheduledTaskInfo -TaskName "WindowsBrowserGuard"
```

In Task Scheduler GUI:
1. Open Task Scheduler (Win + R → `taskschd.msc`)
2. Navigate to "Task Scheduler Library"
3. Find "WindowsBrowserGuard"
4. Click the "History" tab to see execution events

**Verify the task:**
```powershell
Get-ScheduledTask -TaskName "WindowsBrowserGuard"
```

**Start the task manually (without waiting for login):**
```powershell
Start-ScheduledTask -TaskName "WindowsBrowserGuard"
```

**Remove the task:**
```powershell
Unregister-ScheduledTask -TaskName "WindowsBrowserGuard" -Confirm:$false
```

### Option 2: Registry Run Key (Simpler but requires UAC prompt)

Add to registry startup (will prompt for elevation at each login):

```powershell
$exePath = "C:\Path\To\WindowsBrowserGuard.exe"
$regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$valueName = "WindowsBrowserGuard"

Set-ItemProperty -Path $regPath -Name $valueName -Value $exePath
Write-Host "✓ Added to startup registry"
```

**Remove from startup:**
```powershell
Remove-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WindowsBrowserGuard"
```

### Option 3: Startup Folder (Least Recommended)

Create a shortcut in the Startup folder:

```powershell
$exePath = "C:\Path\To\WindowsBrowserGuard.exe"
$startupFolder = [Environment]::GetFolderPath('Startup')
$shortcutPath = Join-Path $startupFolder "WindowsBrowserGuard.lnk"

$WScriptShell = New-Object -ComObject WScript.Shell
$shortcut = $WScriptShell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $exePath
$shortcut.WorkingDirectory = Split-Path $exePath
$shortcut.Save()

Write-Host "✓ Shortcut created in Startup folder: $shortcutPath"
```

## Running in Background

### Start as Background Process

**Note**: These examples are for manual usage. Use `start-monitor.ps1` script for easier operation.

```powershell
# Start with PowerShell wrapper for output redirection
Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -Command `"& 'C:\Path\To\WindowsBrowserGuard.exe' *>&1 | Tee-Object -FilePath 'C:\Path\To\WindowsBrowserGuard-log.txt'`"" -WindowStyle Hidden
```

## Managing the Running Process

### Check if Running

```powershell
Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
```

### View Process Details

```powershell
Get-Process -Name "WindowsBrowserGuard" | Format-List *
```

### Stop the Process

```powershell
Stop-Process -Name "WindowsBrowserGuard" -Force
```

### Stop by Process ID (if multiple instances)

```powershell
# Find the process ID
Get-Process -Name "WindowsBrowserGuard"

# Stop specific instance
Stop-Process -Id <PID> -Force
```

## Viewing Logs

### Use view-logs.ps1 Script (Recommended)

The easiest way to view logs:

```powershell
cd docs
.\view-logs.ps1
```

### Manual Log Viewing

If you set up the program with Task Scheduler or used the scripts, logs are written to:

```powershell
# View log file in real-time
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 50 -Wait

# View last 100 lines
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 100

# Search for specific events
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" | Select-String "DETECTED"
```

### For Console/Interactive Runs

If running in a console window, output appears directly in the terminal. No log file is created unless you redirect output.

## How It Works

### Chrome Extension Monitoring

1. Detects when a value is added to a key containing `ExtensionInstallForcelist`
2. Extracts the extension ID (string before the first `;`)
3. Adds the extension ID to the corresponding `ExtensionInstallBlocklist` key
4. Deletes the entire `ExtensionInstallForcelist` key

**Example:**
- Detects: `HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist\1 = "extensionid;https://update.url"`
- Adds: `HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallBlocklist\1 = "extensionid"`
- Deletes: `HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist`

### Firefox Extension Monitoring

1. Detects when `installation_mode` is set to `force_installed` or `normal_installed` under `ExtensionSettings`
2. Extracts the extension ID from the registry path
3. Creates a blocklist entry with `installation_mode = "blocked"`
4. Deletes the original install policy key

**Example:**
- Detects: `Mozilla\Firefox\ExtensionSettings\{ext-id}\installation_mode = "force_installed"`
- Creates: `Mozilla\Firefox\ExtensionSettings\{ext-id}\installation_mode = "blocked"`
- Deletes: The original `{ext-id}` key

## Troubleshooting

### Program doesn't start at login
- Verify the scheduled task exists: `Get-ScheduledTask -TaskName "WindowsBrowserGuard"`
- Check task history in Task Scheduler (taskschd.msc)
- Ensure the executable path is correct

### "Insufficient privileges" errors
- The program must run as Administrator
- If using Task Scheduler, ensure "Run with highest privileges" is checked
- Manually run: Right-click executable → "Run as administrator"

### Registry keys not being deleted
- Verify Administrator privileges: The program will show status on startup
- Check Windows Event Viewer for access denied errors
- Ensure no other process has the registry key open

### Program crashes or stops
- Check the log file if output was redirected
- Run manually in a console to see error messages:
  ```powershell
  Start-Process -FilePath ".\WindowsBrowserGuard.exe" -Verb RunAs -Wait
  ```

### Can't find the process
```powershell
# Search for all Go processes
Get-Process | Where-Object {$_.Name -like "*print*"}

# Check if running as different user
Get-Process -IncludeUserName | Where-Object {# Windows Browser Guard

A Windows registry monitoring tool that automatically detects and blocks unwanted browser extension installations via Group Policy.

## Features

- **Real-time Registry Monitoring**: Watches `HKLM\SOFTWARE\Policies` for changes recursively
- **Chrome Extension Protection**: Detects `ExtensionInstallForcelist` policies, extracts extension IDs, adds them to `ExtensionInstallBlocklist`, then removes the forcelist entry
- **Firefox Extension Protection**: Detects Firefox `ExtensionSettings` force install policies, creates blocklist entries with `installation_mode = "blocked"`, then removes the install policy
- **Detailed Change Tracking**: Shows exactly what changed (added/modified/removed keys and values)
- **Automatic Privilege Elevation**: Requests Administrator access if not running elevated

## Requirements

- Windows OS
- **Administrator privileges** (required for registry deletion)
- Go 1.16+ (for building from source)

## Quick Start

The easiest way to use Windows Browser Guard is with the included helper scripts:

```powershell
cd docs

# Install as scheduled task (runs at login)
.\install-task.ps1

# Or start manually
.\start-monitor.ps1

# View logs
.\view-logs.ps1

# Uninstall
.\uninstall-task.ps1
```

## Setup: Auto-Start at User Login

### Option 1: Use install-task.ps1 Script (Recommended)

The simplest method - just run the installer:

```powershell
cd docs
.\install-task.ps1
```

This creates a scheduled task with:
- Automatic start at user login
- Administrator privileges
- Output logging to WindowsBrowserGuard-log.txt
- Hidden background execution

### Option 2: Manual Task Scheduler Setup

Create a scheduled task manually:

```powershell
# Set the path to your executable (update this path to match your installation)
$exePath = "C:\Path\To\WindowsBrowserGuard.exe"
$logPath = "C:\Path\To\WindowsBrowserGuard-log.txt"
$taskName = "WindowsBrowserGuard"

# Create a wrapper script that redirects output to a log file
$wrapperScript = @"
Start-Process -FilePath '$exePath' -NoNewWindow -RedirectStandardOutput '$logPath' -RedirectStandardError '$logPath' -Wait
"@
$wrapperPath = "C:\Path\To\WindowsBrowserGuard-wrapper.ps1"
Set-Content -Path $wrapperPath -Value $wrapperScript

# Create the scheduled task
$action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-WindowStyle Hidden -ExecutionPolicy Bypass -File `"$wrapperPath`""
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0

Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Description "Monitors registry for unwanted extension installations"

Write-Host "✓ Scheduled task created: $taskName"
Write-Host "✓ Logs will be written to: $logPath"
Write-Host "The program will start automatically at next login with Administrator privileges"
```

**Viewing Task Scheduler Logs:**

When running as a scheduled task, program output is redirected to the log file specified above. To view logs:

```powershell
# View the log file in real-time
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 50 -Wait

# View recent logs
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 100

# Search for specific events
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" | Select-String "ExtensionInstallForcelist"
```

**Check Task Scheduler History:**

Task execution history can be viewed in Task Scheduler:

```powershell
# Open Task Scheduler GUI
taskschd.msc

# Or check task status via PowerShell
Get-ScheduledTaskInfo -TaskName "WindowsBrowserGuard"
```

In Task Scheduler GUI:
1. Open Task Scheduler (Win + R → `taskschd.msc`)
2. Navigate to "Task Scheduler Library"
3. Find "WindowsBrowserGuard"
4. Click the "History" tab to see execution events

**Verify the task:**
```powershell
Get-ScheduledTask -TaskName "WindowsBrowserGuard"
```

**Start the task manually (without waiting for login):**
```powershell
Start-ScheduledTask -TaskName "WindowsBrowserGuard"
```

**Remove the task:**
```powershell
Unregister-ScheduledTask -TaskName "WindowsBrowserGuard" -Confirm:$false
```

### Option 2: Registry Run Key (Simpler but requires UAC prompt)

Add to registry startup (will prompt for elevation at each login):

```powershell
$exePath = "C:\Path\To\WindowsBrowserGuard.exe"
$regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$valueName = "WindowsBrowserGuard"

Set-ItemProperty -Path $regPath -Name $valueName -Value $exePath
Write-Host "✓ Added to startup registry"
```

**Remove from startup:**
```powershell
Remove-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WindowsBrowserGuard"
```

### Option 3: Startup Folder (Least Recommended)

Create a shortcut in the Startup folder:

```powershell
$exePath = "C:\Path\To\WindowsBrowserGuard.exe"
$startupFolder = [Environment]::GetFolderPath('Startup')
$shortcutPath = Join-Path $startupFolder "WindowsBrowserGuard.lnk"

$WScriptShell = New-Object -ComObject WScript.Shell
$shortcut = $WScriptShell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $exePath
$shortcut.WorkingDirectory = Split-Path $exePath
$shortcut.Save()

Write-Host "✓ Shortcut created in Startup folder: $shortcutPath"
```

## Running in Background

### Start as Background Process

**Note**: These examples are for manual usage. Use `start-monitor.ps1` script for easier operation.

```powershell
# Start with PowerShell wrapper for output redirection
Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -Command `"& 'C:\Path\To\WindowsBrowserGuard.exe' *>&1 | Tee-Object -FilePath 'C:\Path\To\WindowsBrowserGuard-log.txt'`"" -WindowStyle Hidden
```

## Managing the Running Process

### Check if Running

```powershell
Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
```

### View Process Details

```powershell
Get-Process -Name "WindowsBrowserGuard" | Format-List *
```

### Stop the Process

```powershell
Stop-Process -Name "WindowsBrowserGuard" -Force
```

### Stop by Process ID (if multiple instances)

```powershell
# Find the process ID
Get-Process -Name "WindowsBrowserGuard"

# Stop specific instance
Stop-Process -Id <PID> -Force
```

## Viewing Logs

### Use view-logs.ps1 Script (Recommended)

The easiest way to view logs:

```powershell
cd docs
.\view-logs.ps1
```

### Manual Log Viewing

If you set up the program with Task Scheduler or used the scripts, logs are written to:

```powershell
# View log file in real-time
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 50 -Wait

# View last 100 lines
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" -Tail 100

# Search for specific events
Get-Content "C:\Path\To\WindowsBrowserGuard-log.txt" | Select-String "DETECTED"
```

### For Console/Interactive Runs

If running in a console window, output appears directly in the terminal. No log file is created unless you redirect output.

## How It Works

### Chrome Extension Monitoring

1. Detects when a value is added to a key containing `ExtensionInstallForcelist`
2. Extracts the extension ID (string before the first `;`)
3. Adds the extension ID to the corresponding `ExtensionInstallBlocklist` key
4. Deletes the entire `ExtensionInstallForcelist` key

**Example:**
- Detects: `HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist\1 = "extensionid;https://update.url"`
- Adds: `HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallBlocklist\1 = "extensionid"`
- Deletes: `HKLM\SOFTWARE\Policies\Google\Chrome\ExtensionInstallForcelist`

### Firefox Extension Monitoring

1. Detects when `installation_mode` is set to `force_installed` or `normal_installed` under `ExtensionSettings`
2. Extracts the extension ID from the registry path
3. Creates a blocklist entry with `installation_mode = "blocked"`
4. Deletes the original install policy key

**Example:**
- Detects: `Mozilla\Firefox\ExtensionSettings\{ext-id}\installation_mode = "force_installed"`
- Creates: `Mozilla\Firefox\ExtensionSettings\{ext-id}\installation_mode = "blocked"`
- Deletes: The original `{ext-id}` key

## Troubleshooting

### Program doesn't start at login
- Verify the scheduled task exists: `Get-ScheduledTask -TaskName "WindowsBrowserGuard"`
- Check task history in Task Scheduler (taskschd.msc)
- Ensure the executable path is correct

### "Insufficient privileges" errors
- The program must run as Administrator
- If using Task Scheduler, ensure "Run with highest privileges" is checked
- Manually run: Right-click executable → "Run as administrator"

### Registry keys not being deleted
- Verify Administrator privileges: The program will show status on startup
- Check Windows Event Viewer for access denied errors
- Ensure no other process has the registry key open

### Program crashes or stops
- Check the log file if output was redirected
- Run manually in a console to see error messages:
  ```powershell
  Start-Process -FilePath ".\WindowsBrowserGuard.exe" -Verb RunAs -Wait
  ```

### Can't find the process
```powershell
# Search for all Go processes
Get-Process | Where-Object {$_.Name -like "*Browser*"}

# Check if running as different user
Get-Process -IncludeUserName | Where-Object {$_.ProcessName -eq "WindowsBrowserGuard"}
```

## Building from Source

```powershell
go build -o WindowsBrowserGuard.exe ./cmd/WindowsBrowserGuard
```

## Security Notes

- This program requires Administrator privileges to delete registry keys
- It monitors the entire `HKLM\SOFTWARE\Policies` tree recursively
- Registry changes are permanent - the program will immediately delete detected extension install policies
- Blocklist entries persist even after the program stops

## Uninstall

1. Stop the running process:
   ```powershell
   Stop-Process -Name "WindowsBrowserGuard" -Force
   ```

2. Remove from startup (if using Task Scheduler):
   ```powershell
   Unregister-ScheduledTask -TaskName "WindowsBrowserGuard" -Confirm:$false
   ```

3. Delete the executable and log files

## License

This tool is provided as-is for registry monitoring and protection purposes.
.ProcessName -eq "WindowsBrowserGuard"}
```

## Building from Source

```powershell
go build -o WindowsBrowserGuard.exe ./cmd/WindowsBrowserGuard
```

## Security Notes

- This program requires Administrator privileges to delete registry keys
- It monitors the entire `HKLM\SOFTWARE\Policies` tree recursively
- Registry changes are permanent - the program will immediately delete detected extension install policies
- Blocklist entries persist even after the program stops

## Uninstall

1. Stop the running process:
   ```powershell
   Stop-Process -Name "WindowsBrowserGuard" -Force
   ```

2. Remove from startup (if using Task Scheduler):
   ```powershell
   Unregister-ScheduledTask -TaskName "WindowsBrowserGuard" -Confirm:$false
   ```

3. Delete the executable and log files

## License

This tool is provided as-is for registry monitoring and protection purposes.

