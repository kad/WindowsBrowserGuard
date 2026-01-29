package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer trace.Tracer
	tp     *sdktrace.TracerProvider
)

// Config holds the configuration for telemetry
type Config struct {
	// TraceOutput can be: empty (disabled), "stdout", file path, or OTLP endpoint
	TraceOutput string
	
	// OTLP configuration
	OTLPEndpoint  string            // OTLP endpoint URL (e.g., "localhost:4317")
	OTLPProtocol  string            // "grpc" or "http"
	OTLPInsecure  bool              // Disable TLS
	OTLPHeaders   map[string]string // Custom headers
}

// InitTracing initializes OpenTelemetry tracing with the specified configuration
func InitTracing(cfg Config) (func(context.Context) error, error) {
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

	// Return shutdown function
	shutdown := func(ctx context.Context) error {
		err := tp.Shutdown(ctx)
		closeErr := closeFunc()
		if err != nil {
			return err
		}
		return closeErr
	}

	return shutdown, nil
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
