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

	rootHandler := handler.NewRoot(queries, log)
	healthHandler := handler.NewHealth(queries, log)
	onboardingHandler := handler.NewOnboarding(queries, log)
	dashboardHandler := handler.NewDashboard(queries, log)
	settingsHandler := handler.NewSettings(queries, log)
	pokemonHandler := handler.NewPokemon(queries, log)
	teamHandler := handler.NewTeam(queries, log)
	suggestionHandler := handler.NewSuggestions(queries, log)
	battleHandler := handler.NewBattle(queries, log)
	guideHandler := handler.NewGuide(queries, log)

	// Configure server and routes
	srv := server.New(cfg, log)

	srv.Handle("GET /{$}", rootHandler)
	srv.Handle("GET /health", healthHandler)

	// Onboarding
	srv.Handle("GET /onboarding", onboardingHandler)
	srv.Handle("POST /onboarding/game", http.HandlerFunc(onboardingHandler.HandleGameStep))
	srv.Handle("POST /onboarding/starter", http.HandlerFunc(onboardingHandler.HandleStarterStep))
	srv.Handle("POST /onboarding/badge", http.HandlerFunc(onboardingHandler.HandleBadgeStep))

	// Dashboard
	srv.Handle("GET /dashboard", dashboardHandler)

	// Settings
	srv.Handle("GET /settings", settingsHandler)
	srv.Handle("PATCH /settings/game", http.HandlerFunc(settingsHandler.HandleGameUpdate))
	srv.Handle("PATCH /settings/starter", http.HandlerFunc(settingsHandler.HandleStarterUpdate))
	srv.Handle("PATCH /settings/badge", http.HandlerFunc(settingsHandler.HandleBadgeUpdate))
	srv.Handle("PATCH /settings/trading", http.HandlerFunc(settingsHandler.HandleTradingUpdate))

	// Pokémon Finder
	srv.Handle("GET /pokemon", pokemonHandler)
	srv.Handle("GET /pokemon/search", http.HandlerFunc(pokemonHandler.HandleSearch))
	srv.Handle("GET /pokemon/{id}", http.HandlerFunc(pokemonHandler.HandleDetail))

	// Team Builder
	srv.Handle("GET /team", teamHandler)
	srv.Handle("POST /team/members", http.HandlerFunc(teamHandler.HandleAdd))
	srv.Handle("DELETE /team/members/{id}", http.HandlerFunc(teamHandler.HandleRemove))
	srv.Handle("PATCH /team/members/{id}", http.HandlerFunc(teamHandler.HandleUpdate))
	srv.Handle("GET /team/coverage", http.HandlerFunc(teamHandler.HandleCoverage))
	srv.Handle("GET /team/suggestions", suggestionHandler)

	// Battle Helper
	srv.Handle("GET /battle", battleHandler)
	srv.Handle("GET /battle/trainer/{id}", http.HandlerFunc(battleHandler.HandleTrainerMatchup))
	srv.Handle("GET /battle/pokemon/{id}", http.HandlerFunc(battleHandler.HandlePokemonMatchup))
	srv.Handle("GET /battle/search", http.HandlerFunc(battleHandler.HandleSearch))

	// Basics Guide
	srv.Handle("GET /guide", guideHandler)
	srv.Handle("GET /guide/types", http.HandlerFunc(guideHandler.HandleTypes))
	srv.Handle("GET /guide/natures", http.HandlerFunc(guideHandler.HandleNatures))
	srv.Handle("GET /guide/abilities", http.HandlerFunc(guideHandler.HandleAbilities))
	srv.Handle("GET /guide/abilities/search", http.HandlerFunc(guideHandler.HandleAbilitySearch))
	srv.Handle("GET /guide/evs-ivs", http.HandlerFunc(guideHandler.HandleEVsIVs))
	srv.Handle("GET /guide/status", http.HandlerFunc(guideHandler.HandleStatus))
	srv.Handle("GET /guide/moves", http.HandlerFunc(guideHandler.HandleMoves))
	srv.Handle("GET /guide/basics", http.HandlerFunc(guideHandler.HandleBasics))
	srv.Handle("GET /guide/catching", http.HandlerFunc(guideHandler.HandleCatching))
	srv.Handle("GET /guide/gym-tips", http.HandlerFunc(guideHandler.HandleGymTips))
	srv.Handle("GET /guide/recommended", http.HandlerFunc(guideHandler.HandleRecommended))
	srv.Handle("GET /guide/items", http.HandlerFunc(guideHandler.HandleItems))
	srv.Handle("GET /guide/mechanics/{game}", http.HandlerFunc(guideHandler.HandleMechanics))

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
