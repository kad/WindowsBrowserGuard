package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

// InitTracing initializes OpenTelemetry tracing with the specified output
// If traceOutput is empty, tracing is disabled
// If traceOutput is "stdout", traces go to stdout
// Otherwise, traceOutput is treated as a file path
func InitTracing(traceOutput string) (func(context.Context) error, error) {
	if traceOutput == "" {
		// Tracing disabled
		tracer = otel.Tracer("windowsbrowserguard")
		return func(ctx context.Context) error { return nil }, nil
	}

	var w io.Writer
	var closeFunc func() error

	if traceOutput == "stdout" {
		w = os.Stdout
		closeFunc = func() error { return nil }
	} else {
		// Open file for writing
		file, err := os.Create(traceOutput)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace file: %w", err)
		}
		w = file
		closeFunc = file.Close
	}

	// Create stdout exporter
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(w),
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
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
