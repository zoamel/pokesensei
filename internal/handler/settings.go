package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/view"
)

type SettingsStore interface {
	GetActiveGameState(ctx context.Context) (generated.GameState, error)
	UpdateGameVersion(ctx context.Context, arg generated.UpdateGameVersionParams) error
	UpdateStarter(ctx context.Context, arg generated.UpdateStarterParams) error
	UpdateBadgeCount(ctx context.Context, arg generated.UpdateBadgeCountParams) error
	UpdateTradingEnabled(ctx context.Context, arg generated.UpdateTradingEnabledParams) error
	ListGameVersions(ctx context.Context) ([]generated.GameVersion, error)
	ClearTeam(ctx context.Context, gameStateID int64) error
	DeactivateAllGameStates(ctx context.Context) error
	ActivateGameState(ctx context.Context, id int64) error
	GetGameStateForVersion(ctx context.Context, gameVersionID sql.NullInt64) (generated.GameState, error)
	CreateGameState(ctx context.Context, arg generated.CreateGameStateParams) (generated.GameState, error)
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

	gs, err := h.store.GetActiveGameState(ctx)
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

// PATCH /settings/game — switch to a different game version (creates new game state if needed)
func (h *SettingsHandler) HandleGameUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	gameVersionID, err := strconv.Atoi(r.FormValue("game_version_id"))
	if err != nil {
		http.Error(w, "Invalid game version", http.StatusBadRequest)
		return
	}

	gvID := sql.NullInt64{Int64: int64(gameVersionID), Valid: true}

	// Check if a game state already exists for this version
	existing, err := h.store.GetGameStateForVersion(ctx, gvID)
	if err == nil {
		// Switch to existing playthrough
		if err := h.store.DeactivateAllGameStates(ctx); err != nil {
			h.log.Error("failed to deactivate game states", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if err := h.store.ActivateGameState(ctx, existing.ID); err != nil {
			h.log.Error("failed to activate game state", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	} else {
		// Create new playthrough for this game version
		if err := h.store.DeactivateAllGameStates(ctx); err != nil {
			h.log.Error("failed to deactivate game states", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if _, err := h.store.CreateGameState(ctx, generated.CreateGameStateParams{
			GameVersionID: gvID,
		}); err != nil {
			h.log.Error("failed to create game state", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
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
	gs, err := h.store.GetActiveGameState(ctx)
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
		StarterPokemonID: sql.NullInt64{Int64: int64(starterID), Valid: true},
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
	gs, err := h.store.GetActiveGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	badgeCount, err := strconv.Atoi(r.FormValue("badge_count"))
	if err != nil || badgeCount < 0 || badgeCount > 16 {
		http.Error(w, "Invalid badge count", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateBadgeCount(ctx, generated.UpdateBadgeCountParams{
		BadgeCount: int64(badgeCount),
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
	gs, err := h.store.GetActiveGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	tradingEnabled := r.FormValue("trading_enabled") == "on" || r.FormValue("trading_enabled") == "true"
	var tradingVal int64
	if tradingEnabled {
		tradingVal = 1
	}

	if err := h.store.UpdateTradingEnabled(ctx, generated.UpdateTradingEnabledParams{
		TradingEnabled: tradingVal,
		ID:             gs.ID,
	}); err != nil {
		h.log.Error("failed to update trading", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
