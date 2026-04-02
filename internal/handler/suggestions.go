package handler

import (
	"context"
	"log/slog"
	"net/http"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/gamecontext"
	"zoamel/pokesensei/internal/suggest"
	"zoamel/pokesensei/internal/view"
)

type SuggestionStore interface {
	ListTeamMembers(ctx context.Context, gameStateID int64) ([]generated.ListTeamMembersRow, error)
	ListAllPokemon(ctx context.Context, maxPokedex int64) ([]generated.Pokemon, error)
	GetPokemonWithTypes(ctx context.Context, id int64) ([]generated.GetPokemonWithTypesRow, error)
	GetTypeEfficacyByEra(ctx context.Context, era string) ([]generated.GetTypeEfficacyByEraRow, error)
	GetEvolutionChainByPokemon(ctx context.Context, pokemonID int64) ([]generated.GetEvolutionChainByPokemonRow, error)
	GetMinBadgeByPokemon(ctx context.Context, gameVersionID int64) ([]generated.GetMinBadgeByPokemonRow, error)
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

	gc, _ := gamecontext.FromRequest(r)

	team, _ := h.store.ListTeamMembers(ctx, gc.GameStateID)
	allPokemon, _ := h.store.ListAllPokemon(ctx, gc.MaxPokedex)
	efficacyRows, _ := h.store.GetTypeEfficacyByEra(ctx, gc.TypeChartEra)

	// Build efficacy map
	efficacy := make(map[int64]map[int64]int64)
	for _, e := range efficacyRows {
		if efficacy[e.AttackingTypeID] == nil {
			efficacy[e.AttackingTypeID] = make(map[int64]int64)
		}
		efficacy[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	// Build badge requirement lookup.
	// Pokémon with encounters get their minimum badge; Pokémon without encounters
	// are not catchable in the wild, so they get badge 9 (effectively unavailable).
	badgeMap := make(map[int64]int64)
	badgeRows, _ := h.store.GetMinBadgeByPokemon(ctx, gc.GameVersionID)
	for _, row := range badgeRows {
		badgeMap[row.PokemonID] = row.MinBadge
	}
	const uncatchableBadge int64 = 9

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
			if step.PokemonID == p.ID && step.TradeRequired != 0 {
				candidate.TradeRequired = true
			}
		}

		candidates = append(candidates, candidate)
	}

	// Build current team for input
	currentTeam := make([]suggest.TeamSlot, 0)
	for _, m := range team {
		types, _ := h.store.GetPokemonWithTypes(ctx, m.PokemonID)
		var typeIDs []int64
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
			IsLocked: m.IsLocked != 0,
			Slot:     int(m.Slot),
		})
	}

	input := suggest.SuggestionInput{
		CurrentTeam:    currentTeam,
		Candidates:     candidates,
		BadgeCount:     gc.BadgeCount,
		TradingEnabled: gc.TradingEnabled,
		Efficacy:       efficacy,
	}

	currentSuggestion := h.engine.SuggestCurrent(input)
	plannedSuggestion := h.engine.SuggestPlanned(input)

	if err := view.SuggestionsPartial(currentSuggestion, plannedSuggestion).Render(ctx, w); err != nil {
		h.log.Error("failed to render suggestions", "error", err)
	}
}
