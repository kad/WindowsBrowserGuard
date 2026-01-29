# OpenTelemetry Logging Integration

## Overview

WindowsBrowserGuard now supports OpenTelemetry logging, allowing operational logs to be exported to OTLP-compatible backends alongside distributed traces. This enables unified observability with correlated traces and logs.

## Features

- **Structured Logging**: All logs include contextual attributes (key paths, operations, etc.)
- **Log Levels**: Debug, Info, Warn, Error
- **Trace Correlation**: Logs automatically include trace and span IDs when available
- **OTLP Export**: Logs exported to the same OTLP endpoint as traces
- **Protocol Support**: Both gRPC (default) and HTTP protocols
- **TLS Support**: Secure transport enabled by default

## Configuration

Logging is automatically enabled when you specify an OTLP endpoint. The same endpoint configuration applies to both traces and logs.

### Command-Line Flags

```powershell
# Enable logging via OTLP gRPC (default protocol)
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure

# Enable logging via OTLP HTTP
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4318 --otlp-protocol http --otlp-insecure

# With authentication headers
.\WindowsBrowserGuard.exe --otlp-endpoint api.example.com:4317 --otlp-headers "x-api-key=secret,x-tenant=prod"

# With tracing to file and logging to OTLP
.\WindowsBrowserGuard.exe --trace-file traces.json --otlp-endpoint localhost:4317 --otlp-insecure
```

## Integration with Observability Platforms

### Jaeger

Jaeger supports OTLP and can ingest both traces and logs:

```powershell
# Start Jaeger with OTLP support
docker run -d --name jaeger \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest

# Run WindowsBrowserGuard with logging
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

View logs in Jaeger UI at http://localhost:16686

### Grafana Tempo + Loki

Combine Tempo for traces and Loki for logs:

```yaml
# docker-compose.yml
services:
  tempo:
    image: grafana/tempo:latest
    ports:
      - "4317:4317"  # OTLP gRPC
  
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
  
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
```

```powershell
# Send both traces and logs
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

### AWS CloudWatch

Use AWS Distro for OpenTelemetry (ADOT):

```powershell
# Configure ADOT Collector endpoint
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

### Azure Monitor

Use Azure Monitor OpenTelemetry Exporter:

```powershell
# Send to Azure Monitor via OTLP
.\WindowsBrowserGuard.exe \
  --otlp-endpoint <ingestion-endpoint> \
  --otlp-headers "x-instrumentation-key=<your-key>"
```

## Log Levels

### Info (Default)
Application lifecycle events and successful operations:
- "Registry deletion permissions verified"
- "Capturing initial registry state"
- "Processing existing extension policies"

### Warn
Non-critical issues that don't prevent operation:
- "Insufficient permissions to delete registry keys"
- "Not running as admin - some operations may fail"

### Error
Critical failures that prevent operations:
- "Failed to open registry key"
- "Error capturing registry state"
- "Error converting key path"

### Debug
Detailed diagnostic information (currently not emitted, reserved for future use)

## Log Attributes

All logs include contextual attributes:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `key_path` | Registry key path | `SOFTWARE\Policies` |
| `operation` | Operation being performed | `capture`, `delete`, `watch` |
| `dry_run` | Whether in dry-run mode | `true` or `false` |
| `is_admin` | Administrator status | `true` or `false` |
| `error` | Error message (error logs only) | "Access denied" |
| `permissions` | Registry access permissions | `131097` |

## Trace-Log Correlation

When logging is enabled with an OTLP endpoint, all logs automatically include:

- **Trace ID**: Links log to its distributed trace
- **Span ID**: Links log to specific span in trace
- **Trace Flags**: Sampling decisions

This enables:
1. **Logs-to-Traces Navigation**: Click log entry → view full trace
2. **Traces-to-Logs Navigation**: View trace → see all related logs
3. **Unified Timeline**: See logs and spans in chronological order

## Example: Viewing Correlated Traces and Logs

1. **Start Jaeger**:
   ```powershell
   docker run -d -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one:latest
   ```

2. **Run WindowsBrowserGuard**:
   ```powershell
   .\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4317 --otlp-insecure
   ```

3. **Open Jaeger UI**: http://localhost:16686

4. **Find your trace**:
   - Service: `windowsbrowserguard`
   - Operation: `main.application`

5. **View logs**:
   - Click trace → see all spans
   - Click span → view logs for that operation
   - Logs appear alongside span events

## Architecture

### Telemetry Package (`pkg/telemetry/telemetry.go`)

```go
// Initialize logging and tracing
shutdown, err := telemetry.InitTracing(telemetry.Config{
    OTLPEndpoint:  "localhost:4317",
    OTLPProtocol:  "grpc",
    OTLPInsecure:  true,
})

// Emit structured logs
telemetry.LogInfo(ctx, "Operation successful",
    attribute.String("key", "value"))

telemetry.LogError(ctx, "Operation failed", err,
    attribute.String("key", "value"))
```

### Log Exporters

- **gRPC**: `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`
- **HTTP**: `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp`

Logs are batched before export for efficiency.

### Global Logger

The telemetry package sets up a global logger provider:
- **Logger**: `windowsbrowserguard`
- **Processor**: BatchProcessor (batches logs before export)
- **Resource**: Service name, version, schema URL

## Performance Considerations

- **Batching**: Logs are batched before export (default: 512 logs or 5 seconds)
- **Async Export**: Log export happens asynchronously
- **Minimal Overhead**: Structured logging is efficient (<1ms per log)
- **No Blocking**: Application doesn't wait for log export

## Troubleshooting

### Logs Not Appearing

1. **Check endpoint connectivity**:
   ```powershell
   Test-NetConnection -ComputerName localhost -Port 4317
   ```

2. **Verify OTLP endpoint is running**:
   ```powershell
   docker ps | Select-String jaeger
   ```

3. **Check for TLS errors** (use `--otlp-insecure` for testing)

4. **Verify service name in UI** (should be `windowsbrowserguard`)

### Log Levels

Currently, all operational logs use Info or Warn levels. To add Debug logs:

```go
// In your code
telemetry.LogDebug(ctx, "Detailed diagnostic info",
    attribute.String("detail", "value"))
```

### Testing Without Backend

To test the application without a backend:

1. **Tracing only** (no logging):
   ```powershell
   .\WindowsBrowserGuard.exe --trace-file traces.json --dry-run
   ```

2. **Console output only**:
   ```powershell
   .\WindowsBrowserGuard.exe --dry-run
   ```

## Future Enhancements

Potential improvements:

- [ ] Configurable log levels via command-line flag
- [ ] Separate OTLP endpoints for traces and logs
- [ ] Log sampling for high-volume scenarios
- [ ] Metrics export (third pillar of observability)
- [ ] Custom log formatters

## Related Documentation

- **OPENTELEMETRY.md** - Tracing setup and configuration
- **OTLP-ENDPOINTS.md** - OTLP protocol details and backend setup
- **README.md** - Main project documentation

## References

- [OpenTelemetry Logs Specification](https://opentelemetry.io/docs/specs/otel/logs/)
- [OTLP Protocol](https://opentelemetry.io/docs/specs/otlp/)
- [Go OTel Logs SDK](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/log)
