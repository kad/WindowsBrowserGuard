package main

import (
"flag"
"fmt"
"syscall"
"time"

"github.com/kad/WindowsBrowserGuard/pkg/admin"
"github.com/kad/WindowsBrowserGuard/pkg/monitor"
"github.com/kad/WindowsBrowserGuard/pkg/registry"
"golang.org/x/sys/windows"
)

var (
// Command line flags
dryRun = flag.Bool("dry-run", false, "Run in read-only mode without making changes")
)

var extensionIndex *registry.ExtensionPathIndex
var metrics registry.PerfMetrics

func main() {
flag.Parse()

hasAdmin := admin.CheckAdminAndElevate(*dryRun)
canWrite := hasAdmin && !*dryRun

keyPath := `SOFTWARE\Policies`

if canWrite {
canDelete := admin.CanDeleteRegistryKey(keyPath)
if !canDelete {
fmt.Println("⚠️  WARNING: Insufficient permissions to delete registry keys")
fmt.Println("Key deletion features will be disabled.")
canWrite = false
} else {
fmt.Println("✓ Registry deletion permissions verified")
}
}

key, err := syscall.UTF16PtrFromString(keyPath)
if err != nil {
fmt.Println("Error converting key path:", err)
return
}

var hKey windows.Handle
// In dry-run mode, only request read permissions
var permissions uint32 = windows.KEY_NOTIFY | windows.KEY_READ
if canWrite {
permissions |= windows.DELETE
}

err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, key, 0, permissions, &hKey)
if err != nil {
fmt.Println("Error opening registry key:", err)
return
}
defer windows.RegCloseKey(hKey)

fmt.Println("Capturing initial registry state...")
startTime := time.Now()
previousState, err := monitor.CaptureRegistryState(hKey, keyPath)
if err != nil {
fmt.Println("Error capturing initial state:", err)
return
}
scanDuration := time.Since(startTime)
metrics.StartupTime = scanDuration
metrics.InitialScanKeys = len(previousState.Subkeys) + len(previousState.Values)

fmt.Printf("Initial state: %d subkeys, %d values (captured in %v)\n",
len(previousState.Subkeys), len(previousState.Values), scanDuration)

fmt.Println("Building extension path index...")
indexStart := time.Now()
extensionIndex = registry.NewExtensionPathIndex()
extensionIndex.BuildFromState(previousState)
indexDuration := time.Since(indexStart)
metrics.IndexBuildTime = indexDuration

fmt.Printf("Index built: tracking %d unique extension IDs (in %v)\n",
extensionIndex.GetCount(), indexDuration)

monitor.ProcessExistingPolicies(keyPath, previousState, canWrite, extensionIndex)
monitor.CleanupAllowlists(keyPath, previousState, canWrite)
monitor.CleanupExtensionSettings(keyPath, previousState, canWrite, extensionIndex)

monitor.WatchRegistryChanges(hKey, keyPath, previousState, canWrite, extensionIndex)
}
