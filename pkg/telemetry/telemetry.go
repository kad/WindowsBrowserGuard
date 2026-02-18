package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer trace.Tracer
	tp     *sdktrace.TracerProvider
	logger log.Logger
	lp     *sdklog.LoggerProvider
	meter  metric.Meter
	mp     *sdkmetric.MeterProvider
)

// Config holds the configuration for telemetry
type Config struct {
	// TraceOutput can be: empty (disabled), "stdout", file path, or OTLP endpoint
	TraceOutput string

	// OTLP configuration
	OTLPEndpoint string            // OTLP endpoint URL (e.g., "localhost:4317")
	OTLPProtocol string            // "grpc" or "http"
	OTLPInsecure bool              // Disable TLS
	OTLPHeaders  map[string]string // Custom headers
}

// InitTracing initializes OpenTelemetry tracing with the specified configuration
func InitTracing(cfg Config) (func(context.Context) error, error) {
	// Surface OTel internal errors (e.g., failed exports) to stdout so they appear in logs
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Printf("[OTEL ERROR] %v\n", err)
	}))

	// If no trace output and no OTLP endpoint, tracing is disabled
	if cfg.TraceOutput == "" && cfg.OTLPEndpoint == "" {
		tracer = otel.Tracer("windowsbrowserguard")
		return func(ctx context.Context) error { return nil }, nil
	}

	var exporter sdktrace.SpanExporter
	var err error
	var closeFunc func() error = func() error { return nil }

	// Determine which exporter to use
	if cfg.OTLPEndpoint != "" {
		// Use OTLP exporter
		exporter, err = createOTLPExporter(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	} else {
		// Use stdout/file exporter
		var w io.Writer

		if cfg.TraceOutput == "stdout" {
			w = os.Stdout
		} else {
			// Open file for writing
			file, err := os.Create(cfg.TraceOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to create trace file: %w", err)
			}
			w = file
			closeFunc = file.Close
		}

		exporter, err = stdouttrace.New(
			stdouttrace.WithWriter(w),
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	}

	// Create resource with service information
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("windowsbrowserguard"),
		semconv.ServiceVersion("1.0.0"),
	)

	// Create tracer provider
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Get tracer
	tracer = tp.Tracer("windowsbrowserguard")

	// Initialize logging if OTLP endpoint is configured
	if cfg.OTLPEndpoint != "" {
		logExporter, err := createOTLPLogExporter(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create log exporter: %w", err)
		}

		// Create logger provider
		lp = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
			sdklog.WithResource(res),
		)

		// Set global logger provider
		global.SetLoggerProvider(lp)

		// Get logger
		logger = lp.Logger("windowsbrowserguard")

		// Initialize metrics
		metricExporter, err := createOTLPMetricExporter(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}

		// Create meter provider
		mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			sdkmetric.WithResource(res),
		)

		// Set global meter provider
		otel.SetMeterProvider(mp)

		// Get meter
		meter = mp.Meter("windowsbrowserguard")
	}

	// Return shutdown function
	shutdown := func(ctx context.Context) error {
		var traceErr, logErr, metricErr error

		if tp != nil {
			traceErr = tp.Shutdown(ctx)
		}

		if lp != nil {
			logErr = lp.Shutdown(ctx)
		}

		if mp != nil {
			metricErr = mp.Shutdown(ctx)
		}

		closeErr := closeFunc()

		if traceErr != nil {
			return traceErr
		}
		if logErr != nil {
			return logErr
		}
		if metricErr != nil {
			return metricErr
		}
		return closeErr
	}

	return shutdown, nil
}

// createOTLPLogExporter creates an OTLP log exporter based on the protocol
func createOTLPLogExporter(cfg Config) (sdklog.Exporter, error) {
	protocol := strings.ToLower(cfg.OTLPProtocol)
	if protocol == "" {
		protocol = "grpc"
	}

	switch protocol {
	case "grpc":
		return createOTLPLogGRPCExporter(cfg)
	case "http":
		return createOTLPLogHTTPExporter(cfg)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

// createOTLPLogGRPCExporter creates a gRPC OTLP log exporter
func createOTLPLogGRPCExporter(cfg Config) (sdklog.Exporter, error) {
	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlploggrpc.WithHeaders(cfg.OTLPHeaders))
	}

	return otlploggrpc.New(context.Background(), opts...)
}

// createOTLPLogHTTPExporter creates an HTTP OTLP log exporter
func createOTLPLogHTTPExporter(cfg Config) (sdklog.Exporter, error) {
	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(cfg.OTLPHeaders))
	}

	return otlploghttp.New(context.Background(), opts...)
}

// Metric exporters

// createOTLPMetricExporter creates an OTLP metric exporter based on the protocol
func createOTLPMetricExporter(cfg Config) (sdkmetric.Exporter, error) {
	protocol := strings.ToLower(cfg.OTLPProtocol)
	if protocol == "" {
		protocol = "grpc"
	}

	switch protocol {
	case "grpc":
		return createOTLPMetricGRPCExporter(cfg)
	case "http":
		return createOTLPMetricHTTPExporter(cfg)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

// createOTLPMetricGRPCExporter creates a gRPC OTLP metric exporter
func createOTLPMetricGRPCExporter(cfg Config) (sdkmetric.Exporter, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.OTLPHeaders))
	}

	return otlpmetricgrpc.New(context.Background(), opts...)
}

// createOTLPMetricHTTPExporter creates an HTTP OTLP metric exporter
func createOTLPMetricHTTPExporter(cfg Config) (sdkmetric.Exporter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.OTLPHeaders))
	}

	return otlpmetrichttp.New(context.Background(), opts...)
}

// createOTLPExporter creates an OTLP exporter based on the protocol
func createOTLPExporter(cfg Config) (sdktrace.SpanExporter, error) {
	protocol := strings.ToLower(cfg.OTLPProtocol)
	if protocol == "" {
		protocol = "grpc" // Default to gRPC
	}

	switch protocol {
	case "grpc":
		return createOTLPGRPCExporter(cfg)
	case "http":
		return createOTLPHTTPExporter(cfg)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s (use 'grpc' or 'http')", protocol)
	}
}

// createOTLPGRPCExporter creates a gRPC OTLP exporter
func createOTLPGRPCExporter(cfg Config) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.OTLPHeaders))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}

// createOTLPHTTPExporter creates an HTTP OTLP exporter
func createOTLPHTTPExporter(cfg Config) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
	}

	if cfg.OTLPInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	if len(cfg.OTLPHeaders) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.OTLPHeaders))
	}

	return otlptracehttp.New(context.Background(), opts...)
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = otel.Tracer("windowsbrowserguard")
	}

	opts := []trace.SpanStartOption{}
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	return tracer.Start(ctx, name, opts...)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		opts := []trace.EventOption{}
		if len(attrs) > 0 {
			opts = append(opts, trace.WithAttributes(attrs...))
		}
		span.AddEvent(name, opts...)
	}
}

// SetAttributes adds attributes to the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(attrs...)
	}
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.RecordError(err)
	}
}

// Logging functions

// LogDebug emits a debug-level log message
func LogDebug(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	emitLog(ctx, log.SeverityDebug, msg, attrs...)
}

// LogInfo emits an info-level log message
func LogInfo(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	emitLog(ctx, log.SeverityInfo, msg, attrs...)
}

// LogWarn emits a warning-level log message
func LogWarn(ctx context.Context, msg string, attrs ...attribute.KeyValue) {
	emitLog(ctx, log.SeverityWarn, msg, attrs...)
}

// LogError emits an error-level log message
func LogError(ctx context.Context, msg string, err error, attrs ...attribute.KeyValue) {
	allAttrs := attrs
	if err != nil {
		allAttrs = append(allAttrs, attribute.String("error", err.Error()))
	}
	emitLog(ctx, log.SeverityError, msg, allAttrs...)
}

// emitLog is the internal function that emits logs
func emitLog(ctx context.Context, severity log.Severity, msg string, attrs ...attribute.KeyValue) {
	if logger == nil {
		return // Logging not initialized
	}

	// Convert attributes to log.KeyValue, preserving type information
	logAttrs := make([]log.KeyValue, len(attrs))
	for i, attr := range attrs {
		switch attr.Value.Type() {
		case attribute.INT64:
			logAttrs[i] = log.Int(string(attr.Key), int(attr.Value.AsInt64()))
		case attribute.FLOAT64:
			logAttrs[i] = log.Float64(string(attr.Key), attr.Value.AsFloat64())
		case attribute.BOOL:
			logAttrs[i] = log.Bool(string(attr.Key), attr.Value.AsBool())
		default:
			logAttrs[i] = log.String(string(attr.Key), attr.Value.AsString())
		}
	}

	// Create the log record
	record := log.Record{}
	record.SetTimestamp(time.Now())
	record.SetSeverity(severity)
	record.SetBody(log.StringValue(msg))
	record.AddAttributes(logAttrs...)

	logger.Emit(ctx, record)
}

// Printf formats a message and emits it to both stdout and the OTel log pipeline.
// Use as a drop-in replacement for fmt.Printf to also export logs via OTLP.
func Printf(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Print(msg)
	body := strings.TrimRight(msg, "\n\r")
	if body != "" {
		emitLog(ctx, log.SeverityInfo, body)
	}
}

// Println emits args (space-separated) to both stdout and the OTel log pipeline.
// Use as a drop-in replacement for fmt.Println to also export logs via OTLP.
func Println(ctx context.Context, args ...interface{}) {
	fmt.Println(args...)
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprint(a)
	}
	msg := strings.Join(parts, " ")
	if msg != "" {
		emitLog(ctx, log.SeverityInfo, msg)
	}
}

// Metrics functions

// RecordExtensionDetected increments the counter for detected extensions
func RecordExtensionDetected(ctx context.Context, browser string, extensionID string) {
	if meter == nil {
		return
	}
	counter, _ := meter.Int64Counter("browser_guard.extensions.detected",
		metric.WithDescription("Number of forced extensions detected"),
		metric.WithUnit("{extension}"))
	counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("browser", browser),
			attribute.String("extension_id", extensionID),
		))
}

// RecordExtensionBlocked increments the counter for blocked extensions
func RecordExtensionBlocked(ctx context.Context, browser string, extensionID string) {
	if meter == nil {
		return
	}
	counter, _ := meter.Int64Counter("browser_guard.extensions.blocked",
		metric.WithDescription("Number of extensions blocked"),
		metric.WithUnit("{extension}"))
	counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("browser", browser),
			attribute.String("extension_id", extensionID),
		))
}

// RecordRegistryOperation records a registry operation
func RecordRegistryOperation(ctx context.Context, operation string, success bool) {
	if meter == nil {
		return
	}
	counter, _ := meter.Int64Counter("browser_guard.registry.operations",
		metric.WithDescription("Number of registry operations performed"),
		metric.WithUnit("{operation}"))
	counter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.Bool("success", success),
		))
}

// RecordRegistryStateSize records the size of registry state
func RecordRegistryStateSize(ctx context.Context, subkeys int, values int) {
	if meter == nil {
		return
	}

	subkeysGauge, _ := meter.Int64Gauge("browser_guard.registry.subkeys",
		metric.WithDescription("Number of registry subkeys being monitored"),
		metric.WithUnit("{subkey}"))
	subkeysGauge.Record(ctx, int64(subkeys))

	valuesGauge, _ := meter.Int64Gauge("browser_guard.registry.values",
		metric.WithDescription("Number of registry values being monitored"),
		metric.WithUnit("{value}"))
	valuesGauge.Record(ctx, int64(values))
}

// RecordOperationDuration records the duration of an operation
func RecordOperationDuration(ctx context.Context, operation string, duration time.Duration) {
	if meter == nil {
		return
	}
	histogram, _ := meter.Float64Histogram("browser_guard.operation.duration",
		metric.WithDescription("Duration of operations"),
		metric.WithUnit("ms"))
	histogram.Record(ctx, float64(duration.Milliseconds()),
		metric.WithAttributes(attribute.String("operation", operation)))
}
