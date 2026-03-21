package handler

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"zoamel/pokesensei/db/generated"
)

// mockTeamStore implements TeamStore with controllable return values.
type mockTeamStore struct {
	getTeamMemberDetailRow generated.GetTeamMemberDetailRow
	getTeamMemberDetailErr error
	listNatures            []generated.Nature
	listPokemonAbilities   []generated.ListPokemonAbilitiesRow
	listTeamMemberMoves    []generated.ListTeamMemberMovesRow
	listAvailableMoves     []generated.ListAvailableMovesRow
	setNatureErr           error
	setAbilityErr          error
	addTeamMemberMoveErr   error
	removeTeamMemberMoveErr error
	gameState              generated.GameState
	pokemonTypes           []generated.GetPokemonWithTypesRow
	versionGroupID         sql.NullInt64
}

func (m *mockTeamStore) GetGameState(ctx context.Context) (generated.GameState, error) {
	return m.gameState, nil
}

func (m *mockTeamStore) ListTeamMembers(ctx context.Context, gameStateID int64) ([]generated.ListTeamMembersRow, error) {
	return nil, nil
}

func (m *mockTeamStore) AddTeamMember(ctx context.Context, arg generated.AddTeamMemberParams) (generated.AddTeamMemberRow, error) {
	return generated.AddTeamMemberRow{}, nil
}

func (m *mockTeamStore) RemoveTeamMember(ctx context.Context, id int64) error {
	return nil
}

func (m *mockTeamStore) UpdateTeamMemberLevel(ctx context.Context, arg generated.UpdateTeamMemberLevelParams) error {
	return nil
}

func (m *mockTeamStore) UpdateTeamMemberLock(ctx context.Context, arg generated.UpdateTeamMemberLockParams) error {
	return nil
}

func (m *mockTeamStore) GetPokemonWithTypes(ctx context.Context, id int64) ([]generated.GetPokemonWithTypesRow, error) {
	return m.pokemonTypes, nil
}

func (m *mockTeamStore) ListTypes(ctx context.Context) ([]generated.Type, error) {
	return nil, nil
}

func (m *mockTeamStore) GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error) {
	return nil, nil
}

func (m *mockTeamStore) GetTeamMemberDetail(ctx context.Context, id int64) (generated.GetTeamMemberDetailRow, error) {
	return m.getTeamMemberDetailRow, m.getTeamMemberDetailErr
}

func (m *mockTeamStore) ListNatures(ctx context.Context) ([]generated.Nature, error) {
	return m.listNatures, nil
}

func (m *mockTeamStore) ListPokemonAbilities(ctx context.Context, pokemonID int64) ([]generated.ListPokemonAbilitiesRow, error) {
	return m.listPokemonAbilities, nil
}

func (m *mockTeamStore) ListTeamMemberMoves(ctx context.Context, teamMemberID int64) ([]generated.ListTeamMemberMovesRow, error) {
	return m.listTeamMemberMoves, nil
}

func (m *mockTeamStore) ListAvailableMoves(ctx context.Context, arg generated.ListAvailableMovesParams) ([]generated.ListAvailableMovesRow, error) {
	return m.listAvailableMoves, nil
}

func (m *mockTeamStore) GetVersionGroupIDByGameVersion(ctx context.Context, id int64) (sql.NullInt64, error) {
	return m.versionGroupID, nil
}

func (m *mockTeamStore) SetTeamMemberNature(ctx context.Context, arg generated.SetTeamMemberNatureParams) error {
	return m.setNatureErr
}

func (m *mockTeamStore) SetTeamMemberAbility(ctx context.Context, arg generated.SetTeamMemberAbilityParams) error {
	return m.setAbilityErr
}

func (m *mockTeamStore) AddTeamMemberMove(ctx context.Context, arg generated.AddTeamMemberMoveParams) (generated.TeamMemberMove, error) {
	return generated.TeamMemberMove{}, m.addTeamMemberMoveErr
}

func (m *mockTeamStore) RemoveTeamMemberMove(ctx context.Context, id int64) error {
	return m.removeTeamMemberMoveErr
}

// helpers

func newTeamHandler(store *mockTeamStore) *TeamHandler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewTeam(store, logger)
}

func validMemberDetail() generated.GetTeamMemberDetailRow {
	return generated.GetTeamMemberDetailRow{
		ID:          42,
		PokemonID:   1,
		Level:       50,
		Slot:        1,
		PokemonName: "Bulbasaur",
		PokemonSlug: "bulbasaur",
	}
}

func validGameState() generated.GameState {
	return generated.GameState{
		ID:            1,
		GameVersionID: sql.NullInt64{Int64: 1, Valid: true},
	}
}

// TestHandleDetail_Success: valid id, mock returns member. Expect 200 and pokemon name in body.
func TestHandleDetail_Success(t *testing.T) {
	store := &mockTeamStore{
		getTeamMemberDetailRow: validMemberDetail(),
		gameState:              validGameState(),
		versionGroupID:         sql.NullInt64{Int64: 1, Valid: true},
		pokemonTypes: []generated.GetPokemonWithTypesRow{
			{TypeID: 12, TypeName: "Grass", TypeSlug: "grass"},
		},
	}
	h := newTeamHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/team/members/42", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.HandleDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty body")
	}
	if !strings.Contains(body, "Bulbasaur") {
		t.Errorf("body does not contain pokemon name %q", "Bulbasaur")
	}
}

// TestHandleDetail_InvalidID: non-numeric id. Expect 400.
func TestHandleDetail_InvalidID(t *testing.T) {
	h := newTeamHandler(&mockTeamStore{})

	req := httptest.NewRequest(http.MethodGet, "/team/members/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	h.HandleDetail(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestHandleDetail_NotFound: store returns an error. Expect 404.
func TestHandleDetail_NotFound(t *testing.T) {
	store := &mockTeamStore{
		getTeamMemberDetailErr: errors.New("not found"),
	}
	h := newTeamHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/team/members/99", nil)
	req.SetPathValue("id", "99")
	rec := httptest.NewRecorder()

	h.HandleDetail(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestHandleSetNature_Success: valid id with nature_id form value. Expect 200.
func TestHandleSetNature_Success(t *testing.T) {
	store := &mockTeamStore{
		getTeamMemberDetailRow: validMemberDetail(),
	}
	h := newTeamHandler(store)

	form := url.Values{"nature_id": {"1"}}
	req := httptest.NewRequest(http.MethodPatch, "/team/members/42/nature", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.HandleSetNature(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestHandleSetNature_Clear: empty nature_id clears the nature. Expect 200.
func TestHandleSetNature_Clear(t *testing.T) {
	store := &mockTeamStore{
		getTeamMemberDetailRow: validMemberDetail(),
	}
	h := newTeamHandler(store)

	form := url.Values{"nature_id": {""}}
	req := httptest.NewRequest(http.MethodPatch, "/team/members/42/nature", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.HandleSetNature(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestHandleAddMove_Success: empty move list, post move_id. Expect 200.
func TestHandleAddMove_Success(t *testing.T) {
	store := &mockTeamStore{
		listTeamMemberMoves:    []generated.ListTeamMemberMovesRow{},
		getTeamMemberDetailRow: validMemberDetail(),
		gameState:              validGameState(),
		versionGroupID:         sql.NullInt64{Int64: 1, Valid: true},
	}
	h := newTeamHandler(store)

	form := url.Values{"move_id": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/team/members/42/moves", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.HandleAddMove(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestHandleAddMove_Full: 4 moves already assigned. Expect 409.
func TestHandleAddMove_Full(t *testing.T) {
	store := &mockTeamStore{
		listTeamMemberMoves: []generated.ListTeamMemberMovesRow{
			{ID: 1, Slot: 1, MoveID: 10},
			{ID: 2, Slot: 2, MoveID: 20},
			{ID: 3, Slot: 3, MoveID: 30},
			{ID: 4, Slot: 4, MoveID: 40},
		},
	}
	h := newTeamHandler(store)

	form := url.Values{"move_id": {"5"}}
	req := httptest.NewRequest(http.MethodPost, "/team/members/42/moves", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()

	h.HandleAddMove(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

// TestHandleRemoveMove_Success: valid ids, removal succeeds. Expect 200.
func TestHandleRemoveMove_Success(t *testing.T) {
	store := &mockTeamStore{
		getTeamMemberDetailRow: validMemberDetail(),
		gameState:              validGameState(),
		versionGroupID:         sql.NullInt64{Int64: 1, Valid: true},
	}
	h := newTeamHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/team/members/42/moves/7", nil)
	req.SetPathValue("id", "42")
	req.SetPathValue("tmMoveId", "7")
	rec := httptest.NewRecorder()

	h.HandleRemoveMove(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestHandleRemoveMove_InvalidID: non-numeric tmMoveId. Expect 400.
func TestHandleRemoveMove_InvalidID(t *testing.T) {
	h := newTeamHandler(&mockTeamStore{})

	req := httptest.NewRequest(http.MethodDelete, "/team/members/42/moves/abc", nil)
	req.SetPathValue("id", "42")
	req.SetPathValue("tmMoveId", "abc")
	rec := httptest.NewRecorder()

	h.HandleRemoveMove(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
