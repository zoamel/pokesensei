package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"zoamel/pokesensei/db"
	"zoamel/pokesensei/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		databaseURL  string
		games        string
		maxDex       int
		seedTrainers bool
	)

	flag.StringVar(&databaseURL, "database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	flag.StringVar(&games, "games", "frlg,hgss", "Comma-separated game groups to import (frlg, hgss)")
	flag.IntVar(&maxDex, "max-dex", 493, "Maximum national dex number to import (default: 493 for Gen IV)")
	flag.BoolVar(&seedTrainers, "seed-trainers", false, "Import trainer seed data from db/seed/ JSON files")
	flag.Parse()

	if databaseURL == "" {
		return fmt.Errorf("--database-url or DATABASE_URL is required")
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx := context.Background()

	// Run migrations first
	if err := database.RunMigrations(databaseURL, db.EmbedMigrations); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Info("migrations completed")

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("creating pool: %w", err)
	}
	defer pool.Close()

	client := NewPokeAPIClient(log)
	importer := NewImporter(pool, client, log)

	// Parse game groups
	gameGroups := strings.Split(games, ",")
	for i := range gameGroups {
		gameGroups[i] = strings.TrimSpace(gameGroups[i])
	}

	// Always import shared data first
	log.Info("importing shared data (types, natures)")
	if err := importer.ImportTypes(ctx); err != nil {
		return fmt.Errorf("importing types: %w", err)
	}
	if err := importer.ImportTypeEfficacy(ctx); err != nil {
		return fmt.Errorf("importing type efficacy: %w", err)
	}
	if err := importer.ImportNatures(ctx); err != nil {
		return fmt.Errorf("importing natures: %w", err)
	}

	// Import game versions
	if err := importer.ImportGameVersions(ctx); err != nil {
		return fmt.Errorf("importing game versions: %w", err)
	}

	// Import Pokémon data (collects ability IDs but defers pokemon_abilities inserts)
	log.Info("importing Pokémon", "max_dex", maxDex)
	if err := importer.ImportPokemon(ctx, maxDex); err != nil {
		return fmt.Errorf("importing pokemon: %w", err)
	}

	// Import abilities (uses abilitySeen collected during Pokémon import)
	if err := importer.ImportAbilities(ctx); err != nil {
		return fmt.Errorf("importing abilities: %w", err)
	}

	// Now insert pokemon_abilities (abilities table is populated)
	if err := importer.FlushPokemonAbilities(ctx); err != nil {
		return fmt.Errorf("flushing pokemon abilities: %w", err)
	}

	// Import moves and learnsets per game group
	for _, group := range gameGroups {
		vg, ok := VersionGroups[group]
		if !ok {
			return fmt.Errorf("unknown game group: %s (valid: frlg, hgss)", group)
		}
		log.Info("importing game-specific data", "group", group, "version_group_id", vg.VersionGroupID)

		if err := importer.ImportMoves(ctx); err != nil {
			return fmt.Errorf("importing moves for %s: %w", group, err)
		}

		if err := importer.ImportLearnsets(ctx, maxDex, vg.VersionGroupID); err != nil {
			return fmt.Errorf("importing learnsets for %s: %w", group, err)
		}

		if err := importer.ImportEncounters(ctx, vg); err != nil {
			return fmt.Errorf("importing encounters for %s: %w", group, err)
		}
	}

	// Import evolution chains
	if err := importer.ImportEvolutionChains(ctx, maxDex); err != nil {
		return fmt.Errorf("importing evolution chains: %w", err)
	}

	// Seed trainer data from JSON files
	if seedTrainers {
		seedImporter := NewSeedImporter(pool, log)
		seedFiles := map[string]string{
			"frlg": "db/seed/frlg_trainers.json",
			"hgss": "db/seed/hgss_trainers.json",
		}
		for _, group := range gameGroups {
			if seedFile, ok := seedFiles[group]; ok {
				log.Info("seeding trainer data", "file", seedFile)
				if err := seedImporter.ImportTrainersFromFile(ctx, seedFile); err != nil {
					return fmt.Errorf("seeding trainers from %s: %w", seedFile, err)
				}
			}
		}
	}

	log.Info("import complete")
	return nil
}
