package handler

import (
	"context"
	"database/sql"
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
	ListTeamMembers(ctx context.Context, gameStateID int64) ([]generated.ListTeamMembersRow, error)
	AddTeamMember(ctx context.Context, arg generated.AddTeamMemberParams) (generated.AddTeamMemberRow, error)
	RemoveTeamMember(ctx context.Context, id int64) error
	UpdateTeamMemberLevel(ctx context.Context, arg generated.UpdateTeamMemberLevelParams) error
	UpdateTeamMemberLock(ctx context.Context, arg generated.UpdateTeamMemberLockParams) error
	GetPokemonWithTypes(ctx context.Context, id int64) ([]generated.GetPokemonWithTypesRow, error)
	ListTypes(ctx context.Context) ([]generated.Type, error)
	GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error)
	GetTeamMemberDetail(ctx context.Context, id int64) (generated.GetTeamMemberDetailRow, error)
	ListNatures(ctx context.Context) ([]generated.Nature, error)
	ListPokemonAbilities(ctx context.Context, pokemonID int64) ([]generated.ListPokemonAbilitiesRow, error)
	ListTeamMemberMoves(ctx context.Context, teamMemberID int64) ([]generated.ListTeamMemberMovesRow, error)
	ListAvailableMoves(ctx context.Context, arg generated.ListAvailableMovesParams) ([]generated.ListAvailableMovesRow, error)
	GetVersionGroupIDByGameVersion(ctx context.Context, id int64) (sql.NullInt64, error)
	SetTeamMemberNature(ctx context.Context, arg generated.SetTeamMemberNatureParams) error
	SetTeamMemberAbility(ctx context.Context, arg generated.SetTeamMemberAbilityParams) error
	AddTeamMemberMove(ctx context.Context, arg generated.AddTeamMemberMoveParams) (generated.TeamMemberMove, error)
	RemoveTeamMemberMove(ctx context.Context, id int64) error
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

	pokemonID, err := strconv.ParseInt(r.FormValue("pokemon_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid Pokémon ID", http.StatusBadRequest)
		return
	}

	level := int64(5)
	if l, err := strconv.Atoi(r.FormValue("level")); err == nil && l >= 1 && l <= 100 {
		level = int64(l)
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
	rows, err := h.store.GetPokemonWithTypes(ctx, pokemonID)
	if err != nil || len(rows) == 0 {
		h.log.Error("failed to get pokemon", "error", err)
		http.Error(w, "Pokémon not found", http.StatusBadRequest)
		return
	}
	name := rows[0].Name

	if _, err := h.store.AddTeamMember(ctx, generated.AddTeamMemberParams{
		GameStateID: gs.ID,
		PokemonID:   pokemonID,
		Level:       level,
		Slot:        int64(slot),
		IsLocked:    0,
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	if err := h.store.RemoveTeamMember(r.Context(), id); err != nil {
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

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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
			Level: int64(level),
			ID:    id,
		}); err != nil {
			h.log.Error("failed to update level", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	if lockStr := r.FormValue("is_locked"); lockStr != "" {
		locked := lockStr == "true" || lockStr == "on"
		var lockedVal int64
		if locked {
			lockedVal = 1
		}
		if err := h.store.UpdateTeamMemberLock(ctx, generated.UpdateTeamMemberLockParams{
			IsLocked: lockedVal,
			ID:       id,
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
	efficacyMap := make(map[int64]map[int64]int64)
	for _, e := range efficacy {
		if efficacyMap[e.AttackingTypeID] == nil {
			efficacyMap[e.AttackingTypeID] = make(map[int64]int64)
		}
		efficacyMap[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	// Get team types
	teamTypes := make([][]int64, 0)
	for _, m := range team {
		pokemonTypes, _ := h.store.GetPokemonWithTypes(ctx, m.PokemonID)
		var typeIDs []int64
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

// GET /team/members/{id} — team member detail page
func (h *TeamHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	member, err := h.store.GetTeamMemberDetail(ctx, id)
	if err != nil {
		h.log.Error("failed to get team member detail", "id", id, "error", err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	types, err := h.store.GetPokemonWithTypes(ctx, member.PokemonID)
	if err != nil {
		h.log.Error("failed to get pokemon types", "error", err)
	}
	var typeInfos []view.TypeInfo
	for _, t := range types {
		typeInfos = append(typeInfos, view.TypeInfo{ID: t.TypeID, Name: t.TypeName, Slug: t.TypeSlug})
	}

	natures, err := h.store.ListNatures(ctx)
	if err != nil {
		h.log.Error("failed to list natures", "error", err)
	}

	abilities, err := h.store.ListPokemonAbilities(ctx, member.PokemonID)
	if err != nil {
		h.log.Error("failed to list abilities", "error", err)
	}

	moves, err := h.store.ListTeamMemberMoves(ctx, id)
	if err != nil {
		h.log.Error("failed to list team member moves", "error", err)
	}

	var available []generated.ListAvailableMovesRow
	gs, err := h.store.GetGameState(ctx)
	if err == nil && gs.GameVersionID.Valid {
		vgID, err := h.store.GetVersionGroupIDByGameVersion(ctx, gs.GameVersionID.Int64)
		if err == nil && vgID.Valid {
			available, err = h.store.ListAvailableMoves(ctx, generated.ListAvailableMovesParams{
				PokemonID:      member.PokemonID,
				VersionGroupID: vgID.Int64,
				LevelLearnedAt: member.Level,
			})
			if err != nil {
				h.log.Error("failed to list available moves", "error", err)
			}
		}
	}

	assignedIDs := make(map[int64]bool)
	for _, m := range moves {
		assignedIDs[m.MoveID] = true
	}

	data := view.TeamMemberDetailData{
		Member:          member,
		Types:           typeInfos,
		Natures:         natures,
		Abilities:       abilities,
		Moves:           moves,
		Available:       available,
		AssignedMoveIDs: assignedIDs,
	}

	if err := view.TeamMemberDetailPage(data).Render(ctx, w); err != nil {
		h.log.Error("failed to render team member detail", "error", err)
	}
}

// PATCH /team/members/{id}/nature — set or clear nature
func (h *TeamHandler) HandleSetNature(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	var natureID sql.NullInt64
	if v := r.FormValue("nature_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			http.Error(w, "Invalid nature_id", http.StatusBadRequest)
			return
		}
		natureID = sql.NullInt64{Int64: n, Valid: true}
	}

	if err := h.store.SetTeamMemberNature(ctx, generated.SetTeamMemberNatureParams{
		NatureID: natureID,
		ID:       id,
	}); err != nil {
		h.log.Error("failed to set nature", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	member, err := h.store.GetTeamMemberDetail(ctx, id)
	if err != nil {
		h.log.Error("failed to re-fetch member after nature set", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := view.TeamMemberStatSummary(member).Render(ctx, w); err != nil {
		h.log.Error("failed to render stat summary", "error", err)
	}
}

// PATCH /team/members/{id}/ability — set or clear ability
func (h *TeamHandler) HandleSetAbility(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	var abilityID sql.NullInt64
	if v := r.FormValue("ability_id"); v != "" {
		a, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ability_id", http.StatusBadRequest)
			return
		}
		abilityID = sql.NullInt64{Int64: a, Valid: true}
	}

	if err := h.store.SetTeamMemberAbility(ctx, generated.SetTeamMemberAbilityParams{
		AbilityID: abilityID,
		ID:        id,
	}); err != nil {
		h.log.Error("failed to set ability", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// POST /team/members/{id}/moves — add a move to the team member
func (h *TeamHandler) HandleAddMove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	moveID, err := strconv.ParseInt(r.FormValue("move_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid move_id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	moves, err := h.store.ListTeamMemberMoves(ctx, id)
	if err != nil {
		h.log.Error("failed to list moves", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	occupied := make(map[int64]bool)
	for _, m := range moves {
		occupied[m.Slot] = true
	}

	var slot int64
	for s := int64(1); s <= 4; s++ {
		if !occupied[s] {
			slot = s
			break
		}
	}

	if slot == 0 {
		http.Error(w, "No available move slots", http.StatusConflict)
		return
	}

	if _, err := h.store.AddTeamMemberMove(ctx, generated.AddTeamMemberMoveParams{
		TeamMemberID: id,
		MoveID:       moveID,
		Slot:         slot,
	}); err != nil {
		h.log.Error("failed to add move", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderMovesSection(w, r, id)
}

// DELETE /team/members/{id}/moves/{tmMoveId} — remove a move from the team member
func (h *TeamHandler) HandleRemoveMove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	tmMoveID, err := strconv.ParseInt(r.PathValue("tmMoveId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid move ID", http.StatusBadRequest)
		return
	}

	if err := h.store.RemoveTeamMemberMove(r.Context(), tmMoveID); err != nil {
		h.log.Error("failed to remove move", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderMovesSection(w, r, id)
}

// GET /team/members/{id}/moves — HTMX partial: moves section refresh
func (h *TeamHandler) HandleMovesPartial(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	h.renderMovesSection(w, r, id)
}

// renderMovesSection re-fetches all move data and renders the moves section partial.
func (h *TeamHandler) renderMovesSection(w http.ResponseWriter, r *http.Request, memberID int64) {
	ctx := r.Context()

	member, err := h.store.GetTeamMemberDetail(ctx, memberID)
	if err != nil {
		h.log.Error("failed to get member for moves section", "error", err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	moves, err := h.store.ListTeamMemberMoves(ctx, memberID)
	if err != nil {
		h.log.Error("failed to list team member moves", "error", err)
	}

	var available []generated.ListAvailableMovesRow
	gs, err := h.store.GetGameState(ctx)
	if err == nil && gs.GameVersionID.Valid {
		vgID, err := h.store.GetVersionGroupIDByGameVersion(ctx, gs.GameVersionID.Int64)
		if err == nil && vgID.Valid {
			available, err = h.store.ListAvailableMoves(ctx, generated.ListAvailableMovesParams{
				PokemonID:      member.PokemonID,
				VersionGroupID: vgID.Int64,
				LevelLearnedAt: member.Level,
			})
			if err != nil {
				h.log.Error("failed to list available moves", "error", err)
			}
		}
	}

	assignedIDs := make(map[int64]bool)
	for _, m := range moves {
		assignedIDs[m.MoveID] = true
	}

	if err := view.TeamMemberMovesSection(memberID, moves, available, assignedIDs).Render(ctx, w); err != nil {
		h.log.Error("failed to render moves section", "error", err)
	}
}

// setHXTrigger encodes multiple HTMX trigger events as a single JSON header.
func setHXTrigger(w http.ResponseWriter, events map[string]any) {
	b, _ := json.Marshal(events)
	w.Header().Set("HX-Trigger", string(b))
}

// computeCoverage returns a map of defender type ID -> best coverage factor from team.
// Factor > 100 means the team has a super-effective option.
func computeCoverage(types []generated.Type, teamTypes [][]int64, efficacy map[int64]map[int64]int64) map[int64]int64 {
	coverage := make(map[int64]int64)
	for _, t := range types {
		best := int64(0)
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
