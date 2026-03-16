package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/matchup"
	"zoamel/pokesensei/internal/view"
)

type BattleStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	ListTeamMembers(ctx context.Context, gameStateID int64) ([]generated.ListTeamMembersRow, error)
	ListTrainersByGame(ctx context.Context, gameVersionID int64) ([]generated.Trainer, error)
	GetTrainerByID(ctx context.Context, id int64) (generated.Trainer, error)
	ListTrainerPokemon(ctx context.Context, trainerID int64) ([]generated.ListTrainerPokemonRow, error)
	ListTrainerPokemonMoves(ctx context.Context, trainerPokemonID int64) ([]generated.ListTrainerPokemonMovesRow, error)
	GetPokemonByID(ctx context.Context, id int64) (generated.Pokemon, error)
	GetPokemonWithTypes(ctx context.Context, id int64) ([]generated.GetPokemonWithTypesRow, error)
	ListPokemonMovesAtLevel(ctx context.Context, arg generated.ListPokemonMovesAtLevelParams) ([]generated.ListPokemonMovesAtLevelRow, error)
	GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error)
	ListTypes(ctx context.Context) ([]generated.Type, error)
	SearchPokemonFiltered(ctx context.Context, arg generated.SearchPokemonFilteredParams) ([]generated.Pokemon, error)
}

type BattleHandler struct {
	store  BattleStore
	engine *matchup.Engine
	log    *slog.Logger
}

func NewBattle(store BattleStore, log *slog.Logger) *BattleHandler {
	return &BattleHandler{
		store:  store,
		engine: matchup.New(),
		log:    log,
	}
}

// GET /battle — battle helper page with trainer list
func (h *BattleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}

	var trainers []generated.Trainer
	if gs.GameVersionID.Valid {
		trainers, _ = h.store.ListTrainersByGame(ctx, gs.GameVersionID.Int64)
	}

	if err := view.BattlePage(trainers).Render(ctx, w); err != nil {
		h.log.Error("failed to render battle page", "error", err)
	}
}

// GET /battle/trainer/{id} — HTMX partial: trainer roster with matchups
func (h *BattleHandler) HandleTrainerMatchup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	trainerID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid trainer ID", http.StatusBadRequest)
		return
	}

	trainer, err := h.store.GetTrainerByID(ctx, trainerID)
	if err != nil {
		http.Error(w, "Trainer not found", http.StatusNotFound)
		return
	}

	trainerPokemon, err := h.store.ListTrainerPokemon(ctx, trainerID)
	if err != nil {
		h.log.Error("failed to list trainer pokemon", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	team, efficacy := h.loadTeamAndEfficacy(ctx)

	// Build matchups for each trainer Pokémon
	var matchups []view.BattleMatchup
	for _, tp := range trainerPokemon {
		types, _ := h.store.GetPokemonWithTypes(ctx, tp.PokemonID)
		var typeIDs []int64
		for _, t := range types {
			typeIDs = append(typeIDs, t.TypeID)
		}

		opponent := matchup.Pokemon{
			ID:        tp.PokemonID,
			Name:      tp.PokemonName,
			SpriteURL: tp.SpriteUrl,
			Types:     typeIDs,
			Level:     tp.Level,
		}

		results := h.engine.RankTeam(team, h.loadTeamMoves(ctx, team), opponent, efficacy)

		matchups = append(matchups, view.BattleMatchup{
			Opponent: opponent,
			Rankings: results,
		})
	}

	if err := view.TrainerMatchupPartial(trainer, matchups).Render(ctx, w); err != nil {
		h.log.Error("failed to render trainer matchup", "error", err)
	}
}

// GET /battle/pokemon/{id} — HTMX partial: single pokemon matchup
func (h *BattleHandler) HandlePokemonMatchup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pokemonID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid Pokémon ID", http.StatusBadRequest)
		return
	}

	pokemon, err := h.store.GetPokemonByID(ctx, pokemonID)
	if err != nil {
		http.Error(w, "Pokémon not found", http.StatusNotFound)
		return
	}

	types, _ := h.store.GetPokemonWithTypes(ctx, pokemonID)
	var typeIDs []int64
	for _, t := range types {
		typeIDs = append(typeIDs, t.TypeID)
	}

	opponent := matchup.Pokemon{
		ID:        pokemon.ID,
		Name:      pokemon.Name,
		SpriteURL: pokemon.SpriteUrl,
		Types:     typeIDs,
	}

	team, efficacy := h.loadTeamAndEfficacy(ctx)
	results := h.engine.RankTeam(team, h.loadTeamMoves(ctx, team), opponent, efficacy)

	mu := view.BattleMatchup{
		Opponent: opponent,
		Rankings: results,
	}

	if err := view.PokemonMatchupPartial(mu).Render(ctx, w); err != nil {
		h.log.Error("failed to render pokemon matchup", "error", err)
	}
}

// GET /battle/search — HTMX partial: pokemon search for matchup
func (h *BattleHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.URL.Query().Get("name")
	if name == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	params := generated.SearchPokemonFilteredParams{
		Name: sql.NullString{String: name, Valid: true},
	}

	results, err := h.store.SearchPokemonFiltered(ctx, params)
	if err != nil {
		h.log.Error("search failed", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.BattleSearchResults(results).Render(ctx, w); err != nil {
		h.log.Error("failed to render battle search", "error", err)
	}
}

func (h *BattleHandler) loadTeamAndEfficacy(ctx context.Context) ([]matchup.Pokemon, map[int64]map[int64]int64) {
	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		return nil, nil
	}

	members, _ := h.store.ListTeamMembers(ctx, gs.ID)
	var team []matchup.Pokemon
	for _, m := range members {
		types, _ := h.store.GetPokemonWithTypes(ctx, m.PokemonID)
		var typeIDs []int64
		for _, t := range types {
			typeIDs = append(typeIDs, t.TypeID)
		}
		team = append(team, matchup.Pokemon{
			ID:        m.PokemonID,
			Name:      m.PokemonName,
			SpriteURL: m.SpriteUrl,
			Types:     typeIDs,
			Level:     m.Level,
		})
	}

	efficacyRows, _ := h.store.GetTypeEfficacy(ctx)
	efficacy := make(map[int64]map[int64]int64)
	for _, e := range efficacyRows {
		if efficacy[e.AttackingTypeID] == nil {
			efficacy[e.AttackingTypeID] = make(map[int64]int64)
		}
		efficacy[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	return team, efficacy
}

func (h *BattleHandler) loadTeamMoves(ctx context.Context, team []matchup.Pokemon) map[int64][]matchup.Move {
	gs, _ := h.store.GetGameState(ctx)
	vgID := int64(7)
	if gs.GameVersionID.Valid {
		vgID = int64(versionGroupForGame(gs.GameVersionID.Int64))
	}

	moves := make(map[int64][]matchup.Move)
	for _, member := range team {
		level := member.Level
		if level == 0 {
			level = 50
		}
		rows, _ := h.store.ListPokemonMovesAtLevel(ctx, generated.ListPokemonMovesAtLevelParams{
			PokemonID:      member.ID,
			VersionGroupID: vgID,
			LevelLearnedAt: level,
		})
		for _, r := range rows {
			power := int64(0)
			if r.Power.Valid {
				power = r.Power.Int64
			}
			var typeID int64
			if r.TypeID.Valid {
				typeID = r.TypeID.Int64
			}
			moves[member.ID] = append(moves[member.ID], matchup.Move{
				ID:          r.MoveID,
				Name:        r.Name,
				TypeID:      typeID,
				Power:       power,
				DamageClass: r.DamageClass,
			})
		}
	}
	return moves
}

// GET /battle/types — interactive type chart
func (h *BattleHandler) HandleTypeChart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	types, err := h.store.ListTypes(ctx)
	if err != nil {
		h.log.Error("failed to list types", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	efficacy, err := h.store.GetTypeEfficacy(ctx)
	if err != nil {
		h.log.Error("failed to get efficacy", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	matrix := make(map[int64]map[int64]int64)
	for _, e := range efficacy {
		if matrix[e.AttackingTypeID] == nil {
			matrix[e.AttackingTypeID] = make(map[int64]int64)
		}
		matrix[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	if err := view.TypeChartPage(types, matrix).Render(ctx, w); err != nil {
		h.log.Error("failed to render type chart", "error", err)
	}
}
