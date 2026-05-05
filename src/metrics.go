package main

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_http_requests_total",
			Help: "Total number of HTTP requests handled by the API.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "api_http_requests_in_flight",
			Help: "Number of API HTTP requests currently in flight.",
		},
	)
	workStepsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "api_work_steps_total",
			Help: "Total synthetic work steps completed.",
		},
	)
)

func observeRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		route := routeLabel(r)
		status := strconv.Itoa(recorder.status)
		httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route, status).Observe(duration.Seconds())

		logger.Info(
			"request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.status),
			slog.Duration("duration", duration),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func routeLabel(r *http.Request) string {
	switch r.URL.Path {
	case "/healthz", "/hello", "/work", "/error", "/metrics":
		return r.URL.Path
	default:
		return "unknown"
	}
}
