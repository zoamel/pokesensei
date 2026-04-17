# Pokémon X/Y — Phase A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Pokémon X and Y playable in PokéSensei — once this lands, the user can pick X or Y in the game switcher and every existing feature (finder, detail, team builder with the existing gap matrix, battle helper, type chart) works against XY using the post-Fairy type era that's already in migration 008.

**Architecture:** Pure-data addition. One new migration (011) inserts a `version_groups` row for XY, two `game_versions` rows (X, Y), and 12 `starter_groups` rows (Kalos + Kanto gift starters). Two hand-authored data files (`cmd/import/badges/xy.json`, `db/seed/xy_trainers.json`) feed the existing importer. One small refactor makes seed-file discovery slug-driven so future games are pure data.

**Tech Stack:** Go 1.25, SQLite (`modernc.org/sqlite`), goose migrations, sqlc, existing `cmd/import/` pipeline. No new dependencies.

**Spec reference:** `docs/superpowers/specs/2026-04-17-pokemon-xy-design.md` (Phase A section).

---

## File Structure

**New files:**
- `db/migrations/011_xy_game.sql` — inserts XY version group, X/Y game versions, 12 starter rows.
- `cmd/import/badges/xy.json` — Kalos location-slug → badge-number mapping (0–8).
- `db/seed/xy_trainers.json` — 8 gym leaders + Elite Four + Champion + rivals. ~25 trainers total.
- `internal/database/migration_011_test.go` — integration test for migration 011.
- `cmd/import/seed_discovery_test.go` — unit test for the slug-driven seed-file helper.
- `cmd/import/seed_validation_test.go` — parses `xy_trainers.json` and asserts structural invariants.

**Modified files:**
- `cmd/import/main.go` — replace hardcoded `seedFiles` map with slug-based file discovery using a new helper in `seed_discovery.go`.
- `cmd/import/seed_discovery.go` — new tiny file housing the helper (keeps `main.go` slim and makes the helper testable).
- `README.md` — list XY as supported.

**Not touched:**
- `internal/` handlers — no code changes needed. All multi-game logic already runs off `version_groups` + `game_versions` + `GameContext` middleware.
- `cmd/import/importer.go` — no code changes. The existing `ImportPokemon`, `ImportMoves`, `ImportLearnsets`, `ImportEncounters` all resolve version groups from the DB.
- `db/queries/` — no query changes.

---

## Task 1: Migration 011 — XY version group, X/Y versions, starters

**Files:**
- Test: `internal/database/migration_011_test.go` (new)
- Create: `db/migrations/011_xy_game.sql`

### Background

`RunMigrations` is invoked via `internal/database.RunMigrations(dbPath, db.EmbedMigrations)`. Migrations are embedded via `db/embed.go`, so any new file in `db/migrations/` is picked up automatically. The existing `database_test.go::TestRunMigrations_BackfillsGameVersionGroupIDs` is the template for DB-level migration tests.

PokéAPI ID references:
- Version group `xy` → `id = 16`
- Version `x` → `id = 23`
- Version `y` → `id = 24`
- Pokémon: Chespin=650, Fennekin=653, Froakie=656, Bulbasaur=1, Charmander=4, Squirtle=7

**Important:** `version_groups.id = 16` coexists with `game_versions.id = 16` (SoulSilver). They are separate tables; no conflict.

### Steps

- [ ] **Step 1: Write the failing test**

Create `internal/database/migration_011_test.go`:

```go
package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"zoamel/pokesensei/db"
)

func TestRunMigrations_011_InsertsXYGameData(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "pokesensei.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer sqlDB.Close()

	if err := RunMigrations(dbPath, db.EmbedMigrations); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	ctx := context.Background()

	var vgName, vgSlug, vgEra string
	var vgGeneration, vgMaxDex, vgMaxBadges int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT name, slug, generation, max_pokedex, type_chart_era, max_badges
		FROM version_groups WHERE id = 16
	`).Scan(&vgName, &vgSlug, &vgGeneration, &vgMaxDex, &vgEra, &vgMaxBadges)
	if err != nil {
		t.Fatalf("query xy version group: %v", err)
	}
	if vgSlug != "xy" || vgName != "X / Y" || vgGeneration != 6 ||
		vgMaxDex != 721 || vgEra != "post_fairy" || vgMaxBadges != 8 {
		t.Fatalf("xy version group fields wrong: slug=%q name=%q gen=%d maxDex=%d era=%q badges=%d",
			vgSlug, vgName, vgGeneration, vgMaxDex, vgEra, vgMaxBadges)
	}

	gameVersions := map[int64]struct {
		slug string
		name string
	}{
		23: {"x", "X"},
		24: {"y", "Y"},
	}
	for id, want := range gameVersions {
		var slug, name string
		var vgID sql.NullInt64
		err := sqlDB.QueryRowContext(ctx, `
			SELECT slug, name, version_group_id FROM game_versions WHERE id = ?
		`, id).Scan(&slug, &name, &vgID)
		if err != nil {
			t.Fatalf("query game_version %d: %v", id, err)
		}
		if slug != want.slug || name != want.name ||
			!vgID.Valid || vgID.Int64 != 16 {
			t.Fatalf("game_version %d wrong: slug=%q name=%q vgID=%+v",
				id, slug, name, vgID)
		}
	}

	expectedStarters := map[int64][]int64{
		23: {1, 4, 7, 650, 653, 656},
		24: {1, 4, 7, 650, 653, 656},
	}
	for gvID, wantIDs := range expectedStarters {
		rows, err := sqlDB.QueryContext(ctx, `
			SELECT pokemon_id FROM starter_groups
			WHERE game_version_id = ? ORDER BY pokemon_id
		`, gvID)
		if err != nil {
			t.Fatalf("query starters for %d: %v", gvID, err)
		}
		var got []int64
		for rows.Next() {
			var pid int64
			if err := rows.Scan(&pid); err != nil {
				rows.Close()
				t.Fatalf("scan starter row: %v", err)
			}
			got = append(got, pid)
		}
		rows.Close()
		if len(got) != len(wantIDs) {
			t.Fatalf("game_version %d starters: got %d rows, want %d", gvID, len(got), len(wantIDs))
		}
		for i, want := range wantIDs {
			if got[i] != want {
				t.Fatalf("game_version %d starter[%d] = %d, want %d", gvID, i, got[i], want)
			}
		}
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/database/ -run TestRunMigrations_011 -v
```

Expected: FAIL with "no rows in result set" on the `version_groups WHERE id = 16` query — migration 011 doesn't exist yet.

- [ ] **Step 3: Create the migration file**

Create `db/migrations/011_xy_game.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
INSERT INTO version_groups (id, name, slug, generation, max_pokedex, type_chart_era, max_badges)
VALUES (16, 'X / Y', 'xy', 6, 721, 'post_fairy', 8);

INSERT INTO game_versions (id, version_group_id, name, slug)
VALUES (23, 16, 'X', 'x'),
       (24, 16, 'Y', 'y');

-- Starters for X and Y: Kalos trio (Chespin #650, Fennekin #653, Froakie #656)
-- plus the Kanto gift starters from Professor Sycamore at Lumiose City
-- (Bulbasaur #1, Charmander #4, Squirtle #7). Both sets are obtainable early.
INSERT INTO starter_groups (game_version_id, pokemon_id) VALUES
    (23, 1), (23, 4), (23, 7), (23, 650), (23, 653), (23, 656),
    (24, 1), (24, 4), (24, 7), (24, 650), (24, 653), (24, 656);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM starter_groups WHERE game_version_id IN (23, 24);
DELETE FROM game_versions WHERE id IN (23, 24);
DELETE FROM version_groups WHERE id = 16;
-- +goose StatementEnd
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./internal/database/ -run TestRunMigrations_011 -v
```

Expected: PASS.

- [ ] **Step 5: Run the full test suite to confirm no regression**

```bash
go test ./...
```

Expected: all pass. In particular the existing `TestRunMigrations_BackfillsGameVersionGroupIDs` should still pass because migration 011 adds new rows and does not touch the FRLG/HGSS rows the backfill seeds.

- [ ] **Step 6: Commit**

```bash
git add db/migrations/011_xy_game.sql internal/database/migration_011_test.go
git commit -m "feat: add migration 011 for Pokémon X/Y game data"
```

---

## Task 2: Slug-driven seed-file discovery refactor

**Files:**
- Create: `cmd/import/seed_discovery.go`
- Test: `cmd/import/seed_discovery_test.go`
- Modify: `cmd/import/main.go:152-164` (the `if seedTrainers { ... }` block)

### Background

`cmd/import/main.go` currently hardcodes a map:

```go
seedFiles := map[string]string{
    "frlg": "db/seed/frlg_trainers.json",
    "hgss": "db/seed/hgss_trainers.json",
}
for _, slug := range gameSlugs {
    if seedFile, ok := seedFiles[slug]; ok { ... }
}
```

Every new game currently requires a code change here. Replace with a helper that derives the path from the slug and skips missing files. The spec calls this out as the "zero Go code changes per new game" aspiration.

### Steps

- [ ] **Step 1: Write the failing test**

Create `cmd/import/seed_discovery_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSeedFile_ReturnsPathWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	seedDir := filepath.Join(dir, "db", "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(seedDir, "xy_trainers.json")
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	path, ok := resolveSeedFile("xy")
	if !ok {
		t.Fatal("expected ok=true for existing file")
	}
	if path != "db/seed/xy_trainers.json" {
		t.Errorf("path = %q, want db/seed/xy_trainers.json", path)
	}
}

func TestResolveSeedFile_ReturnsFalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, ok := resolveSeedFile("nonexistent")
	if ok {
		t.Error("expected ok=false for missing file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/import/ -run TestResolveSeedFile -v
```

Expected: FAIL with "undefined: resolveSeedFile".

- [ ] **Step 3: Implement the helper**

Create `cmd/import/seed_discovery.go`:

```go
package main

import (
	"fmt"
	"os"
)

// resolveSeedFile returns the trainer seed file path for a game slug if the
// file exists. The convention is db/seed/<slug>_trainers.json. This lets new
// games be added as pure data without any Go code change.
func resolveSeedFile(slug string) (string, bool) {
	path := fmt.Sprintf("db/seed/%s_trainers.json", slug)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
go test ./cmd/import/ -run TestResolveSeedFile -v
```

Expected: PASS for both cases.

- [ ] **Step 5: Replace the hardcoded map in `cmd/import/main.go`**

Find this block (around lines 150–165):

```go
	// Seed trainer data from JSON files
	if seedTrainers {
		seedImporter := NewSeedImporter(sqlDB, log)
		seedFiles := map[string]string{
			"frlg": "db/seed/frlg_trainers.json",
			"hgss": "db/seed/hgss_trainers.json",
		}
		for _, slug := range gameSlugs {
			if seedFile, ok := seedFiles[slug]; ok {
				log.Info("seeding trainer data", "file", seedFile)
				if err := seedImporter.ImportTrainersFromFile(ctx, seedFile); err != nil {
					return fmt.Errorf("seeding trainers from %s: %w", seedFile, err)
				}
			}
		}
	}
```

Replace with:

```go
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
```

- [ ] **Step 6: Confirm the whole package still builds**

```bash
go build ./cmd/import/
```

Expected: no output (success).

- [ ] **Step 7: Run the full test suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add cmd/import/seed_discovery.go cmd/import/seed_discovery_test.go cmd/import/main.go
git commit -m "refactor: discover trainer seed files by slug convention"
```

---

## Task 3: Badge map `cmd/import/badges/xy.json`

**Files:**
- Create: `cmd/import/badges/xy.json`

### Background

The importer calls `LoadBadgeMap("xy")` which reads `cmd/import/badges/xy.json` (see `cmd/import/gamedata.go:20`). Keys are PokéAPI location slugs; values are the badge number (0–8) required to access that location.

`cmd/import/badges/hgss.json` is the reference pattern. Source the XY data from Bulbapedia's "Kalos Route" pages and gym progression:

| Badge # | Gym | Earned at | Region opens |
|---|---|---|---|
| 0 | — | — | Vaniville/Aquacorde/Santalune Forest/Route 1–3, Santalune City |
| 1 | Bug (Viola) | Santalune | Route 4, Lumiose City (partial), Route 5, Camphrier Town, Route 6–7 |
| 2 | Rock (Grant) | Cyllage | Route 8, Geosenge, Route 9, Route 10, Route 11, Reflection Cave, Shalour City |
| 3 | Fighting (Korrina) | Shalour | Tower of Mastery/Mega Ring, Route 12–13, Coumarine City |
| 4 | Grass (Ramos) | Coumarine | Route 14, Laverre City, Poké Ball Factory, Route 15, Dendemille Town |
| 5 | Electric (Clemont) | Lumiose (full) | Route 16, Frost Cavern, Route 17, Anistar City |
| 6 | Fairy (Valerie) | Laverre | Route 18, Couriway Town, Route 19, Snowbelle City |
| 7 | Psychic (Olympia) | Anistar | Route 20, Pokémon Village, Route 21, Snowbelle (second entry) |
| 8 | Ice (Wulfric) | Snowbelle | Victory Road, Pokémon League |

### Steps

- [ ] **Step 1: Create `cmd/import/badges/xy.json`**

Populate with every relevant Kalos PokéAPI location slug, keyed by the PokéAPI slug (kebab-case), mapped to the badge that gates access. Use the table above as ground truth. The file should resemble:

```json
{
  "vaniville-town": 0,
  "aquacorde-town": 0,
  "kalos-route-1": 0,
  "kalos-route-2": 0,
  "santalune-forest": 0,
  "santalune-city": 0,
  "kalos-route-3": 0,
  "kalos-route-4": 1,
  "lumiose-city": 1,
  "kalos-route-5": 1,
  "camphrier-town": 1,
  "kalos-route-6": 1,
  "parfum-palace": 1,
  "kalos-route-7": 1,
  "connecting-cave": 1,
  "kalos-route-8": 2,
  "ambrette-town": 2,
  "kalos-route-9": 2,
  "cyllage-city": 2,
  "kalos-route-10": 2,
  "geosenge-town": 2,
  "kalos-route-11": 2,
  "reflection-cave": 2,
  "shalour-city": 2,
  "tower-of-mastery": 3,
  "kalos-route-12": 3,
  "azure-bay": 3,
  "kalos-route-13": 3,
  "coumarine-city": 3,
  "kalos-route-14": 4,
  "laverre-city": 4,
  "poke-ball-factory": 4,
  "kalos-route-15": 4,
  "lost-hotel": 4,
  "dendemille-town": 4,
  "kalos-route-16": 5,
  "frost-cavern": 5,
  "kalos-route-17": 5,
  "anistar-city": 5,
  "kalos-route-18": 6,
  "couriway-town": 6,
  "kalos-route-19": 6,
  "snowbelle-city": 6,
  "kalos-route-20": 7,
  "pokemon-village": 7,
  "kalos-route-21": 7,
  "victory-road-kalos": 8,
  "kalos-pokemon-league": 8,
  "terminus-cave": 4,
  "chamber-of-emptiness": 8
}
```

**Verification aids:**
- Each value must be in `0..8`.
- Each key must match PokéAPI's location slug exactly (kebab-case, use singular, and note `kalos-route-N` not `route-N-kalos`). If unsure, confirm by querying `https://pokeapi.co/api/v2/location/<slug>/` — a 200 response proves the slug.

- [ ] **Step 2: Validate it parses as JSON and all values are in range**

```bash
python3 -c "import json; d = json.load(open('cmd/import/badges/xy.json')); assert all(isinstance(v, int) and 0 <= v <= 8 for v in d.values()), 'bad badge value'; print(f'OK: {len(d)} locations')"
```

Expected: `OK: N locations` where N matches your file.

- [ ] **Step 3: Confirm the importer can load it (dry-run)**

Write a short ad-hoc verification via `go run`:

```bash
go run -tags=ignore ./cmd/import --help 2>&1 | head -5
```

The importer itself will validate the file during the next task's end-to-end run; just confirming the module compiles with the new data in place.

```bash
go build ./...
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add cmd/import/badges/xy.json
git commit -m "feat: add XY badge-to-location mapping for import pipeline"
```

---

## Task 4: Trainer seed file — gym leaders

**Files:**
- Create: `db/seed/xy_trainers.json`

### Background

The trainer seed schema is defined by `cmd/import/seed.go` (struct `SeedFile`, `SeedTrainer`, `SeedTrainerPokemon`). `db/seed/hgss_trainers.json` is the reference pattern. Each trainer has:

- `name`, `trainer_class`, `badge_number`, `specialty_type` (optional), `encounter_name`
- `pokemon`: list of `{ pokemon_id, level, position, moves: [move-slug, ...] }`

Move slugs must exist in the `moves` table (populated by the importer from PokeAPI). The seed importer logs a warning for unknown slugs but does not fail — which is why Task 7 adds explicit validation.

Source each trainer's roster from Bulbapedia (e.g., `https://bulbapedia.bulbagarden.net/wiki/Viola`). Use the **X/Y** team (not ORAS; not Pokémon Masters; not the anime).

### Trainer classes used in this project

Existing seed files use: `gym_leader`, `elite_four`, `champion`, `rival`. Stick to those for consistency.

### Gym leader roster for XY

| # | Name | Class | Specialty | Gym | Roster |
|---|---|---|---|---|---|
| 1 | Viola | gym_leader | bug | Santalune City Gym | Surskit L10, Vivillon L12 |
| 2 | Grant | gym_leader | rock | Cyllage City Gym | Amaura L25, Tyrunt L25 |
| 3 | Korrina | gym_leader | fighting | Shalour City Gym | Mienfoo L29, Machoke L28, Hawlucha L32 (Mega not used in first battle) |
| 4 | Ramos | gym_leader | grass | Coumarine City Gym | Jumpluff L30, Weepinbell L31, Gogoat L34 |
| 5 | Clemont | gym_leader | electric | Lumiose City Gym | Emolga L35, Magneton L35, Heliolisk L37 |
| 6 | Valerie | gym_leader | fairy | Laverre City Gym | Mawile L38, Mr. Mime L39, Sylveon L42 |
| 7 | Olympia | gym_leader | psychic | Anistar City Gym | Sigilyph L44, Slowking L45, Meowstic L48 |
| 8 | Wulfric | gym_leader | ice | Snowbelle City Gym | Abomasnow L56, Cryogonal L55, Avalugg L59 |

### Steps

- [ ] **Step 1: Create the skeleton with gym leaders**

Create `db/seed/xy_trainers.json`. Use the HGSS file (`db/seed/hgss_trainers.json`) as a structural reference. Populate with the 8 gym leaders.

**Pokémon ID quick lookup** (from PokéAPI, national dex):
- Surskit=283, Vivillon=666, Amaura=698, Tyrunt=696, Mienfoo=619, Machoke=67, Hawlucha=701, Jumpluff=189, Weepinbell=70, Gogoat=673, Emolga=587, Magneton=82, Heliolisk=695, Mawile=303, Mr. Mime=122, Sylveon=700, Sigilyph=561, Slowking=199, Meowstic=678 (male), Abomasnow=460, Cryogonal=615, Avalugg=713.

Starter skeleton (fill in moves by checking Bulbapedia's trainer pages; the snippet below shows one complete entry as a pattern):

```json
{
  "game_version_slugs": ["x", "y"],
  "trainers": [
    {
      "name": "Viola",
      "trainer_class": "gym_leader",
      "badge_number": 1,
      "specialty_type": "bug",
      "encounter_name": "Santalune City Gym",
      "pokemon": [
        { "pokemon_id": 283, "level": 10, "position": 1, "moves": ["bubble", "quick-attack"] },
        { "pokemon_id": 666, "level": 12, "position": 2, "moves": ["gust", "struggle-bug", "harden", "infestation"] }
      ]
    },
    {
      "name": "Grant",
      "trainer_class": "gym_leader",
      "badge_number": 2,
      "specialty_type": "rock",
      "encounter_name": "Cyllage City Gym",
      "pokemon": [
        { "pokemon_id": 698, "level": 25, "position": 1, "moves": ["rock-throw", "take-down", "aurora-beam", "thunder-wave"] },
        { "pokemon_id": 696, "level": 25, "position": 2, "moves": ["rock-tomb", "bite", "charm", "stomp"] }
      ]
    }
    /* ... 6 more gym leaders, same structure ... */
  ]
}
```

**Move-slug guidance:**
- Slugs are PokéAPI kebab-case: `rock-throw`, `aurora-beam`, `bubble`, `struggle-bug`, `quick-attack`, `infestation`, `thunder-wave`, `rock-tomb`, `stomp`, `take-down`, `charm`, `bite`, etc.
- When in doubt, check `https://pokeapi.co/api/v2/move/<slug>/` — 200 response confirms.

Populate all 8 gym leaders in this step. Movesets can be looked up at Bulbapedia's individual gym-leader pages (e.g., `https://bulbapedia.bulbagarden.net/wiki/Korrina`) under the "Pokémon X and Y" section.

- [ ] **Step 2: Validate the file parses**

```bash
python3 -c "import json; d = json.load(open('db/seed/xy_trainers.json')); print(f'OK: {len(d[\"trainers\"])} trainers, versions={d[\"game_version_slugs\"]}')"
```

Expected: `OK: 8 trainers, versions=['x', 'y']`.

- [ ] **Step 3: Confirm Go still builds**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add db/seed/xy_trainers.json
git commit -m "feat: seed XY gym leaders"
```

---

## Task 5: Trainer seed — Elite Four + Champion

**Files:**
- Modify: `db/seed/xy_trainers.json`

### Elite Four + Champion roster

| Name | Class | Badge | Specialty | Encounter | Roster |
|---|---|---|---|---|---|
| Malva | elite_four | 8 | fire | Pokémon League — Malva's Room | Pyroar L63, Torkoal L63, Chandelure L63, Talonflame L65 |
| Siebold | elite_four | 8 | water | Pokémon League — Siebold's Room | Clawitzer L63, Starmie L63, Gyarados L63, Barbaracle L65 |
| Wikstrom | elite_four | 8 | steel | Pokémon League — Wikstrom's Room | Klefki L63, Probopass L63, Scizor L63, Aegislash L65 |
| Drasna | elite_four | 8 | dragon | Pokémon League — Drasna's Room | Dragalge L63, Druddigon L63, Altaria L63, Noivern L65 |
| Diantha | champion | 8 | — | Pokémon League — Champion's Room | Hawlucha L64, Tyrantrum L65, Aurorus L65, Gourgeist L65, Goodra L66, Gardevoir L68 (Mega-Evolves, but we store the base form) |

**Note on Diantha:** Her Gardevoir Mega-Evolves in battle. Phase A does **not** model trainer Megas — just insert the base Gardevoir (#282) and its moves. Phase B does not change trainer seeds.

### Steps

- [ ] **Step 1: Append Elite Four + Champion to `db/seed/xy_trainers.json`**

Extend the `trainers` array in place. Pattern:

```json
{
  "name": "Malva",
  "trainer_class": "elite_four",
  "badge_number": 8,
  "specialty_type": "fire",
  "encounter_name": "Pokémon League — Malva's Room",
  "pokemon": [
    { "pokemon_id": 668, "level": 63, "position": 1, "moves": ["..."] },
    { "pokemon_id": 324, "level": 63, "position": 2, "moves": ["..."] },
    { "pokemon_id": 609, "level": 63, "position": 3, "moves": ["..."] },
    { "pokemon_id": 663, "level": 65, "position": 4, "moves": ["..."] }
  ]
}
```

**Pokémon IDs:** Pyroar=668, Torkoal=324, Chandelure=609, Talonflame=663, Clawitzer=693, Starmie=121, Gyarados=130, Barbaracle=689, Klefki=707, Probopass=476, Scizor=212, Aegislash=681 (blade/shield form — use 681), Dragalge=691, Druddigon=621, Altaria=334, Noivern=715, Hawlucha=701, Tyrantrum=697, Aurorus=699, Gourgeist=711 (average size; PokéAPI treats small/large as forms, use 711 for base), Goodra=706, Gardevoir=282.

Fill in the specific movesets from Bulbapedia. Each E4 member has 4 Pokémon; Diantha has 6.

- [ ] **Step 2: Validate the file still parses and count grew**

```bash
python3 -c "import json; d = json.load(open('db/seed/xy_trainers.json')); print(f'OK: {len(d[\"trainers\"])} trainers')"
```

Expected: `OK: 13 trainers` (8 gyms + 4 E4 + 1 champion).

- [ ] **Step 3: Commit**

```bash
git add db/seed/xy_trainers.json
git commit -m "feat: seed XY Elite Four and Champion Diantha"
```

---

## Task 6: Trainer seed — rivals

**Files:**
- Modify: `db/seed/xy_trainers.json`

### Rival encounters

The player's rival is Serena (if male PC) or Calem (if female PC). The other three — Shauna, Tierno, Trevor — show up at scripted points. Include the notable story battles; skip cutscene one-off Pokémon.

**Recommended rival battle set** (Bulbapedia lists ~8 rival battles for the main story):

| Name | Class | Badge | Encounter | Notes |
|---|---|---|---|---|
| Calem | rival | 0 | Aquacorde Town (first battle) | 1 Pokémon (starter counter) |
| Shauna | rival | 0 | Aquacorde Town (first battle) | 1 Pokémon (starter counter) |
| Tierno | rival | 0 | Santalune Forest | 1 Pokémon |
| Trevor | rival | 1 | Route 5 | 1–2 Pokémon |
| Calem | rival | 2 | Cyllage City | 3 Pokémon |
| Calem | rival | 4 | Route 14 (Laverre) | ~4 Pokémon |
| Calem | rival | 8 | Victory Road | ~6 Pokémon |
| Shauna | rival | 6 | Anistar City | 3 Pokémon |

For the player who picks each starter, the rival's counter starter differs. Seed **one representative path** — pick the scenario where the player chose **Chespin (650)**; rival counter is Fennekin (653). If you want to handle all three player-starter scenarios in the future, that's data-only and can be added later.

**Name disambiguation:** Multiple Calem rival battles share the name "Calem". The seed schema allows duplicate names (no UNIQUE constraint on name). The combination of `name + encounter_name` is effectively unique. Existing FRLG/HGSS seeds do the same for multi-battle rivals.

### Steps

- [ ] **Step 1: Append rivals to `db/seed/xy_trainers.json`**

Add the 8 rival entries. Use Bulbapedia's "Calem (game)" / "Serena (game)" / "Shauna" / "Tierno" / "Trevor" pages for rosters.

Pattern for a mid-game Calem battle:

```json
{
  "name": "Calem",
  "trainer_class": "rival",
  "badge_number": 4,
  "encounter_name": "Route 14",
  "pokemon": [
    { "pokemon_id": 654, "level": 38, "position": 1, "moves": ["..."] },
    { "pokemon_id": 68,  "level": 38, "position": 2, "moves": ["..."] },
    { "pokemon_id": 38,  "level": 39, "position": 3, "moves": ["..."] },
    { "pokemon_id": 672, "level": 40, "position": 4, "moves": ["..."] }
  ]
}
```

(The `654` / `68` / `38` / `672` above are Braixen / Machoke / Ninetales / Skiddo — placeholder; cross-check Bulbapedia.)

- [ ] **Step 2: Validate the file still parses and count grew**

```bash
python3 -c "import json; d = json.load(open('db/seed/xy_trainers.json')); print(f'OK: {len(d[\"trainers\"])} trainers')"
```

Expected: `OK: 21 trainers` (13 from Task 5 + 8 rivals). Adjust if you chose a different rival-battle subset.

- [ ] **Step 3: Commit**

```bash
git add db/seed/xy_trainers.json
git commit -m "feat: seed XY rival battles (Calem/Shauna/Tierno/Trevor)"
```

---

## Task 7: Seed validation test

**Files:**
- Create: `cmd/import/seed_validation_test.go`

### Background

The seed importer *warns* on unknown move slugs but does not fail — silent data corruption risk. Add a test that catches structural problems in `xy_trainers.json` at test time, before the importer ever runs. The same test template can validate FRLG/HGSS too, which is a free regression benefit.

### Steps

- [ ] **Step 1: Write the test**

Create `cmd/import/seed_validation_test.go`:

```go
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
```

- [ ] **Step 2: Run the test**

```bash
go test ./cmd/import/ -run TestSeedFile_StructuralValidation -v
```

Expected: all three sub-tests PASS. If a move slug is empty or a Pokémon ID is out of range in the XY file, the test fails pointing at the exact index — fix the JSON and rerun.

- [ ] **Step 3: Commit**

```bash
git add cmd/import/seed_validation_test.go
git commit -m "test: validate trainer seed files structurally"
```

---

## Task 8: End-to-end import run and manual verification

**Files:** none created; executes the pipeline and spot-checks results.

### Background

With data files in place, the existing importer should produce a working XY DB. This task validates the full pipeline end-to-end against a fresh database.

### Steps

- [ ] **Step 1: Back up the current dev database (optional safety net)**

```bash
cp data/pokesensei.db data/pokesensei.db.bak 2>/dev/null || true
```

- [ ] **Step 2: Run the importer for XY**

The importer is additive across version groups; passing `--games=frlg,hgss,xy` makes the existing games and XY all present. If you only want XY, pass `--games=xy`. Re-running is safe — the importer clears and re-populates each section.

```bash
go run ./cmd/import --games=frlg,hgss,xy --seed-trainers
```

Expected: log lines showing migrations applied (including 011), types imported, natures imported, game versions imported, Pokémon imported (up to `max_dex=721`), moves/learnsets/encounters imported per version group, trainers seeded for each of the three groups. Total time: several minutes (PokéAPI is rate-limited).

If the run fails for a specific location in `xy.json` (unknown PokéAPI slug), the error will name the slug — fix in the JSON and rerun.

- [ ] **Step 3: Spot-check the database**

```bash
sqlite3 data/pokesensei.db "SELECT id, name, slug, generation, max_pokedex, type_chart_era FROM version_groups ORDER BY id;"
```

Expected output:

```
7|FireRed / LeafGreen|frlg|3|386|pre_fairy
10|HeartGold / SoulSilver|hgss|4|493|pre_fairy
16|X / Y|xy|6|721|post_fairy
```

```bash
sqlite3 data/pokesensei.db "SELECT id, slug, version_group_id FROM game_versions WHERE version_group_id = 16;"
```

Expected:

```
23|x|16
24|y|16
```

```bash
sqlite3 data/pokesensei.db "SELECT COUNT(*) FROM trainers WHERE game_version_id IN (23, 24);"
```

Expected: `42` if you seeded 21 trainers (each appears once per version). Adjust to match your trainer count × 2.

```bash
sqlite3 data/pokesensei.db "SELECT COUNT(*) FROM pokemon;"
```

Expected: `721` (or matches the highest `max_pokedex` across loaded version groups).

```bash
sqlite3 data/pokesensei.db "SELECT COUNT(*) FROM type_efficacy WHERE era = 'post_fairy';"
```

Expected: `324` (18 × 18 matrix).

- [ ] **Step 4: Start the dev server and manually verify**

```bash
make dev
```

Then in a browser:

1. Navigate to `/onboarding` (or clear active game via UI) — the game version dropdown should include X and Y.
2. Pick X → pick Froakie (or any of the 6 starters) → set badge count to 0 → complete onboarding.
3. Dashboard should show "Pokémon X" as active.
4. `/pokemon` finder: filter by Kalos gym badge 1 → Vivillon / route encounters under badge 1 appear.
5. `/battle/trainers` — Viola should appear. Click through a matchup with a starter team.
6. `/battle/types` — the chart should show 18×18 with a Fairy row/column.
7. `/team` — type coverage matrix renders with 18 types.

- [ ] **Step 5: Stop the dev server**

Ctrl+C the `make dev` process.

- [ ] **Step 6: Confirm full test suite still passes**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 7: No commit needed**

This task is execution-only; the code changes that produced these results are already committed in Tasks 1–7.

---

## Task 9: Update README

**Files:**
- Modify: `README.md`

### Steps

- [ ] **Step 1: Add XY to the README's supported-games mention**

`README.md` does not currently enumerate supported games. Add a new short section near the top, right after the Tech Stack bullets:

```markdown
## Supported Games

PokéSensei is a multi-game companion. Currently bundled:

- **FireRed / LeafGreen** (Gen III, Kanto)
- **HeartGold / SoulSilver** (Gen IV, Johto + Kanto)
- **X / Y** (Gen VI, Kalos)

Adding a new game is data-driven: insert a row into `version_groups`, add a
badge map under `cmd/import/badges/`, and a trainer seed JSON under `db/seed/`.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: document X/Y in supported games list"
```

---

## Self-Review

**Spec coverage (Phase A section of `docs/superpowers/specs/2026-04-17-pokemon-xy-design.md`):**

| Spec item | Task |
|---|---|
| A.1 Migration 011 (version_groups, game_versions, starter_groups) | Task 1 |
| A.2 `cmd/import/badges/xy.json` | Task 3 |
| A.2 `db/seed/xy_trainers.json` (gyms + E4 + champion + rivals) | Tasks 4, 5, 6 |
| A.3 Slug-driven seed-file discovery refactor | Task 2 |
| A.4 Import command runs cleanly | Task 8 |
| A.5 Verification (onboarding, finder, battle helper, type chart) | Task 8 |
| Update README | Task 9 |
| Structural safety net for seed JSON | Task 7 (bonus: also covers FRLG/HGSS) |

**Placeholder scan:** No "TBD" / "TODO" / "similar to task N". Where movesets aren't fully enumerated (Tasks 4–6), the plan names the source (specific Bulbapedia pages) and provides a concrete per-Pokémon template — that's task-scoped data authoring, not a placeholder hand-off.

**Type consistency:** `resolveSeedFile`, `SeedFile`, `SeedTrainer`, `SeedTrainerPokemon`, `ImportTrainersFromFile`, `RunMigrations`, `EmbedMigrations` — names match across all tasks and the existing codebase.

**Edge cases addressed:** duplicate rival names (documented — allowed by schema); unknown move slugs (Task 7 test catches empty slugs; the importer warns for actual unknown-but-nonempty slugs, which will surface in Task 8's log output).

---

**Plan complete and saved to `docs/superpowers/plans/2026-04-17-pokemon-xy-phase-a.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
