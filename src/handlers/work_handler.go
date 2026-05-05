package handlers

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (h *Handlers) Work(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "perform synthetic work")
	defer span.End()

	steps := queryInt(r, "steps", 3)
	if steps < 1 {
		steps = 1
	}
	if steps > 10 {
		steps = 10
	}
	span.SetAttributes(attribute.Int("work.steps", steps))
	h.logger.Info("starting synthetic work", slog.Int("steps", steps))

	results := make([]string, 0, steps)
	for i := 1; i <= steps; i++ {
		results = append(results, h.runStep(ctx, i))
	}
	h.workStepsTotal.Add(float64(steps))

	writeJSON(w, http.StatusOK, map[string]any{
		"steps":   steps,
		"results": results,
	})
}

func (h *Handlers) runStep(ctx context.Context, step int) string {
	_, span := h.tracer.Start(ctx, "work step", trace.WithAttributes(attribute.Int("step", step)))
	defer span.End()

	delay := time.Duration(20+rand.Intn(180)) * time.Millisecond
	h.logger.Info("running synthetic work step", slog.Int("step", step), slog.Duration("delay", delay))
	time.Sleep(delay)

	return "step " + strconv.Itoa(step) + " complete"
}
