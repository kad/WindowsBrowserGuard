package main

import (
	"context"
	"flag"
	"strings"
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

	// OTLP flags
	otlpEndpoint = flag.String("otlp-endpoint", "", "OTLP endpoint (e.g., localhost:4317 or localhost:4318)")
	otlpProtocol = flag.String("otlp-protocol", "grpc", "OTLP protocol: 'grpc' or 'http' (default: grpc)")
	otlpInsecure = flag.Bool("otlp-insecure", false, "Disable TLS for OTLP connection")
	otlpHeaders  = flag.String("otlp-headers", "", "OTLP headers as comma-separated key=value pairs (e.g., 'key1=val1,key2=val2')")
)

var extensionIndex *registry.ExtensionPathIndex
var metrics registry.PerfMetrics

func main() {
	flag.Parse()

	// Parse OTLP headers
	headers := make(map[string]string)
	if *otlpHeaders != "" {
		pairs := strings.Split(*otlpHeaders, ",")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	// Initialize tracing
	ctx := context.Background()
	cfg := telemetry.Config{
		TraceOutput:  *traceFile,
		OTLPEndpoint: *otlpEndpoint,
		OTLPProtocol: *otlpProtocol,
		OTLPInsecure: *otlpInsecure,
		OTLPHeaders:  headers,
	}

	shutdown, err := telemetry.InitTracing(cfg)
	if err != nil {
		telemetry.Printf(ctx, "Warning: Failed to initialize tracing: %v\n", err)
	} else if *traceFile != "" || *otlpEndpoint != "" {
		if *otlpEndpoint != "" {
			telemetry.Printf(ctx, "📊 Tracing enabled: OTLP %s://%s\n", *otlpProtocol, *otlpEndpoint)
		} else {
			telemetry.Printf(ctx, "📊 Tracing enabled: %s\n", *traceFile)
		}
		defer func() {
			if err := shutdown(ctx); err != nil {
				telemetry.Printf(ctx, "Warning: Failed to shutdown tracing: %v\n", err)
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
			telemetry.Println(ctx, "⚠️  WARNING: Insufficient permissions to delete registry keys")
			telemetry.Println(ctx, "Key deletion features will be disabled.")
			canWrite = false
			telemetry.AddEvent(ctx, "insufficient-permissions")
		} else {
			telemetry.Println(ctx, "✓ Registry deletion permissions verified")
			telemetry.AddEvent(ctx, "permissions-verified")
		}
	}

	key, err := syscall.UTF16PtrFromString(keyPath)
	if err != nil {
		telemetry.Println(ctx, "Error converting key path:", err)
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
		telemetry.Println(ctx, "Error opening registry key:", err)
		telemetry.RecordError(ctx, err)
		return
	}
	defer windows.RegCloseKey(hKey)

	telemetry.Println(ctx, "Capturing initial registry state...")
	startTime := time.Now()
	previousState, err := monitor.CaptureRegistryState(ctx, hKey, keyPath)
	if err != nil {
		telemetry.Println(ctx, "Error capturing initial state:", err)
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

	telemetry.Printf(ctx, "Initial state: %d subkeys, %d values (captured in %v)\n",
		len(previousState.Subkeys), len(previousState.Values), scanDuration)

	telemetry.Println(ctx, "Building extension path index...")
	indexStart := time.Now()
	extensionIndex = registry.NewExtensionPathIndex()
	extensionIndex.BuildFromState(previousState)
	indexDuration := time.Since(indexStart)
	metrics.IndexBuildTime = indexDuration

	telemetry.SetAttributes(ctx,
		attribute.Int("index.extension-count", extensionIndex.GetCount()),
		attribute.String("index.build-duration", indexDuration.String()),
	)

	telemetry.Printf(ctx, "Index built: tracking %d unique extension IDs (in %v)\n",
		extensionIndex.GetCount(), indexDuration)

	monitor.ProcessExistingPolicies(ctx, keyPath, previousState, canWrite, extensionIndex)
	monitor.CleanupAllowlists(ctx, keyPath, previousState, canWrite)
	monitor.CleanupExtensionSettings(ctx, keyPath, previousState, canWrite, extensionIndex)

	monitor.WatchRegistryChanges(ctx, hKey, keyPath, previousState, canWrite, extensionIndex)
}
