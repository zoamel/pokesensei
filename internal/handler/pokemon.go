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

type PokemonStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	SearchPokemonFiltered(ctx context.Context, arg generated.SearchPokemonFilteredParams) ([]generated.Pokemon, error)
	ListTypes(ctx context.Context) ([]generated.Type, error)
	GetPokemonByID(ctx context.Context, id int32) (generated.Pokemon, error)
	GetPokemonWithTypes(ctx context.Context, id int32) ([]generated.GetPokemonWithTypesRow, error)
	ListPokemonAbilities(ctx context.Context, pokemonID int32) ([]generated.ListPokemonAbilitiesRow, error)
	ListEncountersByPokemon(ctx context.Context, arg generated.ListEncountersByPokemonParams) ([]generated.ListEncountersByPokemonRow, error)
	GetEvolutionChainByPokemon(ctx context.Context, pokemonID int32) ([]generated.GetEvolutionChainByPokemonRow, error)
	ListPokemonMoves(ctx context.Context, arg generated.ListPokemonMovesParams) ([]generated.ListPokemonMovesRow, error)
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

	gs, _ := h.store.GetGameState(ctx)

	if err := view.PokemonFinderPage(types, gs).Render(ctx, w); err != nil {
		h.log.Error("failed to render pokemon finder", "error", err)
	}
}

// GET /pokemon/search — HTMX partial with debounced search results
func (h *PokemonHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := generated.SearchPokemonFilteredParams{}

	if name := r.URL.Query().Get("name"); name != "" {
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if typeID := r.URL.Query().Get("type_id"); typeID != "" {
		if id, err := strconv.Atoi(typeID); err == nil && id > 0 {
			params.TypeID = pgtype.Int4{Int32: int32(id), Valid: true}
		}
	}
	if badge := r.URL.Query().Get("max_badge"); badge != "" {
		if b, err := strconv.Atoi(badge); err == nil {
			params.MaxBadge = pgtype.Int2{Int16: int16(b), Valid: true}
		}
	}
	if gvID := r.URL.Query().Get("game_version_id"); gvID != "" {
		if id, err := strconv.Atoi(gvID); err == nil {
			params.GameVersionID = pgtype.Int4{Int32: int32(id), Valid: true}
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
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid Pokémon ID", http.StatusBadRequest)
		return
	}

	pokemon, err := h.store.GetPokemonByID(ctx, int32(id))
	if err != nil {
		http.Error(w, "Pokémon not found", http.StatusNotFound)
		return
	}

	types, _ := h.store.GetPokemonWithTypes(ctx, int32(id))
	abilities, _ := h.store.ListPokemonAbilities(ctx, int32(id))
	evoChain, _ := h.store.GetEvolutionChainByPokemon(ctx, int32(id))

	gs, _ := h.store.GetGameState(ctx)

	var encounters []generated.ListEncountersByPokemonRow
	if gs.GameVersionID.Valid {
		encounters, _ = h.store.ListEncountersByPokemon(ctx, generated.ListEncountersByPokemonParams{
			PokemonID:     int32(id),
			GameVersionID: gs.GameVersionID.Int32,
		})
	}

	// Get moves for current game version group
	var moves []generated.ListPokemonMovesRow
	if gs.GameVersionID.Valid {
		vgID := versionGroupForGame(gs.GameVersionID.Int32)
		moves, _ = h.store.ListPokemonMoves(ctx, generated.ListPokemonMovesParams{
			PokemonID:      int32(id),
			VersionGroupID: int32(vgID),
		})
	}

	detail := view.PokemonDetail{
		Pokemon:    pokemon,
		Encounters: encounters,
		EvoChain:   evoChain,
		Abilities:  abilities,
		Moves:      moves,
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

func versionGroupForGame(gameVersionID int32) int {
	switch gameVersionID {
	case 10, 11:
		return 7 // FRLG
	case 15, 16:
		return 10 // HGSS
	default:
		return 7
	}
}
