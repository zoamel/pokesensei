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
	ListPokemonMoves(ctx context.Context, arg generated.ListPokemonMovesParams) ([]generated.ListPokemonMovesRow, error)
	ListAvailableMoves(ctx context.Context, arg generated.ListAvailableMovesParams) ([]generated.ListAvailableMovesRow, error)
	ListStartersByGame(ctx context.Context, gameVersionID int64) ([]int64, error)
	GetActiveGameState(ctx context.Context) (generated.GameState, error)
	GetPokemonWithFieldMoves(ctx context.Context, versionGroupID int64) ([]int64, error)
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

	// Determine the player's chosen starter and build an exclusion set of the
	// other starters (and their evolutions), since starter trios are mutually
	// exclusive in the main games.
	starterIDs, _ := h.store.ListStartersByGame(ctx, gc.GameVersionID)
	chosenStarterID := h.determineChosenStarter(ctx, team, starterIDs)
	excludedStarters := h.buildStarterExclusion(ctx, starterIDs, chosenStarterID)

	// Build efficacy map
	efficacy := make(map[int64]map[int64]int64)
	for _, e := range efficacyRows {
		if efficacy[e.AttackingTypeID] == nil {
			efficacy[e.AttackingTypeID] = make(map[int64]int64)
		}
		efficacy[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	// Load Pokémon IDs that can learn at least one field/HM move in this game version.
	utilityRows, _ := h.store.GetPokemonWithFieldMoves(ctx, gc.VersionGroupID)
	utilityIDs := make(map[int64]bool, len(utilityRows))
	for _, id := range utilityRows {
		utilityIDs[id] = true
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
		if excludedStarters[p.ID] {
			continue
		}
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
		CurrentTeam:       currentTeam,
		Candidates:        candidates,
		BadgeCount:        gc.BadgeCount,
		TradingEnabled:    gc.TradingEnabled,
		Efficacy:          efficacy,
		UtilityPokemonIDs: utilityIDs,
	}

	currentSuggestion := h.engine.SuggestCurrent(input)
	plannedSuggestion := h.engine.SuggestPlanned(input)

	// Determine an "average-ish" level for current suggestions based on the user's
	// existing team. Default to 5 when the team is empty.
	suggestLevel := averageTeamLevel(team)

	h.attachMoves(ctx, &currentSuggestion, gc.VersionGroupID, suggestLevel, true)
	h.attachMoves(ctx, &plannedSuggestion, gc.VersionGroupID, suggestLevel, false)

	h.engine.SelectMovesForResult(&currentSuggestion, efficacy)
	h.engine.SelectMovesForResult(&plannedSuggestion, efficacy)

	if err := view.SuggestionsPartial(currentSuggestion, plannedSuggestion).Render(ctx, w); err != nil {
		h.log.Error("failed to render suggestions", "error", err)
	}
}

// determineChosenStarter returns the Pokémon ID of the player's starter.
//
// Resolution order:
//  1. If the active game state has `starter_pokemon_id` set, use it.
//  2. Otherwise, inspect team slot 1. Walk its evolution chain back to the root;
//     if the root is one of the game's starters, use it.
//  3. If nothing can be determined, return 0 (no starter — no exclusions apply).
func (h *SuggestionHandler) determineChosenStarter(ctx context.Context, team []generated.ListTeamMembersRow, starterIDs []int64) int64 {
	if len(starterIDs) == 0 {
		return 0
	}

	if gs, err := h.store.GetActiveGameState(ctx); err == nil && gs.StarterPokemonID.Valid {
		return gs.StarterPokemonID.Int64
	}

	if len(team) == 0 {
		return 0
	}

	// Team is ordered by slot; find slot 1.
	var slot1ID int64
	for _, m := range team {
		if m.Slot == 1 {
			slot1ID = m.PokemonID
			break
		}
	}
	if slot1ID == 0 {
		return 0
	}

	// Walk the evolution chain to the root (the Pokémon with no predecessor).
	chain, err := h.store.GetEvolutionChainByPokemon(ctx, slot1ID)
	if err != nil || len(chain) == 0 {
		return 0
	}
	var rootID int64
	for _, step := range chain {
		if !step.EvolvesFromID.Valid {
			rootID = step.PokemonID
			break
		}
	}
	if rootID == 0 {
		return 0
	}

	// Only treat it as the chosen starter if it's actually in this game's starter group.
	for _, sid := range starterIDs {
		if sid == rootID {
			return rootID
		}
	}
	return 0
}

// buildStarterExclusion returns a set of Pokémon IDs (starter + all evolutions)
// that should be filtered out of candidates because the player can't obtain
// them — they picked a different starter.
func (h *SuggestionHandler) buildStarterExclusion(ctx context.Context, starterIDs []int64, chosen int64) map[int64]bool {
	excluded := make(map[int64]bool)
	if chosen == 0 {
		// No chosen starter known — don't exclude anything, preserve old behaviour.
		return excluded
	}
	for _, sid := range starterIDs {
		if sid == chosen {
			continue
		}
		chain, err := h.store.GetEvolutionChainByPokemon(ctx, sid)
		if err != nil {
			continue
		}
		for _, step := range chain {
			excluded[step.PokemonID] = true
		}
	}
	return excluded
}

// averageTeamLevel returns the rounded average level of the current team, or 5 if empty.
func averageTeamLevel(team []generated.ListTeamMembersRow) int64 {
	if len(team) == 0 {
		return 5
	}
	var total int64
	for _, m := range team {
		total += m.Level
	}
	return total / int64(len(team))
}

// attachMoves loads the learnable moves for each picked Pokémon and attaches
// them to the slot's Pokémon. When levelAware is true the level-restricted
// query is used; otherwise the full learnset is loaded.
func (h *SuggestionHandler) attachMoves(ctx context.Context, result *suggest.SuggestionResult, versionGroupID, level int64, levelAware bool) {
	for i := range result.Slots {
		p := result.Slots[i].Pokemon
		if p == nil {
			continue
		}
		p.AvailableMoves = h.loadMovesForPokemon(ctx, p.ID, versionGroupID, level, levelAware)
	}
}

func (h *SuggestionHandler) loadMovesForPokemon(ctx context.Context, pokemonID, versionGroupID, level int64, levelAware bool) []suggest.CandidateMove {
	if levelAware {
		rows, err := h.store.ListAvailableMoves(ctx, generated.ListAvailableMovesParams{
			PokemonID:      pokemonID,
			VersionGroupID: versionGroupID,
			LevelLearnedAt: level,
		})
		if err != nil {
			h.log.Error("failed to load available moves", "pokemon_id", pokemonID, "error", err)
			return nil
		}
		return convertAvailableMoves(rows)
	}

	rows, err := h.store.ListPokemonMoves(ctx, generated.ListPokemonMovesParams{
		PokemonID:      pokemonID,
		VersionGroupID: versionGroupID,
	})
	if err != nil {
		h.log.Error("failed to load pokemon moves", "pokemon_id", pokemonID, "error", err)
		return nil
	}
	return convertPokemonMoves(rows)
}

func convertAvailableMoves(rows []generated.ListAvailableMovesRow) []suggest.CandidateMove {
	moves := make([]suggest.CandidateMove, 0, len(rows))
	for _, r := range rows {
		moves = append(moves, suggest.CandidateMove{
			ID:          r.ID,
			Name:        r.Name,
			Slug:        r.Slug,
			TypeID:      r.TypeID.Int64,
			TypeName:    r.TypeName.String,
			TypeSlug:    r.TypeSlug.String,
			Power:       r.Power.Int64,
			DamageClass: r.DamageClass,
		})
	}
	return moves
}

func convertPokemonMoves(rows []generated.ListPokemonMovesRow) []suggest.CandidateMove {
	moves := make([]suggest.CandidateMove, 0, len(rows))
	for _, r := range rows {
		moves = append(moves, suggest.CandidateMove{
			ID:          r.MoveID,
			Name:        r.Name,
			Slug:        r.Slug,
			TypeID:      r.TypeID.Int64,
			TypeName:    r.TypeName.String,
			TypeSlug:    r.TypeSlug.String,
			Power:       r.Power.Int64,
			DamageClass: r.DamageClass,
		})
	}
	return moves
}
