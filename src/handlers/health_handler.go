package handlers

import "net/http"

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	h.logger.Info("health check requested")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
