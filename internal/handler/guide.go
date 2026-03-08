package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/view"
)

type GuideStore interface {
	ListTypes(ctx context.Context) ([]generated.Type, error)
	GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error)
	ListNatures(ctx context.Context) ([]generated.Nature, error)
	ListAbilities(ctx context.Context) ([]generated.Ability, error)
	SearchAbilities(ctx context.Context, dollar1 pgtype.Text) ([]generated.Ability, error)
}

type GuideHandler struct {
	store GuideStore
	log   *slog.Logger
}

func NewGuide(store GuideStore, log *slog.Logger) *GuideHandler {
	return &GuideHandler{store: store, log: log}
}

// GET /guide — index page
func (h *GuideHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideIndexPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render guide index", "error", err)
	}
}

// GET /guide/types — interactive type chart
func (h *GuideHandler) HandleTypes(w http.ResponseWriter, r *http.Request) {
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

	// Build matrix: [attacker_id][defender_id] = factor
	matrix := make(map[int32]map[int32]int16)
	for _, e := range efficacy {
		if matrix[e.AttackingTypeID] == nil {
			matrix[e.AttackingTypeID] = make(map[int32]int16)
		}
		matrix[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	if err := view.GuideTypesPage(types, matrix).Render(ctx, w); err != nil {
		h.log.Error("failed to render type chart", "error", err)
	}
}

// GET /guide/natures — natures table
func (h *GuideHandler) HandleNatures(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	natures, err := h.store.ListNatures(ctx)
	if err != nil {
		h.log.Error("failed to list natures", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.GuideNaturesPage(natures).Render(ctx, w); err != nil {
		h.log.Error("failed to render natures", "error", err)
	}
}

// GET /guide/abilities — abilities list
func (h *GuideHandler) HandleAbilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	abilities, err := h.store.ListAbilities(ctx)
	if err != nil {
		h.log.Error("failed to list abilities", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.GuideAbilitiesPage(abilities).Render(ctx, w); err != nil {
		h.log.Error("failed to render abilities", "error", err)
	}
}

// GET /guide/abilities/search — HTMX partial
func (h *GuideHandler) HandleAbilitySearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("q")
	if query == "" {
		abilities, _ := h.store.ListAbilities(ctx)
		if err := view.AbilityListPartial(abilities).Render(ctx, w); err != nil {
			h.log.Error("failed to render ability list", "error", err)
		}
		return
	}

	abilities, err := h.store.SearchAbilities(ctx, pgtype.Text{String: query, Valid: true})
	if err != nil {
		h.log.Error("ability search failed", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.AbilityListPartial(abilities).Render(ctx, w); err != nil {
		h.log.Error("failed to render ability search", "error", err)
	}
}

// GET /guide/evs-ivs — static content
func (h *GuideHandler) HandleEVsIVs(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideEVsIVsPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render EVs/IVs guide", "error", err)
	}
}

// GET /guide/status — static content
func (h *GuideHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideStatusPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render status guide", "error", err)
	}
}

// GET /guide/moves — static content
func (h *GuideHandler) HandleMoves(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideMovesPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render moves guide", "error", err)
	}
}

// GET /guide/mechanics/{game} — static content
func (h *GuideHandler) HandleMechanics(w http.ResponseWriter, r *http.Request) {
	game := r.PathValue("game")
	if err := view.GuideMechanicsPage(game).Render(r.Context(), w); err != nil {
		h.log.Error("failed to render mechanics guide", "error", err)
	}
}

// GET /guide/basics — how Pokémon works tutorial
func (h *GuideHandler) HandleBasics(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideBasicsPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render basics guide", "error", err)
	}
}

// GET /guide/catching — catching Pokémon guide
func (h *GuideHandler) HandleCatching(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideCatchingPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render catching guide", "error", err)
	}
}

// GET /guide/gym-tips — per-gym beginner strategy tips
func (h *GuideHandler) HandleGymTips(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideGymTipsPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render gym tips guide", "error", err)
	}
}

// GET /guide/recommended — recommended Pokémon list
func (h *GuideHandler) HandleRecommended(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideRecommendedPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render recommended guide", "error", err)
	}
}

// GET /guide/items — item basics guide
func (h *GuideHandler) HandleItems(w http.ResponseWriter, r *http.Request) {
	if err := view.GuideItemsPage().Render(r.Context(), w); err != nil {
		h.log.Error("failed to render items guide", "error", err)
	}
}
