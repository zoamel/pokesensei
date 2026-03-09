package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// SeedFile represents the JSON structure of a trainer seed file.
type SeedFile struct {
	GameVersionSlugs []string      `json:"game_version_slugs"`
	Trainers         []SeedTrainer `json:"trainers"`
}

type SeedTrainer struct {
	Name          string               `json:"name"`
	TrainerClass  string               `json:"trainer_class"`
	BadgeNumber   int                  `json:"badge_number"`
	SpecialtyType string               `json:"specialty_type,omitempty"`
	EncounterName string               `json:"encounter_name"`
	Pokemon       []SeedTrainerPokemon `json:"pokemon"`
}

type SeedTrainerPokemon struct {
	PokemonID int      `json:"pokemon_id"`
	Level     int      `json:"level"`
	Position  int      `json:"position"`
	Moves     []string `json:"moves"` // Move slugs
}

// SeedImporter handles importing trainer seed data from JSON files.
type SeedImporter struct {
	db  *sql.DB
	log *slog.Logger
}

func NewSeedImporter(db *sql.DB, log *slog.Logger) *SeedImporter {
	return &SeedImporter{db: db, log: log}
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
	versionIDs := make([]int64, 0, len(seed.GameVersionSlugs))
	for _, slug := range seed.GameVersionSlugs {
		var id int64
		err := si.db.QueryRowContext(ctx, "SELECT id FROM game_versions WHERE slug = ?", slug).Scan(&id)
		if err != nil {
			return fmt.Errorf("game version %q not found: %w", slug, err)
		}
		versionIDs = append(versionIDs, id)
	}

	// Delete existing trainers for these game versions
	for _, versionID := range versionIDs {
		if _, err := si.db.ExecContext(ctx, "DELETE FROM trainers WHERE game_version_id = ?", versionID); err != nil {
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

func (si *SeedImporter) buildMoveSlugMap(ctx context.Context) (map[string]int64, error) {
	rows, err := si.db.QueryContext(ctx, "SELECT id, slug FROM moves")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	moveMap := make(map[string]int64)
	for rows.Next() {
		var id int64
		var slug string
		if err := rows.Scan(&id, &slug); err != nil {
			return nil, err
		}
		moveMap[slug] = id
	}
	return moveMap, rows.Err()
}

func (si *SeedImporter) insertTrainer(ctx context.Context, trainer SeedTrainer, versionID int64, moveMap map[string]int64) error {
	var specialtyType *string
	if trainer.SpecialtyType != "" {
		specialtyType = &trainer.SpecialtyType
	}

	result, err := si.db.ExecContext(ctx,
		`INSERT INTO trainers (name, trainer_class, game_version_id, badge_number, specialty_type, encounter_name)
		VALUES (?, ?, ?, ?, ?, ?)`,
		trainer.Name, trainer.TrainerClass, versionID, trainer.BadgeNumber, specialtyType, trainer.EncounterName,
	)
	if err != nil {
		return fmt.Errorf("inserting trainer row: %w", err)
	}
	trainerID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting trainer id: %w", err)
	}

	for _, tp := range trainer.Pokemon {
		result, err := si.db.ExecContext(ctx,
			`INSERT INTO trainer_pokemon (trainer_id, pokemon_id, level, position)
			VALUES (?, ?, ?, ?)`,
			trainerID, tp.PokemonID, tp.Level, tp.Position,
		)
		if err != nil {
			return fmt.Errorf("inserting trainer pokemon %d: %w", tp.PokemonID, err)
		}
		trainerPokemonID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("getting trainer pokemon id: %w", err)
		}

		// Insert moves
		for slot, moveSlug := range tp.Moves {
			moveID, ok := moveMap[moveSlug]
			if !ok {
				si.log.Warn("unknown move slug in seed data", "move", moveSlug, "trainer", trainer.Name)
				continue
			}
			if _, err := si.db.ExecContext(ctx,
				`INSERT INTO trainer_pokemon_moves (trainer_pokemon_id, move_id, slot)
				VALUES (?, ?, ?)`,
				trainerPokemonID, moveID, slot+1,
			); err != nil {
				return fmt.Errorf("inserting trainer pokemon moves: %w", err)
			}
		}
	}

	return nil
}
