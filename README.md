# Windows Browser Guard - Project Structure

## Overview
Windows Browser Guard is a system monitoring tool that detects and blocks unauthorized browser extension installations via Windows Group Policy.

## Features

### Dry-Run Mode 🔍
Test the application safely without making any changes:
```powershell
.\WindowsBrowserGuard.exe --dry-run
```

**Dry-run mode provides:**
- ✅ Runs without Administrator privileges
- ✅ Detects all extension policies in real-time
- ✅ Watches for registry changes  
- ✅ Shows planned operations (without executing them)
- ✅ Perfect for testing and validation

### OpenTelemetry Tracing 📊
Monitor performance and debug issues with distributed tracing:

**Local file or stdout output:**
```powershell
# Trace to stdout
.\WindowsBrowserGuard.exe --dry-run --trace-file stdout

# Trace to file
.\WindowsBrowserGuard.exe --dry-run --trace-file traces.json
```

**OTLP endpoint (Jaeger, Tempo, cloud providers):**
```powershell
# OTLP gRPC (default)
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4317 --otlp-insecure

# OTLP HTTP
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4318 --otlp-protocol http --otlp-insecure

# With custom headers (authentication)
.\WindowsBrowserGuard.exe --otlp-endpoint collector.example.com:4317 --otlp-headers "x-api-key=secret"
```

**Tracing provides:**
- ✅ Detailed visibility into application behavior
- ✅ Performance monitoring for each operation
- ✅ Complete execution flow traces
- ✅ Error tracking and debugging
- ✅ OpenTelemetry standard format (JSON)
- ✅ Integration with Jaeger, Grafana Tempo, Honeycomb, New Relic, Datadog
- ✅ Both gRPC and HTTP protocols supported

### Production Mode
Run with full blocking capabilities:
```powershell
.\WindowsBrowserGuard.exe
```
Requires Administrator privileges to modify registry keys.

## Project Structure

```
WindowsBrowserGuard/
├── cmd/
│   └── WindowsBrowserGuard/
│       └── main.go                 # Main application entry point (145 lines)
├── pkg/
│   ├── admin/
│   │   └── admin.go                # Windows privilege management
│   ├── buffers/
│   │   └── buffers.go              # Memory buffer pools for performance
│   ├── detection/
│   │   └── detection.go            # Pure detection/parsing logic
│   ├── monitor/
│   │   └── monitor.go              # Registry monitoring and state management
│   ├── pathutils/
│   │   └── pathutils.go            # Path manipulation utilities
│   ├── registry/
│   │   └── registry.go             # Windows Registry operations
│   └── telemetry/
│       └── telemetry.go            # OpenTelemetry tracing support
├── docs/
├── go.mod                          # Go module definition
└── go.sum                          # Go dependencies
```

## Package Descriptions

### cmd/WindowsBrowserGuard
Main application package containing:
- Application startup and initialization
- Administrator privilege checks and elevation
- Extension policy processing logic
- Registry change monitoring loop
- User interface and logging

**Key Functions:**
- `main()` - Application entry point
- `checkAdminAndElevate()` - Verify/request admin privileges
- `processExistingPolicies()` - Handle existing extension policies
- `watchRegistryChanges()` - Monitor for registry changes

### pkg/buffers
Memory buffer pool management for efficient registry operations.
- Reusable buffers to reduce GC pressure
- Separate pools for different buffer sizes
- Automatic buffer clearing for security

**Exported Functions:**
- `GetNameBuffer()` / `PutNameBuffer()` - 256 uint16 buffers for subkey names
- `GetLargeNameBuffer()` / `PutLargeNameBuffer()` - 16384 uint16 buffers for value names  
- `GetDataBuffer()` / `PutDataBuffer()` - 16KB buffers for value data
- `GetLargeDataBuffer()` / `PutLargeDataBuffer()` - 64KB buffers for large values

### pkg/detection
Pure detection and parsing logic with no external dependencies.
- Browser extension policy detection (Chrome, Edge, Firefox)
- Extension ID extraction and validation
- Path analysis and transformations
- **No registry I/O - can be tested without Admin privileges**

**Exported Functions:**
- `ExtractExtensionIDFromValue()` - Parse extension IDs from values
- `IsChromeExtensionForcelist()` - Detect Chrome forcelist paths
- `IsEdgeExtensionForcelist()` - Detect Edge forcelist paths
- `IsFirefoxExtensionSettings()` - Detect Firefox extension paths
- `ValidateExtensionID()` - Validate extension ID format
- `GetBlocklistKeyPath()` - Convert forcelist → blocklist path
- `ParseForcelistValues()` - Extract all extension IDs from forcelist

### pkg/pathutils
String and path manipulation utilities optimized for registry paths.
- Case-insensitive path operations
- Efficient path building and parsing
- Component extraction and replacement

**Exported Functions:**
- `BuildPath()` - Construct registry paths from components
- `SplitPath()` - Parse paths into components
- `Contains()` / `ContainsIgnoreCase()` - Case-insensitive substring checks
- `ReplacePathComponent()` - Replace path components
- `GetParentPath()` / `GetKeyName()` - Path navigation

### pkg/registry
Windows Registry operations using Windows API.
- Recursive registry state capture
- Key and value enumeration
- Key deletion (single and recursive)
- Extension blocklist management

**Exported Types:**
- `RegState` - In-memory registry state (subkeys + values)
- `RegValue` - Registry value (name, type, data)
- `ExtensionPathIndex` - Fast extension lookup index
- `PerfMetrics` - Performance measurement data

**Exported Functions:**
- `CaptureKeyRecursive()` - Recursively capture registry state
- `ReadKeyValues()` - Read all values from a key
- `DeleteRegistryKey()` / `DeleteRegistryKeyRecursive()` - Key deletion
- `AddToBlocklist()` - Add extension to browser blocklist
- `RemoveFromAllowlist()` - Remove extension from allowlist
- `RemoveExtensionSettingsForID()` - Clean up extension settings

## Building

### Build Application
```powershell
# Build from project root
go build -o WindowsBrowserGuard.exe ./cmd/WindowsBrowserGuard

# Or use the build script
.\build.ps1
```

## Running

### Dry-Run Mode (No Admin Required)
Test the application without making changes:
```powershell
.\WindowsBrowserGuard.exe --dry-run
```

This mode:
- Scans and monitors the registry
- Detects extension policies
- Shows what would be blocked/removed
- Does NOT require Administrator privileges
- Safe to run in production for monitoring

### Production Mode (Admin Required)
Run with full capabilities:
```powershell
.\WindowsBrowserGuard.exe
```

This mode:
- Requires Administrator privileges
- Actually blocks and removes extension policies
- Modifies registry keys as needed

## Testing

### Test Dry-Run Mode
```powershell
# See what the application would do
.\WindowsBrowserGuard.exe --dry-run

# Output will show:
# [DRY-RUN] Would delete registry key: ...
# [DRY-RUN] Would add to blocklist: ...
```

### Test Detection Logic
All detection logic is in `pkg/detection/` and can be imported for testing without admin privileges.

## Development

### Adding New Detection Rules
1. Add detection logic to `pkg/detection/detection.go`
2. Add tests to `test_detection.go`
3. Test without Admin: `go run test_detection.go`
4. Integrate into main application

### Adding New Registry Operations
1. Add function to `pkg/registry/registry.go`
2. Export function (capitalize first letter)
3. Use in `cmd/WindowsBrowserGuard/main.go`

### Module Import Path
All packages use the module path: `github.com/kad/WindowsBrowserGuard`

Example imports:
```go
import (
    "github.com/kad/WindowsBrowserGuard/pkg/buffers"
    "github.com/kad/WindowsBrowserGuard/pkg/detection"
    "github.com/kad/WindowsBrowserGuard/pkg/pathutils"
    "github.com/kad/WindowsBrowserGuard/pkg/registry"
)
```

## Performance Optimizations

### Buffer Pooling (pkg/buffers)
- Reuses memory buffers across registry operations
- Reduces garbage collection overhead
- Separate pools for different buffer sizes

### Path Operations (pkg/pathutils)
- Pre-allocated string builders
- Case-insensitive comparisons optimized
- Component-based path parsing

### Registry Scanning (pkg/registry)
- Depth-limited recursive scanning
- Efficient state diffing
- Extension path indexing for O(1) lookups

## Architecture Benefits

### Modularity
- Clear separation of concerns
- Each package has single responsibility
- Easy to test individual components

### Testability
- Detection logic testable without privileges
- Mock-friendly interfaces
- Comprehensive test coverage

### Maintainability
- Standard Go project layout
- Well-documented packages
- Explicit dependencies

### Reusability
- Detection logic can be imported by other tools
- Registry operations are standalone
- Utilities are general-purpose

## OpenTelemetry Logging

In addition to tracing, WindowsBrowserGuard exports structured logs to OTLP backends:

```powershell
# Logs automatically exported when using OTLP endpoint
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

**Log Features:**
- ✅ Structured logging with contextual attributes
- ✅ Multiple log levels (Debug, Info, Warn, Error)
- ✅ Automatic trace-log correlation (includes trace and span IDs)
- ✅ Exported to same OTLP endpoint as traces
- ✅ Viewable in Jaeger, Grafana, cloud platforms

**Example Logs:**
- "Registry deletion permissions verified" (Info)
- "Insufficient permissions to delete registry keys" (Warn)
- "Failed to open registry key" (Error)

See **OPENTELEMETRY-LOGGING.md** for detailed documentation on logging setup, log-trace correlation, and backend integration.
