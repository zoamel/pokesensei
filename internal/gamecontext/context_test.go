package gamecontext

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestFromRequest_Present(t *testing.T) {
	want := GameContext{
		GameStateID:    1,
		GameVersionID:  10,
		VersionGroupID: 7,
		Generation:     3,
		TypeChartEra:   "pre_fairy",
		BadgeCount:     3,
		TradingEnabled: false,
		MaxPokedex:     386,
		MaxBadges:      8,
	}

	ctx := NewContext(context.Background(), want)
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	got, ok := FromRequest(r)
	if !ok {
		t.Fatal("expected GameContext in request")
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFromRequest_Missing(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)

	_, ok := FromRequest(r)
	if ok {
		t.Fatal("expected no GameContext in request")
	}
}
