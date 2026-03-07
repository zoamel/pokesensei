package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedFile represents the JSON structure of a trainer seed file.
type SeedFile struct {
	GameVersionSlugs []string      `json:"game_version_slugs"`
	Trainers         []SeedTrainer `json:"trainers"`
}

type SeedTrainer struct {
	Name           string             `json:"name"`
	TrainerClass   string             `json:"trainer_class"`
	BadgeNumber    int                `json:"badge_number"`
	SpecialtyType  string             `json:"specialty_type,omitempty"`
	EncounterName  string             `json:"encounter_name"`
	Pokemon        []SeedTrainerPokemon `json:"pokemon"`
}

type SeedTrainerPokemon struct {
	PokemonID int      `json:"pokemon_id"`
	Level     int      `json:"level"`
	Position  int      `json:"position"`
	Moves     []string `json:"moves"` // Move slugs
}

// SeedImporter handles importing trainer seed data from JSON files.
type SeedImporter struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewSeedImporter(pool *pgxpool.Pool, log *slog.Logger) *SeedImporter {
	return &SeedImporter{pool: pool, log: log}
}

func (si *SeedImporter) ImportTrainersFromFile(ctx context.Context, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading seed file %s: %w", filePath, err)
	}

	var seed SeedFile
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("parsing seed file %s: %w", filePath, err)
	}

	// Resolve game version IDs from slugs
	versionIDs := make([]int32, 0, len(seed.GameVersionSlugs))
	for _, slug := range seed.GameVersionSlugs {
		var id int32
		err := si.pool.QueryRow(ctx, "SELECT id FROM game_versions WHERE slug = $1", slug).Scan(&id)
		if err != nil {
			return fmt.Errorf("game version %q not found: %w", slug, err)
		}
		versionIDs = append(versionIDs, id)
	}

	// Delete existing trainers for these game versions
	for _, versionID := range versionIDs {
		if _, err := si.pool.Exec(ctx, "DELETE FROM trainers WHERE game_version_id = $1", versionID); err != nil {
			return fmt.Errorf("clearing trainers for version %d: %w", versionID, err)
		}
	}

	// Build a move slug -> ID lookup
	moveMap, err := si.buildMoveSlugMap(ctx)
	if err != nil {
		return fmt.Errorf("building move map: %w", err)
	}

	trainerCount := 0
	for _, versionID := range versionIDs {
		for _, trainer := range seed.Trainers {
			if err := si.insertTrainer(ctx, trainer, versionID, moveMap); err != nil {
				return fmt.Errorf("inserting trainer %q: %w", trainer.Name, err)
			}
			trainerCount++
		}
	}

	si.log.Info("imported trainers from seed", "file", filePath, "count", trainerCount)
	return nil
}

func (si *SeedImporter) buildMoveSlugMap(ctx context.Context) (map[string]int32, error) {
	rows, err := si.pool.Query(ctx, "SELECT id, slug FROM moves")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	moveMap := make(map[string]int32)
	for rows.Next() {
		var id int32
		var slug string
		if err := rows.Scan(&id, &slug); err != nil {
			return nil, err
		}
		moveMap[slug] = id
	}
	return moveMap, rows.Err()
}

func (si *SeedImporter) insertTrainer(ctx context.Context, trainer SeedTrainer, versionID int32, moveMap map[string]int32) error {
	var specialtyType *string
	if trainer.SpecialtyType != "" {
		specialtyType = &trainer.SpecialtyType
	}

	var trainerID int32
	err := si.pool.QueryRow(ctx,
		`INSERT INTO trainers (name, trainer_class, game_version_id, badge_number, specialty_type, encounter_name)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		trainer.Name, trainer.TrainerClass, versionID, trainer.BadgeNumber, specialtyType, trainer.EncounterName,
	).Scan(&trainerID)
	if err != nil {
		return fmt.Errorf("inserting trainer row: %w", err)
	}

	for _, tp := range trainer.Pokemon {
		var trainerPokemonID int32
		err := si.pool.QueryRow(ctx,
			`INSERT INTO trainer_pokemon (trainer_id, pokemon_id, level, position)
			VALUES ($1, $2, $3, $4) RETURNING id`,
			trainerID, tp.PokemonID, tp.Level, tp.Position,
		).Scan(&trainerPokemonID)
		if err != nil {
			return fmt.Errorf("inserting trainer pokemon %d: %w", tp.PokemonID, err)
		}

		// Insert moves
		batch := &pgx.Batch{}
		for slot, moveSlug := range tp.Moves {
			moveID, ok := moveMap[moveSlug]
			if !ok {
				si.log.Warn("unknown move slug in seed data", "move", moveSlug, "trainer", trainer.Name)
				continue
			}
			batch.Queue(
				`INSERT INTO trainer_pokemon_moves (trainer_pokemon_id, move_id, slot)
				VALUES ($1, $2, $3)`,
				trainerPokemonID, moveID, slot+1,
			)
		}

		if batch.Len() > 0 {
			br := si.pool.SendBatch(ctx, batch)
			for i := 0; i < batch.Len(); i++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("inserting trainer pokemon moves: %w", err)
				}
			}
			br.Close()
		}
	}

	return nil
}
