package handlers

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
)

type Handlers struct {
	logger         *slog.Logger
	tracer         trace.Tracer
	workStepsTotal prometheus.Counter
}

func New(logger *slog.Logger, tracer trace.Tracer, workStepsTotal prometheus.Counter) *Handlers {
	return &Handlers{
		logger:         logger,
		tracer:         tracer,
		workStepsTotal: workStepsTotal,
	}
}
