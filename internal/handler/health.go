package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

type HealthChecker interface {
	Ping(ctx context.Context) (int32, error)
}

type HealthHandler struct {
	checker HealthChecker
	log     *slog.Logger
}

func NewHealth(checker HealthChecker, log *slog.Logger) *HealthHandler {
	return &HealthHandler{checker: checker, log: log}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := h.checker.Ping(r.Context())

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		h.log.Error("health check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
