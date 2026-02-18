package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/kad/WindowsBrowserGuard/pkg/admin"
	"github.com/kad/WindowsBrowserGuard/pkg/monitor"
	"github.com/kad/WindowsBrowserGuard/pkg/registry"
	"github.com/kad/WindowsBrowserGuard/pkg/telemetry"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sys/windows"
)

var extensionIndex *registry.ExtensionPathIndex
var metrics registry.PerfMetrics

func main() {
	var (
		dryRun      bool
		quiet       bool
		logFilePath string
		traceFile   string
		otlpURL     string
		otlpHeaders string
	)

	rootCmd := &cobra.Command{
		Use:          "WindowsBrowserGuard",
		Short:        "Monitor and block forced browser extension policies via Windows Registry",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApp(dryRun, quiet, logFilePath, traceFile, otlpURL, otlpHeaders)
		},
	}

	f := rootCmd.Flags()
	f.BoolVar(&dryRun, "dry-run", false, "Read-only mode: detect and log planned operations without making changes")
	f.BoolVar(&quiet, "quiet", false, "Suppress stdout logging (send logs to OTLP pipeline only)")
	f.StringVar(&logFilePath, "log-file", "", "Path to log file; output is appended (always active, independent of --quiet)")
	f.StringVar(&traceFile, "trace-file", "", "Output file for OpenTelemetry traces (use 'stdout' for console)")
	f.StringVar(&otlpURL, "otlp-endpoint", "",
		"OTLP endpoint URL — scheme sets protocol and TLS:\n"+
			"  grpc://host[:4317]   gRPC, no TLS\n"+
			"  grpcs://host[:443]   gRPC, TLS\n"+
			"  http://host[:4318]   HTTP, no TLS\n"+
			"  https://host[:443]   HTTP, TLS")
	f.StringVar(&otlpHeaders, "otlp-headers", "",
		"OTLP headers as comma-separated key=value pairs (e.g. 'Authorization=Bearer token')")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func parseHeaders(raw string) map[string]string {
	headers := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}

func runApp(dryRun, quiet bool, logFilePath, traceFile, rawOTLPEndpoint, otlpHeaders string) error {
	// Apply stdout suppression before any logging
	if quiet {
		telemetry.SetSuppressStdout(true)
	}
	// Open log file if specified — always active regardless of --quiet or OTLP
	if logFilePath != "" {
		if err := telemetry.SetLogFile(logFilePath); err != nil {
			return err
		}
	}
	// Parse OTLP endpoint URL → host:port, protocol, TLS setting
	otlpHost, otlpProtocol, otlpInsecure, err := telemetry.ParseOTLPEndpoint(rawOTLPEndpoint)
	if err != nil {
		return fmt.Errorf("--otlp-endpoint: %w", err)
	}

	ctx := context.Background()
	cfg := telemetry.Config{
		TraceOutput:  traceFile,
		OTLPEndpoint: otlpHost,
		OTLPProtocol: otlpProtocol,
		OTLPInsecure: otlpInsecure,
		OTLPHeaders:  parseHeaders(otlpHeaders),
	}

	shutdown, err := telemetry.InitTracing(cfg)
	if err != nil {
		telemetry.Printf(ctx, "Warning: Failed to initialize tracing: %v\n", err)
	} else if traceFile != "" || otlpHost != "" {
		if otlpHost != "" {
			var scheme string
			switch {
			case otlpProtocol == "grpc" && otlpInsecure:
				scheme = "grpc"
			case otlpProtocol == "grpc":
				scheme = "grpcs"
			case otlpInsecure:
				scheme = "http"
			default:
				scheme = "https"
			}
			telemetry.Printf(ctx, "📊 Telemetry enabled: %s://%s\n", scheme, otlpHost)
		} else {
			telemetry.Printf(ctx, "📊 Tracing enabled: %s\n", traceFile)
		}
		defer func() {
			if err := shutdown(ctx); err != nil {
				fmt.Printf("Warning: Failed to shutdown tracing: %v\n", err)
			}
		}()
	}

	// Start main application span
	ctx, mainSpan := telemetry.StartSpan(ctx, "main.application",
		attribute.Bool("dry-run", dryRun),
	)
	defer mainSpan.End()

	hasAdmin := admin.CheckAdminAndElevate(dryRun)
	canWrite := hasAdmin && !dryRun

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
		return err
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
		return err
	}
	defer windows.RegCloseKey(hKey)

	telemetry.Println(ctx, "Capturing initial registry state...")
	startTime := time.Now()
	previousState, err := monitor.CaptureRegistryState(ctx, hKey, keyPath)
	if err != nil {
		telemetry.Println(ctx, "Error capturing initial state:", err)
		telemetry.RecordError(ctx, err)
		return err
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
	return nil
}
