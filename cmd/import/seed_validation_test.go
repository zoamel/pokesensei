package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSeedFile_StructuralValidation parses each trainer seed file from the
// repo-relative db/seed/ directory and asserts basic invariants. This is a
// safety net for typos/missing fields before the importer runs.
func TestSeedFile_StructuralValidation(t *testing.T) {
	oldWd, _ := os.Getwd()
	// Test runs in the package directory (cmd/import); walk up to repo root.
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	files := []string{
		"db/seed/frlg_trainers.json",
		"db/seed/hgss_trainers.json",
		"db/seed/xy_trainers.json",
	}

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var seed SeedFile
			if err := json.Unmarshal(data, &seed); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}

			if len(seed.GameVersionSlugs) == 0 {
				t.Errorf("%s: game_version_slugs is empty", path)
			}
			if len(seed.Trainers) == 0 {
				t.Errorf("%s: no trainers", path)
			}

			allowedClasses := map[string]bool{
				"gym_leader": true,
				"elite_four": true,
				"champion":   true,
				"rival":      true,
			}
			for i, trainer := range seed.Trainers {
				if trainer.Name == "" {
					t.Errorf("%s[%d]: empty name", path, i)
				}
				if !allowedClasses[trainer.TrainerClass] {
					t.Errorf("%s[%d] %q: unknown trainer_class %q", path, i, trainer.Name, trainer.TrainerClass)
				}
				if trainer.BadgeNumber < 0 || trainer.BadgeNumber > 16 {
					t.Errorf("%s[%d] %q: badge_number %d out of range", path, i, trainer.Name, trainer.BadgeNumber)
				}
				if trainer.EncounterName == "" {
					t.Errorf("%s[%d] %q: empty encounter_name", path, i, trainer.Name)
				}
				if len(trainer.Pokemon) == 0 {
					t.Errorf("%s[%d] %q: no pokemon", path, i, trainer.Name)
				}
				for j, p := range trainer.Pokemon {
					if p.PokemonID <= 0 || p.PokemonID > 721 {
						t.Errorf("%s[%d] %q: pokemon[%d].pokemon_id=%d out of range 1..721", path, i, trainer.Name, j, p.PokemonID)
					}
					if p.Level < 1 || p.Level > 100 {
						t.Errorf("%s[%d] %q: pokemon[%d].level=%d out of range", path, i, trainer.Name, j, p.Level)
					}
					if p.Position < 1 || p.Position > 6 {
						t.Errorf("%s[%d] %q: pokemon[%d].position=%d out of range", path, i, trainer.Name, j, p.Position)
					}
					if len(p.Moves) == 0 || len(p.Moves) > 4 {
						t.Errorf("%s[%d] %q: pokemon[%d] has %d moves (want 1..4)", path, i, trainer.Name, j, len(p.Moves))
					}
					for k, slug := range p.Moves {
						if slug == "" {
							t.Errorf("%s[%d] %q: pokemon[%d].moves[%d] is empty", path, i, trainer.Name, j, k)
						}
					}
				}
			}
		})
	}
}
