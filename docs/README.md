# Registry Extension Monitor

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

## Setup: Auto-Start at User Login

### Option 1: Task Scheduler (Recommended - Runs Elevated)

Create a scheduled task that runs at login with highest privileges:

```powershell
# Set the path to your executable (update this path to match your installation)
$exePath = "C:\Path\To\printwatch.exe"
$logPath = "C:\Path\To\printwatch-log.txt"
$taskName = "RegistryExtensionMonitor"

# Create a wrapper script that redirects output to a log file
$wrapperScript = @"
Start-Process -FilePath '$exePath' -NoNewWindow -RedirectStandardOutput '$logPath' -RedirectStandardError '$logPath' -Wait
"@
$wrapperPath = "C:\Path\To\printwatch-wrapper.ps1"
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
Get-Content "C:\Path\To\printwatch-log.txt" -Tail 50 -Wait

# View recent logs
Get-Content "C:\Path\To\printwatch-log.txt" -Tail 100

# Search for specific events
Get-Content "C:\Path\To\printwatch-log.txt" | Select-String "ExtensionInstallForcelist"
```

**Check Task Scheduler History:**

Task execution history can be viewed in Task Scheduler:

```powershell
# Open Task Scheduler GUI
taskschd.msc

# Or check task status via PowerShell
Get-ScheduledTaskInfo -TaskName "RegistryExtensionMonitor"
```

In Task Scheduler GUI:
1. Open Task Scheduler (Win + R → `taskschd.msc`)
2. Navigate to "Task Scheduler Library"
3. Find "RegistryExtensionMonitor"
4. Click the "History" tab to see execution events

**Verify the task:**
```powershell
Get-ScheduledTask -TaskName "RegistryExtensionMonitor"
```

**Start the task manually (without waiting for login):**
```powershell
Start-ScheduledTask -TaskName "RegistryExtensionMonitor"
```

**Remove the task:**
```powershell
Unregister-ScheduledTask -TaskName "RegistryExtensionMonitor" -Confirm:$false
```

### Option 2: Registry Run Key (Simpler but requires UAC prompt)

Add to registry startup (will prompt for elevation at each login):

```powershell
$exePath = "C:\Path\To\printwatch.exe"
$regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$valueName = "RegistryExtensionMonitor"

Set-ItemProperty -Path $regPath -Name $valueName -Value $exePath
Write-Host "✓ Added to startup registry"
```

**Remove from startup:**
```powershell
Remove-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "RegistryExtensionMonitor"
```

### Option 3: Startup Folder (Least Recommended)

Create a shortcut in the Startup folder:

```powershell
$exePath = "C:\Path\To\printwatch.exe"
$startupFolder = [Environment]::GetFolderPath('Startup')
$shortcutPath = Join-Path $startupFolder "RegistryExtensionMonitor.lnk"

$WScriptShell = New-Object -ComObject WScript.Shell
$shortcut = $WScriptShell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $exePath
$shortcut.WorkingDirectory = Split-Path $exePath
$shortcut.Save()

Write-Host "✓ Shortcut created in Startup folder: $shortcutPath"
```

## Running in Background

### Start as Background Process

```powershell
# Start the program in a hidden window
Start-Process -FilePath "C:\Path\To\printwatch.exe" -WindowStyle Hidden -Verb RunAs
```

### Start with Logging to File

```powershell
$exePath = "C:\Path\To\printwatch.exe"
$logPath = "C:\Path\To\printwatch-log.txt"

Start-Process -FilePath $exePath -WindowStyle Hidden -RedirectStandardOutput $logPath -Verb RunAs
```

## Managing the Running Process

### Check if Running

```powershell
Get-Process -Name "printwatch" -ErrorAction SilentlyContinue
```

### View Process Details

```powershell
Get-Process -Name "printwatch" | Format-List *
```

### Stop the Process

```powershell
Stop-Process -Name "printwatch" -Force
```

### Stop by Process ID (if multiple instances)

```powershell
# Find the process ID
Get-Process -Name "printwatch"

# Stop specific instance
Stop-Process -Id <PID> -Force
```

## Viewing Logs

### For Task Scheduler Setup

If you set up the program with Task Scheduler (Option 1), logs are written to the file specified during setup:

```powershell
# View log file in real-time (update path to match your log file location)
Get-Content "C:\Path\To\printwatch-log.txt" -Tail 50 -Wait

# View last 100 lines
Get-Content "C:\Path\To\printwatch-log.txt" -Tail 100

# Search for specific events
Get-Content "C:\Path\To\printwatch-log.txt" | Select-String "DETECTED"
```

### For Manual Runs with Output Redirection

If you started the program with output redirection:

```powershell
# View log file (update path to match your log file location)
Get-Content "C:\Path\To\printwatch-log.txt" -Tail 50 -Wait
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
- Verify the scheduled task exists: `Get-ScheduledTask -TaskName "RegistryExtensionMonitor"`
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
  Start-Process -FilePath ".\printwatch.exe" -Verb RunAs -Wait
  ```

### Can't find the process
```powershell
# Search for all Go processes
Get-Process | Where-Object {$_.Name -like "*print*"}

# Check if running as different user
Get-Process -IncludeUserName | Where-Object {$_.ProcessName -eq "printwatch"}
```

## Building from Source

```powershell
go build printwatch.go
```

## Security Notes

- This program requires Administrator privileges to delete registry keys
- It monitors the entire `HKLM\SOFTWARE\Policies` tree recursively
- Registry changes are permanent - the program will immediately delete detected extension install policies
- Blocklist entries persist even after the program stops

## Uninstall

1. Stop the running process:
   ```powershell
   Stop-Process -Name "printwatch" -Force
   ```

2. Remove from startup (if using Task Scheduler):
   ```powershell
   Unregister-ScheduledTask -TaskName "RegistryExtensionMonitor" -Confirm:$false
   ```

3. Delete the executable and log files

## License

This tool is provided as-is for registry monitoring and protection purposes.
