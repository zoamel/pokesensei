package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/gamecontext"
	"zoamel/pokesensei/internal/view"
)

type PokemonStore interface {
	SearchPokemonFiltered(ctx context.Context, arg generated.SearchPokemonFilteredParams) ([]generated.Pokemon, error)
	ListTypes(ctx context.Context) ([]generated.Type, error)
	GetPokemonByID(ctx context.Context, id int64) (generated.Pokemon, error)
	GetPokemonWithTypes(ctx context.Context, id int64) ([]generated.GetPokemonWithTypesRow, error)
	ListPokemonAbilities(ctx context.Context, pokemonID int64) ([]generated.ListPokemonAbilitiesRow, error)
	GetEvolutionChainByPokemon(ctx context.Context, pokemonID int64) ([]generated.GetEvolutionChainByPokemonRow, error)
	ListPokemonMoves(ctx context.Context, arg generated.ListPokemonMovesParams) ([]generated.ListPokemonMovesRow, error)
	ListEncountersByPokemon(ctx context.Context, arg generated.ListEncountersByPokemonParams) ([]generated.ListEncountersByPokemonRow, error)
}

type PokemonHandler struct {
	store PokemonStore
	log   *slog.Logger
}

func NewPokemon(store PokemonStore, log *slog.Logger) *PokemonHandler {
	return &PokemonHandler{store: store, log: log}
}

// GET /pokemon — finder page with filter sidebar
func (h *PokemonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	types, err := h.store.ListTypes(ctx)
	if err != nil {
		h.log.Error("failed to list types", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	gc, _ := gamecontext.FromRequest(r)
	gs := generated.GameState{
		ID:             gc.GameStateID,
		GameVersionID:  sql.NullInt64{Int64: gc.GameVersionID, Valid: true},
		BadgeCount:     gc.BadgeCount,
		TradingEnabled: boolToInt64(gc.TradingEnabled),
	}

	if err := view.PokemonFinderPage(types, gs).Render(ctx, w); err != nil {
		h.log.Error("failed to render pokemon finder", "error", err)
	}
}

// GET /pokemon/search — HTMX partial with debounced search results
func (h *PokemonHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gc, _ := gamecontext.FromRequest(r)

	params := generated.SearchPokemonFilteredParams{
		MaxPokedex: gc.MaxPokedex,
	}

	if name := r.URL.Query().Get("name"); name != "" {
		params.Name = sql.NullString{String: name, Valid: true}
	}
	if typeID := r.URL.Query().Get("type_id"); typeID != "" {
		if id, err := strconv.Atoi(typeID); err == nil && id > 0 {
			params.TypeID = sql.NullInt64{Int64: int64(id), Valid: true}
		}
	}
	if badge := r.URL.Query().Get("max_badge"); badge != "" {
		if b, err := strconv.Atoi(badge); err == nil {
			params.MaxBadge = sql.NullInt64{Int64: int64(b), Valid: true}
		}
	}
	if gvID := r.URL.Query().Get("game_version_id"); gvID != "" {
		if id, err := strconv.Atoi(gvID); err == nil {
			params.GameVersionID = sql.NullInt64{Int64: int64(id), Valid: true}
		}
	}

	results, err := h.store.SearchPokemonFiltered(ctx, params)
	if err != nil {
		h.log.Error("search failed", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get types for each result
	pokemonWithTypes := make([]view.PokemonListItem, 0, len(results))
	for _, p := range results {
		types, _ := h.store.GetPokemonWithTypes(ctx, p.ID)
		item := view.PokemonListItem{Pokemon: p}
		for _, t := range types {
			item.Types = append(item.Types, view.TypeInfo{
				ID:   t.TypeID,
				Name: t.TypeName,
				Slug: t.TypeSlug,
			})
		}
		pokemonWithTypes = append(pokemonWithTypes, item)
	}

	if err := view.PokemonSearchResults(pokemonWithTypes).Render(ctx, w); err != nil {
		h.log.Error("failed to render search results", "error", err)
	}
}

// GET /pokemon/{id} — detail view
func (h *PokemonHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid Pokémon ID", http.StatusBadRequest)
		return
	}

	pokemon, err := h.store.GetPokemonByID(ctx, id)
	if err != nil {
		http.Error(w, "Pokémon not found", http.StatusNotFound)
		return
	}

	gc, _ := gamecontext.FromRequest(r)

	types, _ := h.store.GetPokemonWithTypes(ctx, id)
	abilities, _ := h.store.ListPokemonAbilities(ctx, id)
	evoChain, _ := h.store.GetEvolutionChainByPokemon(ctx, id)
	moves, _ := h.store.ListPokemonMoves(ctx, generated.ListPokemonMovesParams{
		PokemonID:      id,
		VersionGroupID: gc.VersionGroupID,
	})
	encounters, _ := h.store.ListEncountersByPokemon(ctx, generated.ListEncountersByPokemonParams{
		PokemonID:     id,
		GameVersionID: gc.GameVersionID,
	})

	detail := view.PokemonDetail{
		Pokemon:        pokemon,
		Abilities:      abilities,
		EvolutionChain: evoChain,
		Moves:          moves,
		Encounters:     encounters,
	}
	for _, t := range types {
		detail.Types = append(detail.Types, view.TypeInfo{
			ID:   t.TypeID,
			Name: t.TypeName,
			Slug: t.TypeSlug,
		})
	}

	if err := view.PokemonDetailPage(detail).Render(ctx, w); err != nil {
		h.log.Error("failed to render pokemon detail", "error", err)
	}
}
