package gamecontext

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"zoamel/pokesensei/db/generated"
)

type mockStore struct {
	row generated.GetActiveGameContextRow
	err error
}

func (m *mockStore) GetActiveGameContext(ctx context.Context) (generated.GetActiveGameContextRow, error) {
	return m.row, m.err
}

var silentLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestMiddleware_InjectsGameContext(t *testing.T) {
	store := &mockStore{
		row: generated.GetActiveGameContextRow{
			ID:             1,
			GameVersionID:  sql.NullInt64{Int64: 10, Valid: true},
			BadgeCount:     3,
			TradingEnabled: 0,
			VersionGroupID: sql.NullInt64{Int64: 7, Valid: true},
			Generation:     3,
			MaxPokedex:     386,
			TypeChartEra:   "pre_fairy",
			MaxBadges:      8,
		},
	}

	var captured GameContext
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gc, ok := FromRequest(r)
		if !ok {
			t.Fatal("expected GameContext")
		}
		captured = gc
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(store, silentLog)(inner)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if captured.GameStateID != 1 {
		t.Errorf("GameStateID = %d, want 1", captured.GameStateID)
	}
	if captured.TypeChartEra != "pre_fairy" {
		t.Errorf("TypeChartEra = %q, want %q", captured.TypeChartEra, "pre_fairy")
	}
	if captured.MaxPokedex != 386 {
		t.Errorf("MaxPokedex = %d, want 386", captured.MaxPokedex)
	}
}

func TestMiddleware_RedirectsWhenNoGameState(t *testing.T) {
	store := &mockStore{err: errors.New("sql: no rows in result set")}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	handler := Middleware(store, silentLog)(inner)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/onboarding" {
		t.Errorf("location = %q, want /onboarding", loc)
	}
}
