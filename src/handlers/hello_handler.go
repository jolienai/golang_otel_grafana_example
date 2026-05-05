package handlers

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func (h *Handlers) Hello(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "build greeting")
	defer span.End()

	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}
	span.SetAttributes(attribute.String("hello.name", name))
	h.logger.Info("building greeting", slog.String("name", name))

	greeting := h.slowGreeting(ctx, name)
	writeJSON(w, http.StatusOK, map[string]string{"message": greeting})
}

func (h *Handlers) slowGreeting(ctx context.Context, name string) string {
	_, span := h.tracer.Start(ctx, "simulate greeting lookup")
	defer span.End()

	delay := time.Duration(30+rand.Intn(120)) * time.Millisecond
	h.logger.Info("simulating greeting lookup", slog.Duration("delay", delay))
	time.Sleep(delay)

	return "hello, " + name
}
