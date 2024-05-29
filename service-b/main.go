package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/FelpsCorrea/OpenTelemetry-Go/service-b/internal/infra/otel"
	"github.com/FelpsCorrea/OpenTelemetry-Go/service-b/internal/infra/web/handlers"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Set up a channel to listen for OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Create a context that is canceled when an interrupt signal is received
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initialize OpenTelemetry provider
	otelProvider := &otel.OpenTelemetryProvider{
		ServiceName:  "microservice-tracer",
		CollectorURL: "otel-collector:4317",
	}

	otelShutdown, err := otelProvider.InitProvider()
	if err != nil {
		fmt.Println("Error initializing OpenTelemetry provider:", err)
		return
	}
	defer otelShutdown(ctx)

	// Set up the router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/{city}", handlers.GetTemperature)
	r.Get("/metrics", promhttpHandler())

	// Start the server in a goroutine
	go func() {
		log.Println("Starting server on port", ":8181")
		if err := http.ListenAndServe(":8181", r); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for an interrupt signal to gracefully shut down
	select {
	case <-sigCh:
		log.Println("Shutting down gracefully, CTRL+C pressed...")
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	// Create a context with a timeout for the shutdown
	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}

// promhttpHandler returns a handler function for Prometheus metrics
func promhttpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	}
}
