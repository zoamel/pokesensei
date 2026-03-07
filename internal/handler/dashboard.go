package handler

import (
	"context"
	"log/slog"
	"net/http"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/view"
)

type DashboardStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	ListTeamMembers(ctx context.Context, gameStateID int32) ([]generated.ListTeamMembersRow, error)
}

type DashboardHandler struct {
	store DashboardStore
	log   *slog.Logger
}

func NewDashboard(store DashboardStore, log *slog.Logger) *DashboardHandler {
	return &DashboardHandler{store: store, log: log}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		// No game state — redirect to onboarding
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}

	team, err := h.store.ListTeamMembers(ctx, gs.ID)
	if err != nil {
		h.log.Error("failed to list team", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.DashboardPage(gs, team).Render(ctx, w); err != nil {
		h.log.Error("failed to render dashboard", "error", err)
	}
}
