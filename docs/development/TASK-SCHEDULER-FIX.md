# Task Scheduler Installation Fix

## Issue
After installing WindowsBrowserGuard via `install-task.ps1`, the daemon did not start after system reboot.

## Root Cause Analysis

### Problem 1: Wrapper Script Blocking
The original wrapper script used:
```powershell
Start-Process -FilePath $exePath -NoNewWindow -RedirectStandardOutput $logPath -RedirectStandardError $logPath -Wait
```

**Issues:**
- The `-Wait` parameter causes the wrapper to wait for the process to exit
- Since WindowsBrowserGuard runs indefinitely, the wrapper never returns
- Task Scheduler thinks the task is still running and won't restart it
- The `-NoNewWindow` with redirection can cause output buffering issues

### Problem 2: Missing User Context on Trigger
The original trigger used:
```powershell
$trigger = New-ScheduledTaskTrigger -AtLogOn
```

**Issue:**
- Without specifying `-User`, the trigger may not fire correctly for the current user
- Task may wait for any user to log on rather than the specific user

### Problem 3: Missing StartWhenAvailable Setting
Original settings didn't include `-StartWhenAvailable` which means:
- If the system is busy at login time, the task may be skipped
- Task won't start if the trigger time was missed

## Solution

### Fixed Wrapper Script
```powershell
# Start the monitor process directly without waiting
# The process will run in the background and keep running
& $exePath *>&1 | Out-File -FilePath $logPath -Append
```

**Improvements:**
- Uses direct execution (`&`) instead of `Start-Process`
- No `-Wait` parameter - wrapper exits immediately after starting the process
- Redirects all output streams (`*>&1`) to log file with `-Append`
- Process continues running after wrapper exits
- Simpler and more reliable than complex Start-Process parameter combinations

### Fixed Trigger
```powershell
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
```

**Improvement:**
- Explicitly specifies the current user
- Ensures task runs when this specific user logs in

### Fixed Settings
```powershell
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit 0 -StartWhenAvailable
```

**Improvements:**
- Added `-StartWhenAvailable` to ensure task runs even if trigger was missed
- Task will start as soon as possible after login completes

## Testing Procedure

1. **Uninstall existing task:**
   ```powershell
   .\docs\uninstall-task.ps1
   ```

2. **Delete old wrapper and log:**
   ```powershell
   Remove-Item "C:\Program Files\KAD\WindowsBrowserGuard-wrapper.ps1" -Force -ErrorAction SilentlyContinue
   Remove-Item "C:\Program Files\KAD\WindowsBrowserGuard-log.txt" -Force -ErrorAction SilentlyContinue
   ```

3. **Install with fixed script:**
   ```powershell
   .\docs\install-task.ps1
   ```

4. **Test manual start:**
   ```powershell
   Start-ScheduledTask -TaskName "WindowsBrowserGuard"
   Start-Sleep -Seconds 3
   Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
   ```

5. **Verify log file is being written:**
   ```powershell
   Get-Content "C:\Program Files\KAD\WindowsBrowserGuard-log.txt" -Tail 20 -Wait
   ```

6. **Test after reboot:**
   - Reboot the system
   - Log in with the same user
   - Check if process started automatically:
     ```powershell
     Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue
     Get-Content "C:\Program Files\KAD\WindowsBrowserGuard-log.txt" -Tail 20
     ```

## Verification Commands

```powershell
# Check task configuration
Get-ScheduledTask -TaskName "WindowsBrowserGuard" | Format-List *

# Check task history
Get-ScheduledTaskInfo -TaskName "WindowsBrowserGuard"

# Check last run result (0 = success)
(Get-ScheduledTaskInfo -TaskName "WindowsBrowserGuard").LastTaskResult

# Check if process is running
Get-Process -Name "WindowsBrowserGuard" -ErrorAction SilentlyContinue

# View Task Scheduler event logs for errors
Get-WinEvent -LogName "Microsoft-Windows-TaskScheduler/Operational" -MaxEvents 20 | 
    Where-Object { $_.Message -like "*WindowsBrowserGuard*" } | 
    Format-List TimeCreated, Id, LevelDisplayName, Message
```

## Documentation Updates

### Files Modified
1. **docs/install-task.ps1** - Fixed wrapper script and task configuration
2. **docs/INDEX.md** - Created comprehensive documentation index

### Files Reorganized
Moved to **docs/features/**:
- OPENTELEMETRY.md
- OPENTELEMETRY-LOGGING.md
- OPENTELEMETRY-METRICS.md
- OTLP-ENDPOINTS.md
- DRY-RUN-MODE.md

Moved to **docs/development/**:
- MAIN-REFACTORING.md
- RESTRUCTURE.md
- CLEANUP-COMPLETE.md
- DEBUG-CLEANUP.md
- REFACTORING-COMPLETE.md
- LOGGING-INTEGRATION.md
- METRICS-INTEGRATION.md
- DOCS-SCRIPTS-UPDATE.md
- TEST-VERIFICATION.md
- detection-module.md
- GORELEASER.md

## Key Learnings

1. **PowerShell Start-Process Limitations:**
   - `-Wait` parameter blocks until process exits
   - Cannot be used for long-running background services
   - `-NoNewWindow` with redirection can cause issues

2. **Direct Execution Preferred for Services:**
   - Using `& $exePath` is simpler and more reliable
   - Wrapper exits immediately while process continues
   - Output redirection with `Out-File -Append` works correctly

3. **Task Scheduler Best Practices:**
   - Always specify `-User` on triggers for user-specific tasks
   - Use `-StartWhenAvailable` to handle missed triggers
   - Set `-ExecutionTimeLimit 0` for indefinite-running services
   - Verify tasks with event logs, not just task status

4. **Documentation Organization:**
   - Separate features from implementation history
   - Create index files for navigation
   - Group related documents together
   - Keep scripts and user guides easily accessible

---

*Created: 2026-02-09*
*Issue: Daemon not starting after reboot when installed via task scheduler*
*Fix: Corrected wrapper script and task configuration*
