package handler

import (
	"context"
	"log/slog"
	"net/http"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/gamecontext"
	"zoamel/pokesensei/internal/view"
)

type DashboardStore interface {
	ListTeamMembers(ctx context.Context, gameStateID int64) ([]generated.ListTeamMembersRow, error)
	ListGameStates(ctx context.Context) ([]generated.ListGameStatesRow, error)
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

	gc, _ := gamecontext.FromRequest(r)

	team, err := h.store.ListTeamMembers(ctx, gc.GameStateID)
	if err != nil {
		h.log.Error("failed to list team", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	games, _ := h.store.ListGameStates(ctx)

	if err := view.DashboardPage(gc, team, games).Render(ctx, w); err != nil {
		h.log.Error("failed to render dashboard", "error", err)
	}
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
