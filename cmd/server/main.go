package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zoamel/pokesensei/db"
	"zoamel/pokesensei/db/generated"
	"zoamel/pokesensei/internal/config"
	"zoamel/pokesensei/internal/database"
	"zoamel/pokesensei/internal/gamecontext"
	"zoamel/pokesensei/internal/handler"
	"zoamel/pokesensei/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Setup structured logger
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	// Run database migrations
	if err := database.RunMigrations(cfg.DatabasePath, db.EmbedMigrations); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Info("migrations completed")

	// Create database connection
	sqlDB, err := database.NewDB(ctx, cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("creating database connection: %w", err)
	}
	defer sqlDB.Close()
	log.Info("database connected")

	// Wire dependencies
	queries := generated.New(sqlDB)

	withGame := gamecontext.Middleware(queries, log)

	healthHandler := handler.NewHealth(queries, log)
	onboardingHandler := handler.NewOnboarding(queries, log)
	dashboardHandler := handler.NewDashboard(queries, log)
	settingsHandler := handler.NewSettings(queries, log)
	pokemonHandler := handler.NewPokemon(queries, log)
	teamHandler := handler.NewTeam(queries, log)
	suggestionHandler := handler.NewSuggestions(queries, log)
	battleHandler := handler.NewBattle(queries, log)

	// Configure server and routes
	srv := server.New(cfg, log)

	// Root redirect — middleware handles onboarding redirect for unauthenticated users
	srv.Handle("GET /{$}", http.RedirectHandler("/dashboard", http.StatusSeeOther))
	srv.Handle("GET /health", healthHandler)

	// Onboarding (no game context middleware — user may not have a game yet)
	srv.Handle("GET /onboarding", onboardingHandler)
	srv.Handle("POST /onboarding/game", http.HandlerFunc(onboardingHandler.HandleGameStep))
	srv.Handle("POST /onboarding/starter", http.HandlerFunc(onboardingHandler.HandleStarterStep))
	srv.Handle("POST /onboarding/badge", http.HandlerFunc(onboardingHandler.HandleBadgeStep))

	// Dashboard
	srv.Handle("GET /dashboard", withGame(dashboardHandler))

	// Settings
	srv.Handle("GET /settings", withGame(settingsHandler))
	srv.Handle("PATCH /settings/game", withGame(http.HandlerFunc(settingsHandler.HandleGameUpdate)))
	srv.Handle("PATCH /settings/starter", withGame(http.HandlerFunc(settingsHandler.HandleStarterUpdate)))
	srv.Handle("PATCH /settings/badge", withGame(http.HandlerFunc(settingsHandler.HandleBadgeUpdate)))
	srv.Handle("PATCH /settings/trading", withGame(http.HandlerFunc(settingsHandler.HandleTradingUpdate)))

	// Pokémon Finder
	srv.Handle("GET /pokemon", withGame(pokemonHandler))
	srv.Handle("GET /pokemon/search", withGame(http.HandlerFunc(pokemonHandler.HandleSearch)))
	srv.Handle("GET /pokemon/{id}", withGame(http.HandlerFunc(pokemonHandler.HandleDetail)))

	// Team Builder
	srv.Handle("GET /team", withGame(teamHandler))
	srv.Handle("POST /team/members", withGame(http.HandlerFunc(teamHandler.HandleAdd)))
	srv.Handle("DELETE /team/members/{id}", withGame(http.HandlerFunc(teamHandler.HandleRemove)))
	srv.Handle("PATCH /team/members/{id}", withGame(http.HandlerFunc(teamHandler.HandleUpdate)))
	srv.Handle("GET /team/members/{id}", withGame(http.HandlerFunc(teamHandler.HandleDetail)))
	srv.Handle("PATCH /team/members/{id}/nature", withGame(http.HandlerFunc(teamHandler.HandleSetNature)))
	srv.Handle("PATCH /team/members/{id}/ability", withGame(http.HandlerFunc(teamHandler.HandleSetAbility)))
	srv.Handle("POST /team/members/{id}/moves", withGame(http.HandlerFunc(teamHandler.HandleAddMove)))
	srv.Handle("DELETE /team/members/{id}/moves/{tmMoveId}", withGame(http.HandlerFunc(teamHandler.HandleRemoveMove)))
	srv.Handle("GET /team/members/{id}/moves", withGame(http.HandlerFunc(teamHandler.HandleMovesPartial)))
	srv.Handle("GET /team/coverage", withGame(http.HandlerFunc(teamHandler.HandleCoverage)))
	srv.Handle("GET /team/suggestions", withGame(suggestionHandler))

	// Battle Helper
	srv.Handle("GET /battle", withGame(battleHandler))
	srv.Handle("GET /battle/trainer/{id}", withGame(http.HandlerFunc(battleHandler.HandleTrainerMatchup)))
	srv.Handle("GET /battle/pokemon/{id}", withGame(http.HandlerFunc(battleHandler.HandlePokemonMatchup)))
	srv.Handle("GET /battle/search", withGame(http.HandlerFunc(battleHandler.HandleSearch)))

	// Type chart
	srv.Handle("GET /guide/types", http.RedirectHandler("/battle/types", http.StatusMovedPermanently))
	srv.Handle("GET /battle/types", withGame(http.HandlerFunc(battleHandler.HandleTypeChart)))

	// Static files
	srv.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Graceful shutdown on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		log.Info("shutting down gracefully")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("server stopped")
	return nil
}
