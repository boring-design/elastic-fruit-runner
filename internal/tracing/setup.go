package tracing

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const ServiceName = "elastic-fruit-runner"

// Setup initializes the global OTel TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, traces are exported via OTLP HTTP.
// Otherwise traces are written to stdout (useful for local debugging).
// Returns a shutdown function that must be called before the process exits.
func Setup(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var exporter sdktrace.SpanExporter

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exporter, err = otlptracehttp.New(ctx)
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName(ServiceName)),
		resource.WithOS(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
