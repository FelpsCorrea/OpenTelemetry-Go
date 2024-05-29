package otel

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type OpenTelemetryProvider struct {
	ServiceName    string
	CollectorURL   string
	TracerProvider *sdktrace.TracerProvider
}

func (o *OpenTelemetryProvider) InitProvider() (func(context.Context) error, error) {
	ctx := context.Background()

	// Create a resource with the service name attribute.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(o.ServiceName),
		))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up a context with a timeout for creating the gRPC connection.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create a gRPC connection to the OpenTelemetry Collector.
	conn, err := grpc.DialContext(ctx, o.CollectorURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	// Create a trace exporter using the gRPC connection.
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter:  %w", err)
	}

	// Create a batch span processor for the exporter.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)

	// Initialize the TracerProvider with the batch span processor and resource.
	o.TracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set the global TracerProvider.
	otel.SetTracerProvider(o.TracerProvider)

	// Set the global propagator for context propagation.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return the shutdown function for the TracerProvider.
	return o.TracerProvider.Shutdown, nil
}
