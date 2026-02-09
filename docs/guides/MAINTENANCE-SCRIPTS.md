# Maintenance Scripts Guide

## Overview

Windows Browser Guard includes easy-to-use PowerShell scripts for installation, maintenance, and monitoring. All scripts are located in the `docs/` directory.

## Installation Scripts

### install-task.ps1
**Purpose:** Install Windows Browser Guard as a scheduled task that starts automatically at login.

**Features:**
- Interactive OTLP endpoint configuration
- Creates scheduled task with highest privileges
- Generates wrapper script with configured settings
- Saves configuration to JSON file for maintenance scripts

**Usage:**
```powershell
.\docs\install-task.ps1
```

**Interactive prompts:**
1. Configure OTLP endpoint (optional)
   - Enter endpoint (e.g., `localhost:4317`)
   - Choose protocol: gRPC or HTTP
   - Enable/disable TLS
2. Confirm installation
3. Optionally start immediately

**Output files:**
- `WindowsBrowserGuard-config.json` - Configuration file
- `WindowsBrowserGuard-wrapper.ps1` - Startup wrapper script
- Creates scheduled task: "WindowsBrowserGuard"

### uninstall-task.ps1
**Purpose:** Remove the scheduled task and optionally clean up files.

**Features:**
- Removes scheduled task
- Stops running process (optional)
- Removes wrapper script (optional)
- Removes config file (optional)
- Removes log file (optional)

**Usage:**
```powershell
.\docs\uninstall-task.ps1
```

## Maintenance Scripts

### start.ps1
**Purpose:** Start the Windows Browser Guard monitor.

**Modes:**
1. **Task Scheduler mode (default)** - Starts via scheduled task
2. **Direct mode** - Starts process directly without task scheduler

**Usage:**
```powershell
# Start via task scheduler
.\docs\start.ps1

# Start directly (bypasses task scheduler)
.\docs\start.ps1 -Direct
```

**Features:**
- Loads configuration from `WindowsBrowserGuard-config.json`
- Applies OTLP settings automatically
- Checks for already-running processes
- Verifies successful start
- Can restart if already running

**When to use `-Direct`:**
- Testing without task scheduler
- Manual control over process lifecycle
- Running with different parameters temporarily

### stop.ps1
**Purpose:** Stop the Windows Browser Guard monitor.

**Usage:**
```powershell
.\docs\stop.ps1
```

**Features:**
- Stops running process (if found)
- Stops scheduled task (if running)
- Verifies successful stop
- Safe to run even if nothing is running

### restart.ps1
**Purpose:** Restart the Windows Browser Guard monitor.

**Usage:**
```powershell
# Restart via task scheduler
.\docs\restart.ps1

# Restart in direct mode
.\docs\restart.ps1 -Direct
```

**Features:**
- Stops monitor gracefully
- Waits 2 seconds for cleanup
- Starts monitor with same mode
- Verifies successful restart

**Use cases:**
- Apply configuration changes
- Recover from errors
- Clear stuck state

### status.ps1
**Purpose:** Display comprehensive status information.

**Usage:**
```powershell
.\docs\status.ps1
```

**Information displayed:**
- **Configuration:**
  - Config file location
  - OTLP endpoint settings
- **Process Status:**
  - Running/Not running
  - PID, CPU usage, memory usage
  - Start time and uptime
- **Scheduled Task:**
  - Task name and state
  - Last run time and result
  - Next scheduled run
- **Log File:**
  - Location and size
  - Last modified time
  - Last 5 log entries
- **Quick action commands**

### view-logs.ps1
**Purpose:** View and tail log files with filtering options.

**Usage:**
```powershell
# View last 50 lines
.\docs\view-logs.ps1

# Follow log in real-time
.\docs\view-logs.ps1 -Follow

# View only errors
.\docs\view-logs.ps1 -ErrorsOnly

# View full log
.\docs\view-logs.ps1 -Full
```

## Configuration File

**Location:** `WindowsBrowserGuard-config.json`

**Format:**
```json
{
  "OTLPEndpoint": "localhost:4317",
  "OTLPProtocol": "grpc",
  "OTLPInsecure": true,
  "ExePath": "C:\\Path\\To\\WindowsBrowserGuard.exe",
  "LogPath": "C:\\Path\\To\\WindowsBrowserGuard-log.txt",
  "TaskName": "WindowsBrowserGuard"
}
```

**Fields:**
- `OTLPEndpoint` - OTLP collector endpoint (empty if not configured)
- `OTLPProtocol` - Protocol: "grpc" or "http"
- `OTLPInsecure` - Disable TLS (true/false)
- `ExePath` - Full path to executable
- `LogPath` - Full path to log file
- `TaskName` - Scheduled task name

**Manual editing:**
You can edit this file to change OTLP settings. Run `.\restart.ps1` to apply changes.

## Common Workflows

### Initial Installation
```powershell
# 1. Install as scheduled task
.\docs\install-task.ps1

# 2. Configure OTLP when prompted (optional)

# 3. Start immediately (or wait for next login)

# 4. Check status
.\docs\status.ps1
```

### Daily Monitoring
```powershell
# Check if running
.\docs\status.ps1

# View recent activity
.\docs\view-logs.ps1

# Follow live logs
.\docs\view-logs.ps1 -Follow
```

### Configuration Changes
```powershell
# 1. Stop the monitor
.\docs\stop.ps1

# 2. Edit config file
notepad WindowsBrowserGuard-config.json

# 3. Start with new settings
.\docs\start.ps1
```

### Troubleshooting
```powershell
# Check comprehensive status
.\docs\status.ps1

# View error logs
.\docs\view-logs.ps1 -ErrorsOnly

# Restart to recover
.\docs\restart.ps1

# Check task scheduler for errors
Get-ScheduledTaskInfo -TaskName "WindowsBrowserGuard"
```

### Testing Changes
```powershell
# 1. Stop scheduled task
.\docs\stop.ps1

# 2. Run manually with test settings
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint=localhost:4317

# 3. When satisfied, start normally
.\docs\start.ps1
```

### Uninstallation
```powershell
# Complete removal
.\docs\uninstall-task.ps1

# Answer 'Y' to all prompts to remove everything
# Or 'N' to keep logs/config for later
```

## OTLP Configuration Examples

### Local Jaeger (All-in-One)
```
Endpoint: localhost:4317
Protocol: grpc
Insecure: Yes
```

### Local OTLP Collector
```
Endpoint: localhost:4318
Protocol: http
Insecure: Yes
```

### Production Grafana Cloud
```
Endpoint: otlp-gateway.grafana.net:443
Protocol: grpc
Insecure: No
Headers: Authorization=Bearer <token>
```

### Azure Monitor
```
Endpoint: <workspace>.in.applicationinsights.azure.com:443
Protocol: grpc
Insecure: No
```

### Datadog
```
Endpoint: api.datadoghq.com:443
Protocol: grpc
Insecure: No
Headers: DD-API-KEY=<key>
```

**Note:** Headers must be configured manually in the config JSON file.

## Script Dependencies

All scripts work independently except:
- `start.ps1`, `stop.ps1`, `restart.ps1` - Read `WindowsBrowserGuard-config.json` if it exists
- `status.ps1` - Reads config file to display settings
- All scripts require the executable to be in the same directory

## Permissions

**Administrator required:**
- `install-task.ps1` - Creates scheduled task
- `uninstall-task.ps1` - Removes scheduled task

**No admin required:**
- `start.ps1 -Direct` - Direct process start
- `stop.ps1` - Stops process by PID
- `restart.ps1` - Combination of stop/start
- `status.ps1` - Read-only status check
- `view-logs.ps1` - Read log files

## Best Practices

1. **Use Task Scheduler for production:**
   - Automatic start at login
   - Runs with highest privileges
   - Survives reboots

2. **Use Direct mode for testing:**
   - Quick iterations
   - Different parameters
   - Debugging

3. **Monitor regularly:**
   - Run `status.ps1` periodically
   - Check logs for errors
   - Verify OTLP connectivity

4. **Keep configuration backed up:**
   - Save `WindowsBrowserGuard-config.json`
   - Document OTLP endpoints
   - Note any custom headers

5. **Restart after config changes:**
   - Always restart to apply changes
   - Verify with `status.ps1`
   - Check logs for errors

## Troubleshooting Scripts

### Script won't run
```powershell
# Check execution policy
Get-ExecutionPolicy

# If Restricted, set to RemoteSigned
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Task won't start
```powershell
# Check task details
Get-ScheduledTask -TaskName "WindowsBrowserGuard" | Format-List *

# Check last error
Get-ScheduledTaskInfo -TaskName "WindowsBrowserGuard"

# View task history in Event Viewer
Get-WinEvent -LogName "Microsoft-Windows-TaskScheduler/Operational" -MaxEvents 20
```

### Process keeps stopping
```powershell
# Check if crashing
.\docs\view-logs.ps1 -ErrorsOnly

# Run in direct mode to see output
.\docs\start.ps1 -Direct

# Check with dry-run
.\WindowsBrowserGuard.exe --dry-run
```

### OTLP not connecting
```powershell
# Verify endpoint is reachable
Test-NetConnection -ComputerName localhost -Port 4317

# Check config
Get-Content WindowsBrowserGuard-config.json | ConvertFrom-Json

# Test with curl (HTTP endpoint)
curl http://localhost:4318/v1/traces

# Check logs for OTLP errors
.\docs\view-logs.ps1 | Select-String "OTLP"
```

## Quick Reference

| Task | Command |
|------|---------|
| Install | `.\docs\install-task.ps1` |
| Uninstall | `.\docs\uninstall-task.ps1` |
| Start | `.\docs\start.ps1` |
| Stop | `.\docs\stop.ps1` |
| Restart | `.\docs\restart.ps1` |
| Status | `.\docs\status.ps1` |
| View logs | `.\docs\view-logs.ps1` |
| Follow logs | `.\docs\view-logs.ps1 -Follow` |
| Direct start | `.\docs\start.ps1 -Direct` |
| Check errors | `.\docs\view-logs.ps1 -ErrorsOnly` |

---

*For detailed feature documentation, see: [docs/features/](features/)*  
*For command-line options, see: [README.md](README.md)*
