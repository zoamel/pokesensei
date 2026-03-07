package handler

import (
	"log/slog"
	"net/http"

	"zoamel/pokesensei/internal/view"
)

type HomeHandler struct {
	log *slog.Logger
}

func NewHome(log *slog.Logger) *HomeHandler {
	return &HomeHandler{log: log}
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := view.HomePage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render home page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
