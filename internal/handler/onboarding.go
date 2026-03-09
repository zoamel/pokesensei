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

type GameStateStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	CreateGameState(ctx context.Context, arg generated.CreateGameStateParams) (generated.GameState, error)
	UpdateGameVersion(ctx context.Context, arg generated.UpdateGameVersionParams) error
	UpdateStarter(ctx context.Context, arg generated.UpdateStarterParams) error
	UpdateBadgeCount(ctx context.Context, arg generated.UpdateBadgeCountParams) error
	UpdateTradingEnabled(ctx context.Context, arg generated.UpdateTradingEnabledParams) error
	ListGameVersions(ctx context.Context) ([]generated.GameVersion, error)
}

type OnboardingHandler struct {
	store GameStateStore
	log   *slog.Logger
}

func NewOnboarding(store GameStateStore, log *slog.Logger) *OnboardingHandler {
	return &OnboardingHandler{store: store, log: log}
}

// GET /onboarding — render the full onboarding page
func (h *OnboardingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	versions, err := h.store.ListGameVersions(r.Context())
	if err != nil {
		h.log.Error("failed to list game versions", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.OnboardingPage(versions).Render(r.Context(), w); err != nil {
		h.log.Error("failed to render onboarding", "error", err)
	}
}

// POST /onboarding/game — save game selection, return starter step
func (h *OnboardingHandler) HandleGameStep(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	gameVersionID, err := strconv.Atoi(r.FormValue("game_version_id"))
	if err != nil {
		http.Error(w, "Invalid game version", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get or create game state
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		// No game state yet — create one
		gs, err = h.store.CreateGameState(ctx, generated.CreateGameStateParams{
			GameVersionID: sql.NullInt64{Int64: int64(gameVersionID), Valid: true},
		})
		if err != nil {
			h.log.Error("failed to create game state", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	} else {
		// Update existing
		err = h.store.UpdateGameVersion(ctx, generated.UpdateGameVersionParams{
			GameVersionID: sql.NullInt64{Int64: int64(gameVersionID), Valid: true},
			ID:            gs.ID,
		})
		if err != nil {
			h.log.Error("failed to update game version", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Determine starters based on game version
	starters := startersForGame(gameVersionID)
	if err := view.OnboardingStarterStep(starters).Render(ctx, w); err != nil {
		h.log.Error("failed to render starter step", "error", err)
	}
}

// POST /onboarding/starter — save starter, return badge step
func (h *OnboardingHandler) HandleStarterStep(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	starterID, err := strconv.Atoi(r.FormValue("starter_pokemon_id"))
	if err != nil {
		http.Error(w, "Invalid starter", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "Complete game selection first", http.StatusBadRequest)
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

	if err := view.OnboardingBadgeStep().Render(ctx, w); err != nil {
		h.log.Error("failed to render badge step", "error", err)
	}
}

// POST /onboarding/badge — save badge count, redirect to dashboard
func (h *OnboardingHandler) HandleBadgeStep(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	badgeCount, err := strconv.Atoi(r.FormValue("badge_count"))
	if err != nil || badgeCount < 0 || badgeCount > 8 {
		http.Error(w, "Invalid badge count", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "Complete game selection first", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateBadgeCount(ctx, generated.UpdateBadgeCountParams{
		BadgeCount: int64(badgeCount),
		ID:         gs.ID,
	}); err != nil {
		h.log.Error("failed to update badge count", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to dashboard
	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

func startersForGame(gameVersionID int) []view.StarterInfo {
	// FRLG starters: Bulbasaur, Charmander, Squirtle
	// HGSS starters: Chikorita, Cyndaquil, Totodile
	switch {
	case gameVersionID == 10 || gameVersionID == 11: // FireRed/LeafGreen
		return []view.StarterInfo{
			{1, "Bulbasaur", "Grass/Poison", "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/1.png", "Strong early game, resists first two gyms."},
			{4, "Charmander", "Fire", "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/4.png", "Challenging early but powerful later, evolves into Charizard."},
			{7, "Squirtle", "Water", "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/7.png", "Balanced and reliable throughout the game."},
		}
	case gameVersionID == 15 || gameVersionID == 16: // HeartGold/SoulSilver
		return []view.StarterInfo{
			{152, "Chikorita", "Grass", "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/152.png", "Defensive and supportive, but struggles against early gyms."},
			{155, "Cyndaquil", "Fire", "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/155.png", "Strong special attacker, great against Bug and Steel gyms."},
			{158, "Totodile", "Water", "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon/158.png", "Powerful physical attacker, learns Ice Fang for coverage."},
		}
	default:
		return nil
	}
}
