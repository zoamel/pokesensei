package gamecontext

import (
	"context"
	"net/http"
)

type contextKey struct{}

// GameContext holds the active game state resolved by the middleware.
type GameContext struct {
	GameStateID    int64
	GameVersionID  int64
	VersionGroupID int64
	Generation     int64
	TypeChartEra   string
	BadgeCount     int64
	TradingEnabled bool
	MaxPokedex     int64
	MaxBadges      int64
}

// NewContext returns a new context with the given GameContext attached.
func NewContext(ctx context.Context, gc GameContext) context.Context {
	return context.WithValue(ctx, contextKey{}, gc)
}

// FromRequest extracts the GameContext from a request's context.
// Returns the zero value and false if no GameContext is set.
func FromRequest(r *http.Request) (GameContext, bool) {
	gc, ok := r.Context().Value(contextKey{}).(GameContext)
	return gc, ok
}
