package main

import (
"context"
"flag"
"fmt"
"syscall"
"time"

"github.com/kad/WindowsBrowserGuard/pkg/admin"
"github.com/kad/WindowsBrowserGuard/pkg/monitor"
"github.com/kad/WindowsBrowserGuard/pkg/registry"
"github.com/kad/WindowsBrowserGuard/pkg/telemetry"
"go.opentelemetry.io/otel/attribute"
"golang.org/x/sys/windows"
)

var (
// Command line flags
dryRun    = flag.Bool("dry-run", false, "Run in read-only mode without making changes")
traceFile = flag.String("trace-file", "", "Output file for OpenTelemetry traces (use 'stdout' for console output)")
)

var extensionIndex *registry.ExtensionPathIndex
var metrics registry.PerfMetrics

func main() {
flag.Parse()

// Initialize tracing
ctx := context.Background()
shutdown, err := telemetry.InitTracing(*traceFile)
if err != nil {
fmt.Printf("Warning: Failed to initialize tracing: %v\n", err)
} else if *traceFile != "" {
fmt.Printf("📊 Tracing enabled: %s\n", *traceFile)
defer func() {
if err := shutdown(ctx); err != nil {
fmt.Printf("Warning: Failed to shutdown tracing: %v\n", err)
}
}()
}

// Start main application span
ctx, mainSpan := telemetry.StartSpan(ctx, "main.application",
attribute.Bool("dry-run", *dryRun),
)
defer mainSpan.End()

hasAdmin := admin.CheckAdminAndElevate(*dryRun)
canWrite := hasAdmin && !*dryRun

telemetry.SetAttributes(ctx,
attribute.Bool("has-admin", hasAdmin),
attribute.Bool("can-write", canWrite),
)

keyPath := `SOFTWARE\Policies`

if canWrite {
canDelete := admin.CanDeleteRegistryKey(keyPath)
if !canDelete {
fmt.Println("⚠️  WARNING: Insufficient permissions to delete registry keys")
fmt.Println("Key deletion features will be disabled.")
canWrite = false
telemetry.AddEvent(ctx, "insufficient-permissions")
} else {
fmt.Println("✓ Registry deletion permissions verified")
telemetry.AddEvent(ctx, "permissions-verified")
}
}

key, err := syscall.UTF16PtrFromString(keyPath)
if err != nil {
fmt.Println("Error converting key path:", err)
telemetry.RecordError(ctx, err)
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
telemetry.RecordError(ctx, err)
return
}
defer windows.RegCloseKey(hKey)

fmt.Println("Capturing initial registry state...")
startTime := time.Now()
previousState, err := monitor.CaptureRegistryState(ctx, hKey, keyPath)
if err != nil {
fmt.Println("Error capturing initial state:", err)
telemetry.RecordError(ctx, err)
return
}
scanDuration := time.Since(startTime)
metrics.StartupTime = scanDuration
metrics.InitialScanKeys = len(previousState.Subkeys) + len(previousState.Values)

telemetry.SetAttributes(ctx,
attribute.Int("initial.subkeys", len(previousState.Subkeys)),
attribute.Int("initial.values", len(previousState.Values)),
attribute.String("initial.scan-duration", scanDuration.String()),
)

fmt.Printf("Initial state: %d subkeys, %d values (captured in %v)\n",
len(previousState.Subkeys), len(previousState.Values), scanDuration)

fmt.Println("Building extension path index...")
indexStart := time.Now()
extensionIndex = registry.NewExtensionPathIndex()
extensionIndex.BuildFromState(previousState)
indexDuration := time.Since(indexStart)
metrics.IndexBuildTime = indexDuration

telemetry.SetAttributes(ctx,
attribute.Int("index.extension-count", extensionIndex.GetCount()),
attribute.String("index.build-duration", indexDuration.String()),
)

fmt.Printf("Index built: tracking %d unique extension IDs (in %v)\n",
extensionIndex.GetCount(), indexDuration)

monitor.ProcessExistingPolicies(ctx, keyPath, previousState, canWrite, extensionIndex)
monitor.CleanupAllowlists(ctx, keyPath, previousState, canWrite)
monitor.CleanupExtensionSettings(ctx, keyPath, previousState, canWrite, extensionIndex)

monitor.WatchRegistryChanges(ctx, hKey, keyPath, previousState, canWrite, extensionIndex)
}
