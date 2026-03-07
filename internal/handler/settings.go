package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/view"
)

type SettingsStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	UpdateGameVersion(ctx context.Context, arg generated.UpdateGameVersionParams) error
	UpdateStarter(ctx context.Context, arg generated.UpdateStarterParams) error
	UpdateBadgeCount(ctx context.Context, arg generated.UpdateBadgeCountParams) error
	UpdateTradingEnabled(ctx context.Context, arg generated.UpdateTradingEnabledParams) error
	ListGameVersions(ctx context.Context) ([]generated.GameVersion, error)
	ClearTeam(ctx context.Context, gameStateID int32) error
}

type SettingsHandler struct {
	store SettingsStore
	log   *slog.Logger
}

func NewSettings(store SettingsStore, log *slog.Logger) *SettingsHandler {
	return &SettingsHandler{store: store, log: log}
}

// GET /settings — render settings page
func (h *SettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}

	versions, err := h.store.ListGameVersions(ctx)
	if err != nil {
		h.log.Error("failed to list versions", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.SettingsPage(gs, versions).Render(ctx, w); err != nil {
		h.log.Error("failed to render settings", "error", err)
	}
}

// PATCH /settings/game
func (h *SettingsHandler) HandleGameUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	gameVersionID, err := strconv.Atoi(r.FormValue("game_version_id"))
	if err != nil {
		http.Error(w, "Invalid game version", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateGameVersion(ctx, generated.UpdateGameVersionParams{
		GameVersionID: pgtype.Int4{Int32: int32(gameVersionID), Valid: true},
		ID:            gs.ID,
	}); err != nil {
		h.log.Error("failed to update game", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Changing game resets team
	if err := h.store.ClearTeam(ctx, gs.ID); err != nil {
		h.log.Error("failed to clear team", "error", err)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// PATCH /settings/starter
func (h *SettingsHandler) HandleStarterUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	starterID, err := strconv.Atoi(r.FormValue("starter_pokemon_id"))
	if err != nil {
		http.Error(w, "Invalid starter", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateStarter(ctx, generated.UpdateStarterParams{
		StarterPokemonID: pgtype.Int4{Int32: int32(starterID), Valid: true},
		ID:               gs.ID,
	}); err != nil {
		h.log.Error("failed to update starter", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// PATCH /settings/badge
func (h *SettingsHandler) HandleBadgeUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	badgeCount, err := strconv.Atoi(r.FormValue("badge_count"))
	if err != nil || badgeCount < 0 || badgeCount > 8 {
		http.Error(w, "Invalid badge count", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateBadgeCount(ctx, generated.UpdateBadgeCountParams{
		BadgeCount: int16(badgeCount),
		ID:         gs.ID,
	}); err != nil {
		h.log.Error("failed to update badge", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// PATCH /settings/trading
func (h *SettingsHandler) HandleTradingUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	tradingEnabled := r.FormValue("trading_enabled") == "on" || r.FormValue("trading_enabled") == "true"

	if err := h.store.UpdateTradingEnabled(ctx, generated.UpdateTradingEnabledParams{
		TradingEnabled: tradingEnabled,
		ID:             gs.ID,
	}); err != nil {
		h.log.Error("failed to update trading", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
