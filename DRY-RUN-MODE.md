# Dry-Run Mode Implementation - Summary

## Date: 2026-01-29

---

## Overview

Replaced separate test programs (`test_detection.exe`, `test_registry.exe`, `test_scan.exe`) with a unified **dry-run mode** in the main WindowsBrowserGuard application.

## What Changed

### Files Removed ✅
- ✗ `test_detection.go` (4.63 KB)
- ✗ `test_registry.go` (3.15 KB)
- ✗ `test_scan.go` (6.21 KB)
- ✗ `test_detection.exe`
- ✗ `test_registry.exe`
- ✗ `test_scan.exe`

**Total removed:** ~14 KB of test code + 3 executables

### New Feature Added ✅

#### Dry-Run Mode Flag
```powershell
.\WindowsBrowserGuard.exe --dry-run
```

**Benefits:**
- ✅ Runs WITHOUT Administrator privileges
- ✅ Detects all extension policies
- ✅ Monitors registry changes in real-time
- ✅ Shows what WOULD be done (without doing it)
- ✅ Safe to run in production environments
- ✅ No separate test programs needed

---

## Implementation Details

### 1. Command Line Flag
Added flag parsing in `cmd/WindowsBrowserGuard/main.go`:
```go
var dryRun = flag.Bool("dry-run", false, "Run in read-only mode without making changes")
```

### 2. Admin Check Updated
Modified `checkAdminAndElevate()` to skip elevation in dry-run mode:
```go
if *dryRun {
    fmt.Println("🔍 DRY-RUN MODE: Running in read-only mode")
    fmt.Println("   No changes will be made to the registry")
    return false
}
```

### 3. Registry Functions Updated
Added `dryRun bool` parameter to all write/delete operations:

**In `pkg/registry/registry.go`:**
- `DeleteRegistryKey(baseKeyPath, relativePath string, dryRun bool)`
- `DeleteRegistryKeyRecursive(baseKeyPath, relativePath string, dryRun bool)`
- `AddToBlocklist(baseKeyPath, blocklistPath, extensionID string, dryRun bool)`
- `RemoveFromAllowlist(baseKeyPath, allowlistPath, extensionID string, dryRun bool)`
- `BlockFirefoxExtension(baseKeyPath, extensionID string, dryRun bool)`
- `RemoveExtensionSettingsForID(keyPath, extensionID string, state *RegState, index *ExtensionPathIndex, dryRun bool)`

### 4. Dry-Run Output
When `dryRun = true`, functions print planned operations instead of executing them:
```
[DRY-RUN] Would delete registry key: HKLM\SOFTWARE\Policies\...
[DRY-RUN] Would add to blocklist: HKLM\SOFTWARE\Policies\...
[DRY-RUN]   Extension ID: afdpoidmelmfapkoikmenejmcdpgecfe
```

### 5. Main Loop Updated
Updated all function calls to pass the `canWrite` parameter:
```go
canWrite := hasAdmin && !*dryRun

// Pass canWrite to all processing functions
processExistingPolicies(keyPath, previousState, canWrite, extensionIndex)
watchRegistryChanges(hKey, keyPath, &previousState, canWrite, extensionIndex)
```

---

## Usage Examples

### Test Without Admin (Dry-Run)
```powershell
# Run as regular user
.\WindowsBrowserGuard.exe --dry-run
```

**Output:**
```
🔍 DRY-RUN MODE: Running in read-only mode
   No changes will be made to the registry
   All write/delete operations will be simulated

Capturing initial registry state...
Initial state: 244 subkeys, 349 values (captured in 9.5ms)

[EXISTING CHROME POLICY DETECTED]
Path: Google\Chrome\ExtensionInstallForcelist\1
Value: afdpoidmelmfapkoikmenejmcdpgecfe;https://...

[DRY-RUN] Would add to blocklist: Google\Chrome\ExtensionInstallBlocklist
[DRY-RUN]   Extension ID: afdpoidmelmfapkoikmenejmcdpgecfe
[DRY-RUN] Would delete registry key: HKLM\...\ExtensionInstallForcelist
```

### Production Mode (With Admin)
```powershell
# Run as Administrator
.\WindowsBrowserGuard.exe
```

**Output:**
```
✓ Running with Administrator privileges
✓ Registry deletion permissions verified

Capturing initial registry state...
Initial state: 244 subkeys, 349 values (captured in 9.5ms)

[EXISTING CHROME POLICY DETECTED]
📝 Adding to Chrome blocklist: Google\Chrome\ExtensionInstallBlocklist
✓ Successfully added to blocklist
🗑️  Deleting Chrome forcelist key...
✓ Successfully removed forcelist key
```

---

## Benefits

### Before (Separate Test Programs)
❌ Required 3 different test executables  
❌ Each test program had limited scope  
❌ Couldn't test full application flow  
❌ Had to build multiple programs  
❌ Users confused about which test to run  

### After (Unified Dry-Run Mode)
✅ Single executable for testing and production  
✅ Tests complete application flow  
✅ One flag to switch modes  
✅ Clearer user experience  
✅ Easier to maintain  

---

## Comparison

| Feature | Test Programs | Dry-Run Mode |
|---------|--------------|--------------|
| **Programs needed** | 3 separate exes | 1 executable |
| **Full app flow** | ❌ No | ✅ Yes |
| **Real-time monitoring** | ❌ No | ✅ Yes |
| **Production testing** | ❌ Limited | ✅ Complete |
| **User experience** | Complex | Simple flag |
| **Maintenance** | 3 codebases | 1 codebase |

---

## Updated Build Process

### Build Script (`build.ps1`)
Now builds only the main application:
```powershell
.\build.ps1
```

Output:
```
Building WindowsBrowserGuard...
  ✓ WindowsBrowserGuard.exe

Usage:
  .\WindowsBrowserGuard.exe           (requires admin - makes changes)
  .\WindowsBrowserGuard.exe --dry-run (no admin - read-only mode)
```

---

## Documentation Updates

Updated files:
- ✅ `README.md` - Added dry-run mode section
- ✅ `build.ps1` - Simplified to single executable
- ✅ `DRY-RUN-MODE.md` - This file (implementation details)

---

## Testing

### Manual Testing
```powershell
# Test dry-run mode (no admin)
.\WindowsBrowserGuard.exe --dry-run

# Test production mode (with admin)
.\WindowsBrowserGuard.exe
```

### Expected Behavior

**Dry-Run Mode:**
- ✅ Runs without admin prompts
- ✅ Scans registry successfully
- ✅ Detects extension policies
- ✅ Prints "[DRY-RUN] Would..." messages
- ✅ Does NOT modify registry
- ✅ Monitors changes in real-time

**Production Mode:**
- ✅ Requires admin elevation
- ✅ Scans registry successfully
- ✅ Detects extension policies
- ✅ Actually modifies registry
- ✅ Blocks and removes extensions
- ✅ Monitors and reacts to changes

---

## Code Quality

### Clean Architecture
- Clear separation between detection and execution
- `dryRun` parameter flows through all write operations
- No code duplication
- Easy to test

### Maintainability
- Single codebase for testing and production
- Consistent behavior between modes
- Easy to add new operations
- Well-documented

---

## Summary

✅ **Removed 3 test programs** (~14 KB + 3 executables)  
✅ **Added unified dry-run mode** via `--dry-run` flag  
✅ **Updated all registry operations** to support dry-run  
✅ **Simplified build process** to single executable  
✅ **Updated documentation** with usage examples  
✅ **Improved user experience** with clearer testing workflow  

**Status: Complete and tested! 🎉**

---

## Quick Reference

### Commands
```powershell
# Build
.\build.ps1

# Test (no admin)
.\WindowsBrowserGuard.exe --dry-run

# Production (admin)
.\WindowsBrowserGuard.exe
```

### Flag Options
- `--dry-run` - Run in read-only mode without making changes
- (no flags) - Run in production mode (requires admin)
