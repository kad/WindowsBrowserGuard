# OTLP Endpoint Support

**Date**: 2026-01-29  
**Status**: ✅ COMPLETE

## Overview
Added support for exporting OpenTelemetry traces to OTLP (OpenTelemetry Protocol) HTTP and gRPC endpoints, enabling integration with observability platforms like Jaeger, Zipkin, Grafana Tempo, and cloud providers.

## New Command-Line Flags

### OTLP Configuration
```powershell
--otlp-endpoint <host:port>    # OTLP endpoint (e.g., localhost:4317)
--otlp-protocol <grpc|http>    # Protocol: 'grpc' or 'http' (default: grpc)
--otlp-insecure                # Disable TLS for OTLP connection
--otlp-headers <key=val,...>   # Custom headers (comma-separated)
```

### Existing Flags
```powershell
--trace-file <path|stdout>     # Local file or stdout output
--dry-run                      # Read-only mode
```

## Usage Examples

### 1. OTLP gRPC (Default)
```powershell
# Send to local collector (gRPC, insecure)
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4317 --otlp-insecure

# Send to remote collector with TLS
.\WindowsBrowserGuard.exe --otlp-endpoint collector.example.com:4317
```

### 2. OTLP HTTP
```powershell
# Send to local collector (HTTP, insecure)
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4318 --otlp-protocol http --otlp-insecure

# Send to cloud provider (HTTP with TLS)
.\WindowsBrowserGuard.exe --otlp-endpoint otlp.example.com:443 --otlp-protocol http
```

### 3. With Custom Headers
```powershell
# Add authentication headers
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint collector.example.com:4317 ^
  --otlp-headers "x-api-key=secret123,x-tenant=prod"
```

### 4. Local File Output (No OTLP)
```powershell
# Traditional file output
.\WindowsBrowserGuard.exe --dry-run --trace-file traces.json

# Stdout output
.\WindowsBrowserGuard.exe --dry-run --trace-file stdout
```

## Integration with Observability Platforms

### Jaeger
```powershell
# Start Jaeger all-in-one
docker run -d --name jaeger ^
  -e COLLECTOR_OTLP_ENABLED=true ^
  -p 16686:16686 ^
  -p 4317:4317 ^
  -p 4318:4318 ^
  jaegertracing/all-in-one:latest

# Send traces to Jaeger
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint localhost:4317 ^
  --otlp-insecure

# Open Jaeger UI: http://localhost:16686
```

### Grafana Tempo
```powershell
# Configure Tempo to accept OTLP
# In tempo.yaml:
# distributor:
#   receivers:
#     otlp:
#       protocols:
#         grpc:
#           endpoint: 0.0.0.0:4317

# Send traces to Tempo
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint localhost:4317 ^
  --otlp-insecure
```

### OpenTelemetry Collector
```yaml
# collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  jaeger:
    endpoint: jaeger:14250
  zipkin:
    endpoint: http://zipkin:9411/api/v2/spans

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger, zipkin]
```

```powershell
# Send to collector
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint localhost:4317 ^
  --otlp-insecure
```

### Honeycomb
```powershell
# Send traces to Honeycomb
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint api.honeycomb.io:443 ^
  --otlp-protocol http ^
  --otlp-headers "x-honeycomb-team=YOUR_API_KEY,x-honeycomb-dataset=windowsbrowserguard"
```

### New Relic
```powershell
# Send traces to New Relic
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint otlp.nr-data.net:4317 ^
  --otlp-headers "api-key=YOUR_LICENSE_KEY"
```

### Datadog
```powershell
# Send traces to Datadog Agent
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint localhost:4317 ^
  --otlp-insecure
```

## Implementation Details

### Dependencies Added
```
go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.39.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.39.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.39.0
google.golang.org/grpc v1.77.0
google.golang.org/protobuf v1.36.10
```

### pkg/telemetry/telemetry.go Changes

**New Config Struct**:
```go
type Config struct {
    TraceOutput  string            // File path or "stdout"
    OTLPEndpoint string            // OTLP endpoint (e.g., "localhost:4317")
    OTLPProtocol string            // "grpc" or "http"
    OTLPInsecure bool              // Disable TLS
    OTLPHeaders  map[string]string // Custom headers
}
```

**New Functions**:
- `createOTLPExporter(cfg Config)` - Creates appropriate OTLP exporter
- `createOTLPGRPCExporter(cfg Config)` - Creates gRPC OTLP exporter
- `createOTLPHTTPExporter(cfg Config)` - Creates HTTP OTLP exporter

**Updated Function**:
- `InitTracing(cfg Config)` - Now accepts Config struct instead of string

### cmd/WindowsBrowserGuard/main.go Changes

**New Flags**:
```go
otlpEndpoint = flag.String("otlp-endpoint", "", "...")
otlpProtocol = flag.String("otlp-protocol", "grpc", "...")
otlpInsecure = flag.Bool("otlp-insecure", false, "...")
otlpHeaders  = flag.String("otlp-headers", "", "...")
```

**Header Parsing**:
```go
// Parse comma-separated key=value pairs
headers := make(map[string]string)
if *otlpHeaders != "" {
    pairs := strings.Split(*otlpHeaders, ",")
    for _, pair := range pairs {
        kv := strings.SplitN(pair, "=", 2)
        if len(kv) == 2 {
            headers[kv[0]] = kv[1]
        }
    }
}
```

## Protocol Details

### OTLP gRPC
- **Default Port**: 4317
- **Transport**: HTTP/2
- **Performance**: More efficient for high-throughput
- **Use Case**: Best for production, local collectors

**Configuration**:
```powershell
--otlp-endpoint localhost:4317
--otlp-protocol grpc    # or omit (default)
```

### OTLP HTTP
- **Default Port**: 4318
- **Transport**: HTTP/1.1 or HTTP/2
- **Performance**: Slightly higher overhead
- **Use Case**: Better firewall compatibility, cloud providers

**Configuration**:
```powershell
--otlp-endpoint localhost:4318
--otlp-protocol http
```

## Security Considerations

### TLS/SSL
**Production** (with TLS):
```powershell
# Automatically uses TLS on standard ports
.\WindowsBrowserGuard.exe --otlp-endpoint secure-collector.example.com:4317
```

**Development** (without TLS):
```powershell
# Use --otlp-insecure for local testing
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

### Authentication
**API Key via Headers**:
```powershell
--otlp-headers "authorization=Bearer YOUR_TOKEN"
```

**Custom Headers**:
```powershell
--otlp-headers "x-api-key=secret,x-tenant-id=prod,x-environment=production"
```

## Trace Data Format

Traces sent to OTLP endpoints follow the OpenTelemetry standard:

**Trace Structure**:
- Service Name: `windowsbrowserguard`
- Service Version: `1.0.0`
- Spans: Hierarchical operation traces
- Attributes: Operation metadata
- Events: Timestamped log entries
- Errors: Exception details

**Example Span**:
```json
{
  "name": "monitor.CaptureRegistryState",
  "trace_id": "48b3051eb833eee103860b26506051d7",
  "span_id": "fb5adf595838cd1c",
  "parent_span_id": "2bec90685198b7e7",
  "kind": "INTERNAL",
  "start_time": "2026-01-29T23:52:23.699Z",
  "end_time": "2026-01-29T23:52:23.712Z",
  "attributes": {
    "key-path": "SOFTWARE\\Policies",
    "subkeys-count": 249,
    "values-count": 538
  },
  "resource": {
    "service.name": "windowsbrowserguard",
    "service.version": "1.0.0"
  }
}
```

## Testing

### Build Verification
```powershell
go build -o WindowsBrowserGuard.exe ./cmd/WindowsBrowserGuard
✓ Build successful!
```

### Flag Validation
```powershell
.\WindowsBrowserGuard.exe -h
```
Output shows all OTLP flags:
- `--otlp-endpoint`
- `--otlp-protocol`
- `--otlp-insecure`
- `--otlp-headers`

### Local Testing with Jaeger
```powershell
# 1. Start Jaeger
docker run -d --name jaeger ^
  -e COLLECTOR_OTLP_ENABLED=true ^
  -p 16686:16686 ^
  -p 4317:4317 ^
  jaegertracing/all-in-one:latest

# 2. Run application
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint localhost:4317 ^
  --otlp-insecure

# 3. View traces at http://localhost:16686
```

### Expected Console Output
```
📊 Tracing enabled: OTLP grpc://localhost:4317
🔍 DRY-RUN MODE: Running in read-only mode
   No changes will be made to the registry
   All write/delete operations will be simulated

Capturing initial registry state...
Initial state: 249 subkeys, 538 values (captured in 13.6ms)
...
```

## Troubleshooting

### Connection Issues
**Problem**: `failed to create OTLP exporter: connection refused`

**Solutions**:
1. Verify endpoint is running: `telnet localhost 4317`
2. Check firewall rules
3. Ensure correct port (4317 for gRPC, 4318 for HTTP)
4. Use `--otlp-insecure` for local testing

### TLS Errors
**Problem**: `certificate verify failed`

**Solutions**:
1. Use `--otlp-insecure` for testing
2. Install proper TLS certificates for production
3. Configure collector with valid certificates

### No Traces Appearing
**Problem**: Application runs but no traces in UI

**Solutions**:
1. Check application completes startup (wait 5-10 seconds)
2. Verify collector is configured to receive OTLP
3. Check collector logs for errors
4. Ensure traces are being exported (check application output)

## Performance Impact

### OTLP vs File/Stdout
- **OTLP**: Asynchronous batching, minimal overhead
- **File**: Synchronous writes, slight I/O overhead
- **Stdout**: Same as file but to console

### Recommendations
- **Development**: Use file or stdout for simple debugging
- **Testing**: Use OTLP with local collector
- **Production**: Use OTLP with remote collector or cloud provider

## Migration Guide

### From File Output
**Before**:
```powershell
.\WindowsBrowserGuard.exe --dry-run --trace-file traces.json
```

**After** (with OTLP):
```powershell
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4317 --otlp-insecure
```

### From Stdout Output
**Before**:
```powershell
.\WindowsBrowserGuard.exe --dry-run --trace-file stdout
```

**After** (with OTLP):
```powershell
# Set up local collector, then:
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4317 --otlp-insecure
```

## Benefits

### 1. **Centralized Observability**
- Send traces to centralized platform
- Correlate with metrics and logs
- Unified observability stack

### 2. **Production Monitoring**
- Real-time trace collection
- Automatic aggregation and indexing
- Query and analysis capabilities

### 3. **Vendor Flexibility**
- OpenTelemetry standard format
- Switch between platforms easily
- Avoid vendor lock-in

### 4. **Advanced Features**
- Distributed tracing across services
- Service dependency mapping
- Performance analytics and alerting

## Future Enhancements

### Potential Improvements
1. **Sampling**: Implement trace sampling for high-volume environments
2. **Metrics**: Add OTLP metrics export
3. **Logs**: Add OTLP logs export for unified telemetry
4. **Configuration File**: Support YAML config for complex setups
5. **Auto-discovery**: Detect local collectors automatically

## Backward Compatibility

✅ **100% Compatible**
- All existing flags work unchanged
- File and stdout output still supported
- OTLP is optional (opt-in via `--otlp-endpoint`)
- No breaking changes

## Example Deployment

### Production Setup
```powershell
# Production deployment with monitoring
.\WindowsBrowserGuard.exe ^
  --otlp-endpoint monitoring.corp.example.com:4317 ^
  --otlp-headers "x-api-key=prod-key-123,x-environment=production" ^
  >> C:\Logs\wbg-application.log 2>&1
```

### Development Setup
```powershell
# Local development with Jaeger
docker-compose up -d  # Start Jaeger
.\WindowsBrowserGuard.exe --dry-run ^
  --otlp-endpoint localhost:4317 ^
  --otlp-insecure
```

---

**Conclusion**: OTLP endpoint support successfully integrated, providing production-grade observability with support for gRPC and HTTP protocols, custom headers, and integration with all major observability platforms. The implementation maintains full backward compatibility while enabling enterprise monitoring capabilities.
