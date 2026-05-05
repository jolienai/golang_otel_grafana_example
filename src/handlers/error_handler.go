package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (h *Handlers) Error(w http.ResponseWriter, r *http.Request) {
	_, span := h.tracer.Start(r.Context(), "simulate server error")
	defer span.End()

	err := errors.New("simulated internal server error")
	span.SetAttributes(attribute.Bool("error.simulated", true))
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	h.logger.Error("simulated server error requested", slog.Any("error", err))

	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"error": err.Error(),
	})
}
