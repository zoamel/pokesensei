package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"zoamel/pokesensei/db"
	"zoamel/pokesensei/db/generated"
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
		databasePath string
		games        string
		seedTrainers bool
	)

	flag.StringVar(&databasePath, "database-path", os.Getenv("DATABASE_PATH"), "SQLite database file path")
	flag.StringVar(&games, "games", "frlg,hgss", "Comma-separated game group slugs to import (must exist in version_groups table)")
	flag.BoolVar(&seedTrainers, "seed-trainers", false, "Import trainer seed data from db/seed/ JSON files")
	flag.Parse()

	if databasePath == "" {
		databasePath = "data/pokesensei.db"
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx := context.Background()

	// Run migrations first
	if err := database.RunMigrations(databasePath, db.EmbedMigrations); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Info("migrations completed")

	sqlDB, err := database.NewDB(ctx, databasePath)
	if err != nil {
		return fmt.Errorf("creating database: %w", err)
	}
	defer sqlDB.Close()

	queries := generated.New(sqlDB)

	// Parse game group slugs and resolve against DB
	gameSlugs := strings.Split(games, ",")
	for i := range gameSlugs {
		gameSlugs[i] = strings.TrimSpace(gameSlugs[i])
	}

	var versionGroups []generated.VersionGroup
	for _, slug := range gameSlugs {
		vg, err := queries.GetVersionGroupBySlug(ctx, slug)
		if err != nil {
			return fmt.Errorf("unknown game group %q (not found in version_groups table): %w", slug, err)
		}
		versionGroups = append(versionGroups, vg)
	}

	// Determine global max dex across all requested games
	var maxDex int
	var versionGroupIDs []int
	for _, vg := range versionGroups {
		if int(vg.MaxPokedex) > maxDex {
			maxDex = int(vg.MaxPokedex)
		}
		versionGroupIDs = append(versionGroupIDs, int(vg.ID))
	}
	log.Info("resolved game groups", "slugs", gameSlugs, "max_dex", maxDex)

	client := NewPokeAPIClient(log)
	importer := NewImporter(sqlDB, client, log)

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

	// Import game versions from PokeAPI (based on version_groups in DB)
	if err := importer.ImportGameVersions(ctx, versionGroupIDs); err != nil {
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
	for i, vg := range versionGroups {
		slug := gameSlugs[i]
		vgInfo, err := resolveVersionGroupInfo(ctx, queries, vg)
		if err != nil {
			return fmt.Errorf("resolving version info for %s: %w", slug, err)
		}

		log.Info("importing game-specific data", "group", slug, "version_group_id", vg.ID)

		if err := importer.ImportMoves(ctx); err != nil {
			return fmt.Errorf("importing moves for %s: %w", slug, err)
		}

		if err := importer.ImportLearnsets(ctx, int(vg.MaxPokedex), int(vg.ID)); err != nil {
			return fmt.Errorf("importing learnsets for %s: %w", slug, err)
		}

		if err := importer.ImportEncounters(ctx, slug, vgInfo); err != nil {
			return fmt.Errorf("importing encounters for %s: %w", slug, err)
		}
	}

	// Import evolution chains
	if err := importer.ImportEvolutionChains(ctx, maxDex); err != nil {
		return fmt.Errorf("importing evolution chains: %w", err)
	}

	// Seed trainer data from JSON files. Convention: db/seed/<slug>_trainers.json.
	if seedTrainers {
		seedImporter := NewSeedImporter(sqlDB, log)
		for _, slug := range gameSlugs {
			seedFile, ok := resolveSeedFile(slug)
			if !ok {
				log.Info("no trainer seed file for group, skipping", "group", slug)
				continue
			}
			log.Info("seeding trainer data", "file", seedFile)
			if err := seedImporter.ImportTrainersFromFile(ctx, seedFile); err != nil {
				return fmt.Errorf("seeding trainers from %s: %w", seedFile, err)
			}
		}
	}

	log.Info("import complete")
	return nil
}

// resolveVersionGroupInfo builds a VersionGroupInfo from DB data by querying
// the game_versions table for the individual version IDs.
func resolveVersionGroupInfo(ctx context.Context, queries *generated.Queries, vg generated.VersionGroup) (VersionGroupInfo, error) {
	versions, err := queries.ListGameVersionsByVersionGroup(ctx, sql.NullInt64{Int64: vg.ID, Valid: true})
	if err != nil {
		return VersionGroupInfo{}, fmt.Errorf("listing game versions for group %d: %w", vg.ID, err)
	}

	info := VersionGroupInfo{
		VersionGroupID: int(vg.ID),
		Name:           vg.Name,
		MaxPokedex:     int(vg.MaxPokedex),
	}
	for _, v := range versions {
		info.VersionIDs = append(info.VersionIDs, int(v.ID))
	}
	return info, nil
}
