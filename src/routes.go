package main

import (
	"log/slog"
	"net/http"

	"github.com/jolienai/golang_otel_grafana_example/src/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func newHTTPHandler(logger *slog.Logger) http.Handler {
	apiHandlers := handlers.New(logger, tracer, workStepsTotal)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", apiHandlers.Health)
	mux.HandleFunc("GET /hello", apiHandlers.Hello)
	mux.HandleFunc("GET /work", apiHandlers.Work)
	mux.HandleFunc("GET /error", apiHandlers.Error)
	mux.Handle("GET /metrics", promhttp.Handler())

	return otelhttp.NewHandler(
		observeRequests(logger, mux),
		"http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}
