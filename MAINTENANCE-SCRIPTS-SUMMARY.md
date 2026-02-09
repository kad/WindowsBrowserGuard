# Maintenance Scripts and OTLP Configuration - Summary

## Changes Overview

Added comprehensive maintenance scripts and OTLP endpoint configuration capability to Windows Browser Guard.

## ✅ What Was Added

### 4 New Maintenance Scripts

1. **start.ps1** - Start the monitor
   - Task scheduler mode (default)
   - Direct mode (`-Direct` flag)
   - Auto-loads configuration
   - Checks for conflicts
   - Verifies successful start

2. **stop.ps1** - Stop the monitor
   - Stops process by PID
   - Stops scheduled task
   - Verifies successful stop
   - Safe to run anytime

3. **restart.ps1** - Restart the monitor
   - Graceful stop
   - 2-second delay
   - Restart in same mode
   - Verifies successful restart

4. **status.ps1** - Show comprehensive status
   - Configuration details
   - Process status (PID, CPU, memory, uptime)
   - Task scheduler state
   - Log file info
   - Last 5 log entries

### Enhanced Installation Scripts

1. **install-task.ps1**
   - Interactive OTLP configuration
   - Endpoint, protocol, TLS settings
   - Saves to `WindowsBrowserGuard-config.json`
   - Passes settings to wrapper script

2. **uninstall-task.ps1**
   - Removes configuration file (optional)
   - Complete cleanup workflow

### Configuration System

**File:** `WindowsBrowserGuard-config.json`

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

**Benefits:**
- Single source of truth
- Shared by all scripts
- Easy to backup/restore
- Manual editing supported

### Documentation

1. **docs/guides/MAINTENANCE-SCRIPTS.md** (420 lines)
   - Complete usage guide
   - All scripts explained
   - Common workflows
   - OTLP configuration examples
   - Troubleshooting
   - Quick reference

2. **docs/development/MAINTENANCE-SCRIPTS-IMPLEMENTATION.md** (359 lines)
   - Technical implementation details
   - Code examples
   - Testing checklist
   - Migration notes

3. **docs/INDEX.md** (updated)
   - New scripts listed
   - Updated reading order
   - Link to maintenance guide

## 📋 Quick Reference

```powershell
# Installation
.\docs\install-task.ps1          # Configure OTLP interactively

# Daily operations
.\docs\start.ps1                 # Start monitor
.\docs\stop.ps1                  # Stop monitor
.\docs\restart.ps1               # Restart monitor
.\docs\status.ps1                # Show status
.\docs\view-logs.ps1             # View logs

# Testing
.\docs\start.ps1 -Direct         # Start without task scheduler
.\docs\restart.ps1 -Direct       # Restart in direct mode

# Uninstall
.\docs\uninstall-task.ps1        # Complete removal
```

## 🎯 Use Cases

### Production Installation
```powershell
# Install with OTLP configuration
.\docs\install-task.ps1

# Configure when prompted:
# - Endpoint: otlp-collector.company.com:4317
# - Protocol: gRPC
# - Insecure: No

# Monitor status
.\docs\status.ps1
```

### Local Development
```powershell
# Install with local OTLP
.\docs\install-task.ps1

# Configure:
# - Endpoint: localhost:4318
# - Protocol: HTTP
# - Insecure: Yes

# Test directly
.\docs\start.ps1 -Direct

# Check logs
.\docs\view-logs.ps1 -Follow
```

### Configuration Changes
```powershell
# Stop monitor
.\docs\stop.ps1

# Edit configuration
notepad WindowsBrowserGuard-config.json

# Apply changes
.\docs\start.ps1
```

### Troubleshooting
```powershell
# Get full status
.\docs\status.ps1

# Check for errors
.\docs\view-logs.ps1 -ErrorsOnly

# Restart to recover
.\docs\restart.ps1
```

## 🔧 Technical Details

### OTLP Configuration Flow

1. **Installation:**
   ```
   install-task.ps1 → User prompts → Save to config.json
                   → Build command args → Wrapper script
   ```

2. **Startup:**
   ```
   start.ps1 → Load config.json → Build command args
            → Start process with OTLP flags
   ```

3. **Application:**
   ```
   WindowsBrowserGuard.exe --otlp-endpoint=<endpoint>
                          --otlp-protocol=<protocol>
                          --otlp-insecure
   ```

### Command Arguments Built

From configuration:
- `--otlp-endpoint="localhost:4317"`
- `--otlp-protocol=grpc`
- `--otlp-insecure` (if enabled)

Passed to wrapper script, then to executable.

### Configuration Loading

All scripts use this pattern:
```powershell
if (Test-Path $configPath) {
    $config = Get-Content $configPath | ConvertFrom-Json
    $taskName = $config.TaskName
    $otlpEndpoint = $config.OTLPEndpoint
    # ... use settings
}
```

## 📊 Git Commit

**Commit:** `d47899e`  
**Message:** feat: Add maintenance scripts and OTLP configuration

**Statistics:**
- 9 files changed
- 1,193 insertions (+)
- 8 deletions (-)

**Files:**
- 4 new scripts: start.ps1, stop.ps1, restart.ps1, status.ps1
- 2 new docs: MAINTENANCE-SCRIPTS.md, MAINTENANCE-SCRIPTS-IMPLEMENTATION.md
- 3 modified: install-task.ps1, uninstall-task.ps1, INDEX.md

## ✨ Benefits

### For Users
- **Simple:** Single-command operations
- **Consistent:** Same interface for all tasks
- **Clear:** Status shows everything at a glance
- **Flexible:** Task scheduler or direct mode

### For Operations
- **Observable:** OTLP configured at install
- **Maintainable:** Configuration in one place
- **Reliable:** Verifies all operations
- **Recoverable:** Easy restart and troubleshooting

### For Development
- **Testable:** Direct mode for quick testing
- **Configurable:** Easy to change OTLP endpoints
- **Documented:** Complete guides and examples
- **Extensible:** Easy to add more scripts

## 🚀 Next Steps

Users should:
1. Reinstall with `.\docs\install-task.ps1` to get OTLP configuration
2. Learn the new maintenance scripts
3. Set up OTLP collector/backend
4. Use `.\docs\status.ps1` for monitoring

Optional enhancements:
- Add `configure.ps1` to change settings without reinstall
- Add `validate.ps1` to test OTLP connectivity
- Add `health.ps1` for continuous monitoring
- Add PowerShell module for easier distribution

## 📚 Documentation

**Main Guide:**  
[docs/guides/MAINTENANCE-SCRIPTS.md](docs/guides/MAINTENANCE-SCRIPTS.md)

**Implementation Details:**  
[docs/development/MAINTENANCE-SCRIPTS-IMPLEMENTATION.md](docs/development/MAINTENANCE-SCRIPTS-IMPLEMENTATION.md)

**Navigation:**  
[docs/INDEX.md](docs/INDEX.md)

---

**Date:** 2026-02-09  
**Status:** Complete, tested, documented, and committed  
**Compatibility:** Windows 10/11, PowerShell 5.1+
