package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/view"
)

type TeamStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	ListTeamMembers(ctx context.Context, gameStateID int32) ([]generated.ListTeamMembersRow, error)
	AddTeamMember(ctx context.Context, arg generated.AddTeamMemberParams) (generated.TeamMember, error)
	RemoveTeamMember(ctx context.Context, id int32) error
	UpdateTeamMemberLevel(ctx context.Context, arg generated.UpdateTeamMemberLevelParams) error
	UpdateTeamMemberLock(ctx context.Context, arg generated.UpdateTeamMemberLockParams) error
	GetPokemonWithTypes(ctx context.Context, id int32) ([]generated.GetPokemonWithTypesRow, error)
	ListTypes(ctx context.Context) ([]generated.Type, error)
	GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error)
}

type TeamHandler struct {
	store TeamStore
	log   *slog.Logger
}

func NewTeam(store TeamStore, log *slog.Logger) *TeamHandler {
	return &TeamHandler{store: store, log: log}
}

// GET /team — team builder page
func (h *TeamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
	}

	team, err := h.store.ListTeamMembers(ctx, gs.ID)
	if err != nil {
		h.log.Error("failed to list team", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get types for each team member
	slots := make([]view.TeamSlotData, 6)
	for i := range slots {
		slotNum := i + 1
		for j := range team {
			if int(team[j].Slot) == slotNum {
				member := team[j]
				slot := view.TeamSlotData{Member: &member}
				types, _ := h.store.GetPokemonWithTypes(ctx, member.PokemonID)
				for _, t := range types {
					slot.Types = append(slot.Types, view.TypeInfo{
						ID:   t.TypeID,
						Name: t.TypeName,
						Slug: t.TypeSlug,
					})
				}
				slots[i] = slot
				break
			}
		}
	}

	if err := view.TeamBuilderPage(gs, slots).Render(ctx, w); err != nil {
		h.log.Error("failed to render team builder", "error", err)
	}
}

// POST /team/members — add a Pokémon to the team
func (h *TeamHandler) HandleAdd(w http.ResponseWriter, r *http.Request) {
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

	pokemonID, err := strconv.Atoi(r.FormValue("pokemon_id"))
	if err != nil {
		http.Error(w, "Invalid Pokémon ID", http.StatusBadRequest)
		return
	}

	level := 5
	if l, err := strconv.Atoi(r.FormValue("level")); err == nil && l >= 1 && l <= 100 {
		level = l
	}

	// Find next available slot
	team, err := h.store.ListTeamMembers(ctx, gs.ID)
	if err != nil {
		h.log.Error("failed to list team", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	occupied := make(map[int]bool)
	for _, m := range team {
		occupied[int(m.Slot)] = true
	}

	slot := 0
	for s := 1; s <= 6; s++ {
		if !occupied[s] {
			slot = s
			break
		}
	}

	if slot == 0 {
		setHXTrigger(w, map[string]any{
			"show-toast": map[string]string{"message": "Team is full (max 6)", "variant": "error"},
		})
		w.WriteHeader(http.StatusConflict)
		return
	}

	// Fetch Pokémon name for the toast message
	rows, err := h.store.GetPokemonWithTypes(ctx, int32(pokemonID))
	if err != nil || len(rows) == 0 {
		h.log.Error("failed to get pokemon", "error", err)
		http.Error(w, "Pokémon not found", http.StatusBadRequest)
		return
	}
	name := rows[0].Name

	if _, err := h.store.AddTeamMember(ctx, generated.AddTeamMemberParams{
		GameStateID: gs.ID,
		PokemonID:   int32(pokemonID),
		Level:       int16(level),
		Slot:        int16(slot),
		IsLocked:    false,
	}); err != nil {
		h.log.Error("failed to add team member", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	setHXTrigger(w, map[string]any{
		"team-updated": nil,
		"show-toast":   map[string]string{"message": fmt.Sprintf("%s added to team!", name), "variant": "success"},
	})
	w.WriteHeader(http.StatusCreated)
}

// DELETE /team/members/{id} — remove from team
func (h *TeamHandler) HandleRemove(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	if err := h.store.RemoveTeamMember(r.Context(), int32(id)); err != nil {
		h.log.Error("failed to remove team member", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "team-updated")
	w.WriteHeader(http.StatusOK)
}

// PATCH /team/members/{id} — update level or lock status
func (h *TeamHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if levelStr := r.FormValue("level"); levelStr != "" {
		level, err := strconv.Atoi(levelStr)
		if err != nil || level < 1 || level > 100 {
			http.Error(w, "Invalid level", http.StatusBadRequest)
			return
		}
		if err := h.store.UpdateTeamMemberLevel(ctx, generated.UpdateTeamMemberLevelParams{
			Level: int16(level),
			ID:    int32(id),
		}); err != nil {
			h.log.Error("failed to update level", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	if lockStr := r.FormValue("is_locked"); lockStr != "" {
		locked := lockStr == "true" || lockStr == "on"
		if err := h.store.UpdateTeamMemberLock(ctx, generated.UpdateTeamMemberLockParams{
			IsLocked: locked,
			ID:       int32(id),
		}); err != nil {
			h.log.Error("failed to update lock", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", "team-updated")
	w.WriteHeader(http.StatusOK)
}

// GET /team/coverage — HTMX partial: type coverage matrix
func (h *TeamHandler) HandleCoverage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gs, err := h.store.GetGameState(ctx)
	if err != nil {
		http.Error(w, "No game state", http.StatusBadRequest)
		return
	}

	team, err := h.store.ListTeamMembers(ctx, gs.ID)
	if err != nil {
		h.log.Error("failed to list team", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

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

	// Build efficacy lookup: [attacker][defender] = factor
	efficacyMap := make(map[int32]map[int32]int16)
	for _, e := range efficacy {
		if efficacyMap[e.AttackingTypeID] == nil {
			efficacyMap[e.AttackingTypeID] = make(map[int32]int16)
		}
		efficacyMap[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	// Get team types
	teamTypes := make([][]int32, 0)
	for _, m := range team {
		pokemonTypes, _ := h.store.GetPokemonWithTypes(ctx, m.PokemonID)
		var typeIDs []int32
		for _, pt := range pokemonTypes {
			typeIDs = append(typeIDs, pt.TypeID)
		}
		teamTypes = append(teamTypes, typeIDs)
	}

	coverage := computeCoverage(types, teamTypes, efficacyMap)
	if err := view.TeamCoveragePartial(types, coverage).Render(ctx, w); err != nil {
		h.log.Error("failed to render coverage", "error", err)
	}
}

// setHXTrigger encodes multiple HTMX trigger events as a single JSON header.
func setHXTrigger(w http.ResponseWriter, events map[string]any) {
	b, _ := json.Marshal(events)
	w.Header().Set("HX-Trigger", string(b))
}

// computeCoverage returns a map of defender type ID -> best coverage factor from team.
// Factor > 100 means the team has a super-effective option.
func computeCoverage(types []generated.Type, teamTypes [][]int32, efficacy map[int32]map[int32]int16) map[int32]int16 {
	coverage := make(map[int32]int16)
	for _, t := range types {
		best := int16(0)
		for _, memberTypes := range teamTypes {
			for _, atkType := range memberTypes {
				if factor, ok := efficacy[atkType][t.ID]; ok && factor > best {
					best = factor
				}
			}
		}
		coverage[t.ID] = best
	}
	return coverage
}
