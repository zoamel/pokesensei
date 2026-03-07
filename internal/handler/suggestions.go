package handler

import (
	"context"
	"log/slog"
	"net/http"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/suggest"
	"zoamel/pokesensei/internal/view"
)

type SuggestionStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	ListTeamMembers(ctx context.Context, gameStateID int32) ([]generated.ListTeamMembersRow, error)
	ListAllPokemon(ctx context.Context) ([]generated.Pokemon, error)
	GetPokemonWithTypes(ctx context.Context, id int32) ([]generated.GetPokemonWithTypesRow, error)
	GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error)
	GetEvolutionChainByPokemon(ctx context.Context, pokemonID int32) ([]generated.GetEvolutionChainByPokemonRow, error)
	GetMinBadgeByPokemon(ctx context.Context, gameVersionID int32) ([]generated.GetMinBadgeByPokemonRow, error)
}

type SuggestionHandler struct {
	store  SuggestionStore
	engine *suggest.Engine
	log    *slog.Logger
}

func NewSuggestions(store SuggestionStore, log *slog.Logger) *SuggestionHandler {
	return &SuggestionHandler{
		store:  store,
		engine: suggest.New(),
		log:    log,
	}
}

// GET /team/suggestions — HTMX partial: side-by-side current vs planned
func (h *SuggestionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	team, _ := h.store.ListTeamMembers(ctx, gs.ID)
	allPokemon, _ := h.store.ListAllPokemon(ctx)
	efficacyRows, _ := h.store.GetTypeEfficacy(ctx)

	// Build efficacy map
	efficacy := make(map[int32]map[int32]int16)
	for _, e := range efficacyRows {
		if efficacy[e.AttackingTypeID] == nil {
			efficacy[e.AttackingTypeID] = make(map[int32]int16)
		}
		efficacy[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	// Build badge requirement lookup.
	// Pokémon with encounters get their minimum badge; Pokémon without encounters
	// are not catchable in the wild, so they get badge 9 (effectively unavailable).
	badgeMap := make(map[int32]int16)
	if gs.GameVersionID.Valid {
		badgeRows, _ := h.store.GetMinBadgeByPokemon(ctx, gs.GameVersionID.Int32)
		for _, row := range badgeRows {
			badgeMap[row.PokemonID] = row.MinBadge
		}
	}
	const uncatchableBadge int16 = 9

	// Build candidates with types
	candidates := make([]suggest.Pokemon, 0, len(allPokemon))
	for _, p := range allPokemon {
		types, _ := h.store.GetPokemonWithTypes(ctx, p.ID)
		badge, hasEncounter := badgeMap[p.ID]
		if !hasEncounter {
			badge = uncatchableBadge
		}
		candidate := suggest.Pokemon{
			ID:            p.ID,
			Name:          p.Name,
			SpriteURL:     p.SpriteUrl,
			BadgeRequired: badge,
		}
		for _, t := range types {
			candidate.Types = append(candidate.Types, t.TypeID)
		}

		// Check trade requirement from evolution chain
		evoChain, _ := h.store.GetEvolutionChainByPokemon(ctx, p.ID)
		for _, step := range evoChain {
			if step.PokemonID == p.ID && step.TradeRequired {
				candidate.TradeRequired = true
			}
		}

		candidates = append(candidates, candidate)
	}

	// Build current team for input
	currentTeam := make([]suggest.TeamSlot, 0)
	for _, m := range team {
		types, _ := h.store.GetPokemonWithTypes(ctx, m.PokemonID)
		var typeIDs []int32
		for _, t := range types {
			typeIDs = append(typeIDs, t.TypeID)
		}
		currentTeam = append(currentTeam, suggest.TeamSlot{
			Pokemon: &suggest.Pokemon{
				ID:        m.PokemonID,
				Name:      m.PokemonName,
				SpriteURL: m.SpriteUrl,
				Types:     typeIDs,
			},
			IsLocked: m.IsLocked,
			Slot:     int(m.Slot),
		})
	}

	// Build starter
	var starter *suggest.Pokemon
	if gs.StarterPokemonID.Valid {
		for _, c := range candidates {
			if c.ID == gs.StarterPokemonID.Int32 {
				cp := c
				starter = &cp
				break
			}
		}
	}

	input := suggest.SuggestionInput{
		Starter:        starter,
		CurrentTeam:    currentTeam,
		Candidates:     candidates,
		BadgeCount:     gs.BadgeCount,
		TradingEnabled: gs.TradingEnabled,
		Efficacy:       efficacy,
	}

	currentSuggestion := h.engine.SuggestCurrent(input)
	plannedSuggestion := h.engine.SuggestPlanned(input)

	if err := view.SuggestionsPartial(currentSuggestion, plannedSuggestion).Render(ctx, w); err != nil {
		h.log.Error("failed to render suggestions", "error", err)
	}
}
