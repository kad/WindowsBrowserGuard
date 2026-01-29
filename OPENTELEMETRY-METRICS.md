# OpenTelemetry Metrics Integration

## Overview

WindowsBrowserGuard now exports OpenTelemetry metrics to OTLP-compatible backends, providing quantitative insights into application behavior, extension detections, and registry operations. Combined with traces and logs, metrics complete the **three pillars of observability**.

## Features

- **Counters**: Track cumulative values (extensions detected/blocked, operations performed)
- **Gauges**: Monitor current state (registry size, active monitors)
- **Histograms**: Distribution of operation durations
- **OTLP Export**: Metrics exported to same endpoint as traces and logs
- **Protocol Support**: Both gRPC and HTTP protocols
- **Automatic Collection**: Metrics collected during normal operation

## Available Metrics

### Extension Metrics

#### `browser_guard.extensions.detected`
**Type**: Counter  
**Unit**: `{extension}`  
**Description**: Number of forced extensions detected  
**Attributes**:
- `browser` (string): Browser type (`chrome`, `edge`, `firefox`)
- `extension_id` (string): Extension identifier

**Example**:
```
browser_guard.extensions.detected{browser="chrome", extension_id="abcd1234"} = 5
browser_guard.extensions.detected{browser="edge", extension_id="efgh5678"} = 2
```

#### `browser_guard.extensions.blocked`
**Type**: Counter  
**Unit**: `{extension}`  
**Description**: Number of extensions blocked  
**Attributes**:
- `browser` (string): Browser type
- `extension_id` (string): Extension identifier

**Example**:
```
browser_guard.extensions.blocked{browser="chrome", extension_id="abcd1234"} = 3
```

### Registry Metrics

#### `browser_guard.registry.operations`
**Type**: Counter  
**Unit**: `{operation}`  
**Description**: Number of registry operations performed  
**Attributes**:
- `operation` (string): Operation type (`capture`, `delete`, `add`, `remove`)
- `success` (boolean): Whether operation succeeded

**Example**:
```
browser_guard.registry.operations{operation="capture", success="true"} = 10
browser_guard.registry.operations{operation="delete", success="false"} = 1
```

#### `browser_guard.registry.subkeys`
**Type**: Gauge  
**Unit**: `{subkey}`  
**Description**: Number of registry subkeys being monitored  
**Attributes**: None

**Example**:
```
browser_guard.registry.subkeys = 249
```

#### `browser_guard.registry.values`
**Type**: Gauge  
**Unit**: `{value}`  
**Description**: Number of registry values being monitored  
**Attributes**: None

**Example**:
```
browser_guard.registry.values = 538
```

### Performance Metrics

#### `browser_guard.operation.duration`
**Type**: Histogram  
**Unit**: `ms` (milliseconds)  
**Description**: Duration of operations  
**Attributes**:
- `operation` (string): Operation name (`capture_registry_state`, `delete_key`, etc.)

**Example**:
```
browser_guard.operation.duration{operation="capture_registry_state"}
  count=10, sum=150ms, min=10ms, max=25ms, avg=15ms
```

## Configuration

Metrics are automatically enabled when you specify an OTLP endpoint. No additional flags are required.

### Command-Line Usage

```powershell
# Enable metrics via OTLP gRPC (default)
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure

# Enable metrics via OTLP HTTP
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4318 --otlp-protocol http --otlp-insecure

# With authentication
.\WindowsBrowserGuard.exe --otlp-endpoint api.example.com:4317 --otlp-headers "x-api-key=secret"
```

## Integration with Observability Platforms

### Prometheus + Jaeger

Use OpenTelemetry Collector to forward metrics to Prometheus:

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  jaeger:
    endpoint: jaeger:14250

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
    traces:
      receivers: [otlp]
      exporters: [jaeger]
```

```powershell
# Run WindowsBrowserGuard
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

### Grafana Cloud

Send metrics, traces, and logs to Grafana Cloud:

```powershell
.\WindowsBrowserGuard.exe \
  --otlp-endpoint otlp-gateway.grafana.net:443 \
  --otlp-headers "authorization=Basic <base64-encoded-credentials>"
```

### AWS CloudWatch

Use AWS Distro for OpenTelemetry (ADOT):

```powershell
# ADOT Collector exports to CloudWatch
.\WindowsBrowserGuard.exe --otlp-endpoint localhost:4317 --otlp-insecure
```

### Azure Monitor

Use Azure Monitor OpenTelemetry Exporter:

```powershell
.\WindowsBrowserGuard.exe \
  --otlp-endpoint <ingestion-endpoint> \
  --otlp-headers "x-instrumentation-key=<your-key>"
```

## Dashboards and Visualization

### Grafana Dashboard Example

Create a Grafana dashboard with these panels:

#### Panel 1: Extensions Detected Over Time
```promql
rate(browser_guard_extensions_detected_total[5m])
```

#### Panel 2: Extensions Blocked by Browser
```promql
sum by (browser) (browser_guard_extensions_blocked_total)
```

#### Panel 3: Registry Operation Success Rate
```promql
sum(rate(browser_guard_registry_operations_total{success="true"}[5m]))
/ 
sum(rate(browser_guard_registry_operations_total[5m]))
* 100
```

#### Panel 4: Registry Size
```promql
browser_guard_registry_subkeys
browser_guard_registry_values
```

#### Panel 5: Operation Duration (P95)
```promql
histogram_quantile(0.95, 
  rate(browser_guard_operation_duration_bucket[5m]))
```

### Example Dashboard JSON

```json
{
  "dashboard": {
    "title": "WindowsBrowserGuard Metrics",
    "panels": [
      {
        "title": "Extensions Detected",
        "targets": [{
          "expr": "rate(browser_guard_extensions_detected_total[5m])"
        }]
      },
      {
        "title": "Registry Size",
        "targets": [
          {"expr": "browser_guard_registry_subkeys"},
          {"expr": "browser_guard_registry_values"}
        ]
      },
      {
        "title": "Operation Success Rate",
        "targets": [{
          "expr": "sum(rate(browser_guard_registry_operations_total{success=\"true\"}[5m])) / sum(rate(browser_guard_registry_operations_total[5m])) * 100"
        }]
      }
    ]
  }
}
```

## Alerting Rules

### Prometheus Alerting Rules

```yaml
groups:
  - name: browser_guard_alerts
    rules:
      # Alert on high extension detection rate
      - alert: HighExtensionDetectionRate
        expr: rate(browser_guard_extensions_detected_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High rate of extension detections"
          description: "Detected {{ $value }} extensions per second"

      # Alert on registry operation failures
      - alert: RegistryOperationFailures
        expr: rate(browser_guard_registry_operations_total{success="false"}[5m]) > 1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Registry operations failing"
          description: "{{ $value }} operations per second are failing"

      # Alert on slow registry capture
      - alert: SlowRegistryCapture
        expr: histogram_quantile(0.95, rate(browser_guard_operation_duration_bucket{operation="capture_registry_state"}[5m])) > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Registry capture is slow"
          description: "P95 capture time is {{ $value }}ms"
```

## Metric Cardinality

### Low Cardinality Metrics
- `browser_guard.registry.subkeys` - Single time series
- `browser_guard.registry.values` - Single time series

### Medium Cardinality Metrics
- `browser_guard.registry.operations` - ~8 time series (4 ops × 2 success states)
- `browser_guard.operation.duration` - ~5 time series (operation types)

### High Cardinality Metrics
- `browser_guard.extensions.detected` - Unbounded (unique extension IDs)
- `browser_guard.extensions.blocked` - Unbounded (unique extension IDs)

**Note**: Extension metrics can create many time series if many unique extensions are detected. Consider sampling or aggregation if cardinality becomes an issue.

## Architecture

### Telemetry Package

```go
// Record extension detection
telemetry.RecordExtensionDetected(ctx, "chrome", "abcd1234")

// Record extension blocked
telemetry.RecordExtensionBlocked(ctx, "edge", "efgh5678")

// Record registry operation
telemetry.RecordRegistryOperation(ctx, "delete", true)

// Record registry state size
telemetry.RecordRegistryStateSize(ctx, 249, 538)

// Record operation duration
telemetry.RecordOperationDuration(ctx, "capture_registry_state", duration)
```

### Meter Provider

- **Meter**: `windowsbrowserguard`
- **Reader**: PeriodicReader (exports every 60 seconds by default)
- **Exporter**: OTLP (gRPC or HTTP)
- **Resource**: Service name, version, schema URL

### Export Intervals

- **Default**: 60 seconds
- **Configurable**: Via environment variable `OTEL_METRIC_EXPORT_INTERVAL`
- **On Shutdown**: Remaining metrics flushed

## Performance Considerations

- **Overhead**: Minimal (<1% CPU for metric recording)
- **Memory**: Small footprint (metrics aggregated before export)
- **Batching**: Metrics batched and sent periodically
- **Non-blocking**: Metric recording doesn't block application

## Troubleshooting

### Metrics Not Appearing

1. **Verify OTLP endpoint is reachable**:
   ```powershell
   Test-NetConnection -ComputerName localhost -Port 4317
   ```

2. **Check collector/backend logs** for ingestion errors

3. **Verify service name** in metrics backend (should be `windowsbrowserguard`)

4. **Ensure operations are running** (metrics only recorded during activity)

### High Cardinality Issues

If extension metrics create too many time series:

1. **Aggregate by browser only**:
   ```promql
   sum by (browser) (browser_guard_extensions_detected_total)
   ```

2. **Use recording rules** to pre-aggregate:
   ```yaml
   groups:
     - name: browser_guard_recording
       rules:
         - record: browser_guard:extensions_detected:rate5m
           expr: rate(browser_guard_extensions_detected_total[5m])
   ```

3. **Consider sampling** in high-volume environments

### Testing Metrics

Test metrics locally with Prometheus:

```powershell
# Start Prometheus with OTel Collector
docker run -d -p 9090:9090 -p 4317:4317 otel/opentelemetry-collector

# Run WindowsBrowserGuard
.\WindowsBrowserGuard.exe --dry-run --otlp-endpoint localhost:4317 --otlp-insecure

# Query metrics
# Open http://localhost:9090
# Query: browser_guard_registry_subkeys
```

## Metric Naming Conventions

WindowsBrowserGuard follows OpenTelemetry semantic conventions:

- **Prefix**: `browser_guard` (application namespace)
- **Domain**: `extensions`, `registry`, `operation` (functional area)
- **Name**: Descriptive noun (e.g., `detected`, `blocked`, `duration`)
- **Separator**: Dot (`.`) between components
- **Unit**: Included in metric description

## Correlation with Traces and Logs

Metrics work alongside traces and logs for complete observability:

1. **High-level trends** (metrics) → Detect anomalies
2. **Drill into traces** → Understand what happened
3. **View related logs** → Get detailed context

**Example Workflow**:
1. Alert: High extension detection rate (metric)
2. View traces for extension detection operations
3. Check logs for specific extension IDs and errors
4. Correlate all three using timestamps and service name

## Future Enhancements

Potential additions:

- [ ] System metrics (CPU, memory usage)
- [ ] Network metrics (if applicable)
- [ ] Custom percentiles for histograms
- [ ] Exemplars linking metrics to traces
- [ ] More granular operation metrics

## Related Documentation

- **OPENTELEMETRY.md** - Tracing configuration
- **OPENTELEMETRY-LOGGING.md** - Logging setup
- **OTLP-ENDPOINTS.md** - Backend integration
- **README.md** - Project overview

## References

- [OpenTelemetry Metrics Specification](https://opentelemetry.io/docs/specs/otel/metrics/)
- [Metric Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/general/metrics/)
- [Go OTel Metrics SDK](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/metric)
- [OTLP Protocol](https://opentelemetry.io/docs/specs/otlp/)
