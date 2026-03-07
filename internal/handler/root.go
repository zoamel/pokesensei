package handler

import (
	"context"
	"log/slog"
	"net/http"

	"zoamel/pokesensei/db/generated"
)

type RootStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
}

type RootHandler struct {
	store RootStore
	log   *slog.Logger
}

func NewRoot(store RootStore, log *slog.Logger) *RootHandler {
	return &RootHandler{store: store, log: log}
}

// GET / — redirect to dashboard if game state exists, otherwise onboarding
func (h *RootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := h.store.GetGameState(r.Context())
	if err != nil {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
