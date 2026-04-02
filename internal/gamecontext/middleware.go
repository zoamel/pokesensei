package gamecontext

import (
	"context"
	"log/slog"
	"net/http"

	"zoamel/pokesensei/db/generated"
)

// Store is the database interface the middleware needs.
type Store interface {
	GetActiveGameContext(ctx context.Context) (generated.GetActiveGameContextRow, error)
}

// Middleware loads the active game context from the database and injects it
// into the request context. If no active game exists, it redirects to /onboarding.
func Middleware(store Store, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			row, err := store.GetActiveGameContext(r.Context())
			if err != nil {
				log.Debug("no active game context", "error", err)
				http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
				return
			}

			gc := GameContext{
				GameStateID:    row.ID,
				GameVersionID:  row.GameVersionID.Int64,
				VersionGroupID: row.VersionGroupID.Int64,
				Generation:     row.Generation,
				TypeChartEra:   row.TypeChartEra,
				BadgeCount:     row.BadgeCount,
				TradingEnabled: row.TradingEnabled == 1,
				MaxPokedex:     row.MaxPokedex,
				MaxBadges:      row.MaxBadges,
			}

			ctx := NewContext(r.Context(), gc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
