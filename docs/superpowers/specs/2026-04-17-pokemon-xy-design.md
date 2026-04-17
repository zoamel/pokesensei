# Pokémon X / Y Support

## Context

PokéSensei supports FRLG (Gen III) and HGSS (Gen IV) playthroughs. The user is actively playing Pokémon X and wants full in-game companion support: Pokédex, team builder, battle helper, and type chart — all scoped to the XY version group — plus a new multi-axis **team coverage rating** that replaces the existing gap matrix as the team builder's headline analytics surface.

XY is Gen VI, `post_fairy` era, and the version group PokéSensei's multi-game redesign (spec `2026-04-01-multi-game-redesign-design.md`) explicitly anticipated as "Phase 6+: first post_fairy game." The schema and middleware were designed for this addition; migration 008 already introduced era-aware type efficacy.

This spec covers:
- Adding XY as a fully playable game version group (X and Y as selectable versions).
- Adding Mega Evolution support for player team members (not trainers).
- Adding a multi-axis coverage rating (Offensive / Defensive / Diversity) with letter grade.

---

## Scope Decisions (Recap)

| Decision | Choice |
|---|---|
| Version group coverage | **X and Y** both selectable under `xy` version group (PokéAPI id = 16). ORAS deferred. |
| Pokédex scope | **National through Gen VI** (max_pokedex = 721). Kalos regional dex numbering deferred. |
| Coverage rating shape | **Multi-axis scorecard**: Offensive, Defensive, Diversity — each 0–100 plus an overall letter grade. |
| Moves considered for rating | **Current assigned moves only** (no aspirational / learnset-wide scoring). |
| Offensive vs defensive axis | Offense from moves (with STAB); Defense from Pokémon types. |
| Mega Evolution | **Team-member toggle only**: player team slots get a Mega Stone picker. Trainers do not Mega-Evolve. |
| Trainer seed scope | **Critical path + rivals**: 8 gym leaders, Elite Four, Champion, Serena/Calem rival chain, Shauna/Tierno/Trevor. Team Flare excluded. |
| Delivery | **Three phases, three PRs**, each independently shippable. |

---

## Architecture Alignment

No changes to the established multi-game architecture. Specifically:

- `version_groups` and `game_versions` tables are the extension points; XY is a pure-data addition.
- `GameContext` middleware supplies `TypeChartEra = "post_fairy"` automatically once the XY row is inserted; all existing era-aware queries (`types.sql::GetTypeEfficacy`) activate for free.
- The existing import pipeline (`cmd/import/`) reads `version_groups` from the DB; adding XY requires one migration, one badge JSON, one trainer JSON, and optionally a small refactor to make seed-file discovery slug-driven.

Mega Evolution is schema-additive — a new `pokemon_mega_forms` table plus one nullable column (`team_members.mega_form_id`). It does not alter the `pokemon` table's "one row per national dex number" invariant.

The coverage rating lives in a new isolated package `internal/coverage/` with pure functions (no DB). The handler layer builds an input snapshot from existing queries and passes it in.

---

## Phase A — XY as a Playable Game

**Outcome:** User can pick Pokémon X (or Y) in the game switcher. All existing features (finder, detail page, team builder with current gap matrix, battle helper, type chart) work against XY using the post-Fairy era.

### A.1 Migration `011_xy_game.sql`

```sql
-- +goose Up
-- +goose StatementBegin
INSERT INTO version_groups (id, name, slug, generation, max_pokedex, type_chart_era, max_badges)
VALUES (16, 'X / Y', 'xy', 6, 721, 'post_fairy', 8);

INSERT INTO game_versions (id, version_group_id, name, slug)
VALUES (23, 16, 'X', 'x'),
       (24, 16, 'Y', 'y');

-- Starters: Kalos trio (Chespin #650, Fennekin #653, Froakie #656)
-- plus Kanto gift starters from Professor Sycamore (Bulbasaur #1, Charmander #4, Squirtle #7).
-- Both sets are obtainable early; the onboarding flow should surface both.
INSERT INTO starter_groups (game_version_id, pokemon_id) VALUES
    (23, 650), (23, 653), (23, 656), (23, 1), (23, 4), (23, 7),
    (24, 650), (24, 653), (24, 656), (24, 1), (24, 4), (24, 7);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM starter_groups WHERE game_version_id IN (23, 24);
DELETE FROM game_versions WHERE id IN (23, 24);
DELETE FROM version_groups WHERE id = 16;
-- +goose StatementEnd
```

### A.2 Hand-authored data files

- **`cmd/import/badges/xy.json`** — PokéAPI location slug → badge number (0–8) for every Kalos route/city/dungeon. Source: Bulbapedia XY walkthrough. Gates the finder's "available now" filter.
- **`db/seed/xy_trainers.json`** — roughly 25–30 trainers:
  - 8 Gym Leaders: Viola, Grant, Korrina, Ramos, Clemont, Valerie, Olympia, Wulfric.
  - Elite Four: Malva, Siebold, Wikstrom, Drasna.
  - Champion: Diantha.
  - Rivals: Serena/Calem (each of their main story battles), Shauna, Tierno, Trevor.
  - Schema identical to `hgss_trainers.json`.
  - **Explicitly excluded:** Team Flare admins and Lysandre.

### A.3 Import pipeline refactor

`cmd/import/main.go` currently hardcodes a `seedFiles` map of slug → JSON path. Replace with slug-driven discovery:

```go
if seedTrainers {
    seedImporter := NewSeedImporter(sqlDB, log)
    for _, slug := range gameSlugs {
        seedFile := fmt.Sprintf("db/seed/%s_trainers.json", slug)
        if _, err := os.Stat(seedFile); os.IsNotExist(err) {
            log.Info("no trainer seed file for group, skipping", "group", slug)
            continue
        }
        if err := seedImporter.ImportTrainersFromFile(ctx, seedFile); err != nil {
            return fmt.Errorf("seeding trainers from %s: %w", seedFile, err)
        }
    }
}
```

This matches the "zero Go code changes per new game" aspiration already stated in the existing multi-game spec. Future games (ORAS, SM, etc.) require only data files.

### A.4 Import command

```bash
go run ./cmd/import --games=xy --seed-trainers
```

This pulls Pokémon 1–721 from PokeAPI, imports moves/learnsets/encounters scoped to the XY version group, and seeds the XY trainers.

### A.5 Verification (Phase A)

- New onboarding flow for X → game_state created; user can pick any of the 6 starters (3 Kalos + 3 Kanto).
- Finder shows Pokémon 1–721, filterable by Kalos gym badge.
- Battle helper matchup against Viola uses post-Fairy efficacy: Steel no longer resists Ghost/Dark; Fairy exists.
- Type chart at `/battle/types` shows the full 18×18 grid with XY active.
- Existing coverage matrix renders correctly against 18 types.
- Switching from XY back to FRLG drops the chart to 17×17.

### A.6 Not in Phase A

- Mega Evolution (Phase B).
- Multi-axis rating (Phase C).
- Regional dex numbering.
- Team Flare trainers.

---

## Phase B — Mega Evolution (Team Members)

**Outcome:** A team slot with a Mega-capable Pokémon offers a Mega Stone picker. When set, the team member's **effective types, ability, and stats** use the Mega form. Coverage / battle calculations respect the Mega form. Trainer Mega Evolution is out of scope for v1.

### B.1 Migration `012_mega_forms.sql`

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE pokemon_mega_forms (
    id                INTEGER PRIMARY KEY,       -- PokeAPI pokemon-form id
    base_pokemon_id   INTEGER NOT NULL REFERENCES pokemon(id),
    slug              TEXT NOT NULL UNIQUE,      -- "charizard-mega-x"
    display_name      TEXT NOT NULL,             -- "Mega Charizard X"
    mega_stone_slug   TEXT NOT NULL,             -- "charizardite-x"
    mega_stone_name   TEXT NOT NULL,             -- "Charizardite X"
    type1_id          INTEGER NOT NULL REFERENCES types(id),
    type2_id          INTEGER REFERENCES types(id),
    ability_id        INTEGER REFERENCES abilities(id),
    hp                INTEGER NOT NULL,
    attack            INTEGER NOT NULL,
    defense           INTEGER NOT NULL,
    special_attack    INTEGER NOT NULL,
    special_defense   INTEGER NOT NULL,
    speed             INTEGER NOT NULL,
    introduced_in_vg  INTEGER NOT NULL REFERENCES version_groups(id)
);

CREATE INDEX idx_mega_forms_base ON pokemon_mega_forms(base_pokemon_id);
CREATE INDEX idx_mega_forms_vg   ON pokemon_mega_forms(introduced_in_vg);

-- mega_form_id NULL = team member is not Mega-Evolved.
-- Non-NULL points to the chosen Mega form.
ALTER TABLE team_members ADD COLUMN mega_form_id INTEGER REFERENCES pokemon_mega_forms(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE team_members DROP COLUMN mega_form_id;
DROP TABLE pokemon_mega_forms;
-- +goose StatementEnd
```

**Why a separate table rather than rows in `pokemon`:** Mega forms share the national dex number with the base species; treating them as form variants preserves the one-row-per-species invariant relied on by the finder, evolution chain rendering, and encounter queries.

**Why `introduced_in_vg`:** XY introduced ~28 megas; ORAS later added ~18 more (including primals). Gating the Mega picker by version-group generation ensures that FRLG/HGSS playthroughs see no picker, and future ORAS-era megas don't leak into XY teams.

### B.2 Import step

New function `importer.ImportMegaForms(ctx, vgID)` invoked only for version groups with generation ≥ 6:

1. For each Pokémon in `pokemon` table, fetch `/api/v2/pokemon-species/{id}` and read `varieties`.
2. For each variety whose name ends in `-mega`, `-mega-x`, or `-mega-y`, fetch `/api/v2/pokemon/{variety-name}` for stats, types, and default ability.
3. Resolve the Mega Stone via PokéAPI's held-item data on the Mega variety (e.g., Mega Charizard X is held by `charizardite-x`).
4. Insert into `pokemon_mega_forms` with `introduced_in_vg = 16` for all XY-era megas.

### B.3 Queries (new, in `db/queries/mega_forms.sql`)

- `ListMegaFormsForPokemonInEra(pokemonID, maxGeneration)` — returns Mega forms whose `introduced_in_vg` generation ≤ current game's generation.
- `GetMegaForm(id)` — for detail rendering and effective-type resolution.

### B.4 Handler layer — `effectiveTypes` helper

A single helper (likely in `internal/handler/team.go` or a shared package) consolidates "what types does this team member effectively have right now?":

```go
func effectiveTypes(member TeamMember) []int64 {
    if member.MegaFormID.Valid {
        return []int64{member.MegaType1, member.MegaType2}  // type2 nullable
    }
    return []int64{member.Type1, member.Type2}  // base types, type2 nullable
}
```

Used by the existing `computeCoverage` and by the new rating engine in Phase C, so Mega support ripples through consistently.

### B.5 UI — team slot picker

The team slot edit form gains one `<select>`:

```
Mega Form: [ None ▾ ]
```

Visibility rules:
- Hidden unless the base Pokémon has at least one row in `pokemon_mega_forms` AND at least one has `introduced_in_vg ≤ current game's version group`.
- Options = "None" plus every matching Mega form, labeled as `Mega Charizard X (Charizardite X)`.

### B.6 Verification (Phase B)

- Team slot with Charizard + Charizardite X → coverage matrix treats Charizard as Fire/Dragon.
- Team slot with Pikachu → no Mega dropdown shown.
- FRLG / HGSS playthroughs → no Mega dropdown anywhere.
- Deleting a team member cascades correctly; switching Mega form updates the coverage partial via the existing HTMX trigger.

### B.7 Not in Phase B

- ORAS-era megas (added when ORAS ships).
- Primal Reversion (Groudon / Kyogre).
- Trainer Mega Evolution — Diantha's Mega Gardevoir and gym-leader Megas remain rendered as their base forms in battle helper analysis.
- Generalized held-item modeling — only the Mega Stone → Mega form relationship is represented.

---

## Phase C — Multi-Axis Coverage Rating

**Outcome:** A scorecard with three axes (Offensive, Defensive, Diversity), each 0–100, plus an overall weighted letter grade, replaces the gap matrix as the team builder's primary analytics surface. The matrix remains accessible as an expandable detail view. Works for all games; era-aware and Mega-aware.

### C.1 Package structure

```
internal/coverage/
  coverage.go       — public API: Rate(TeamSnapshot) Scorecard; types
  offensive.go      — offensive axis
  defensive.go      — defensive axis
  diversity.go      — diversity axis
  grade.go          — overall weighting and letter grade
  coverage_test.go  — table-driven tests, pure-function, no DB
```

### C.2 Input and output shapes

```go
type TeamSnapshot struct {
    Members  []Member
    Era      string                            // "pre_fairy" or "post_fairy"
    Efficacy map[int64]map[int64]float64       // attacker -> defender -> multiplier
    AllTypes []int64                           // 17 or 18 type IDs for the era
}

type Member struct {
    Name    string
    TypeIDs []int64          // effective types (Mega form if set); length 1 or 2
    Moves   []DamagingMove   // only moves with power > 0
}

type DamagingMove struct {
    Name   string
    TypeID int64
    Power  int
    IsSTAB bool              // precomputed: TypeID ∈ member.TypeIDs
}

type Scorecard struct {
    Offensive int          // 0..100
    Defensive int          // 0..100
    Diversity int          // 0..100
    Overall   int          // weighted
    Grade     string       // "A".."F"
    Issues    []Issue      // top problems, max ~5
}

type Issue struct {
    Axis     string   // "offensive" | "defensive" | "diversity"
    Severity string   // "critical" | "warning"
    Message  string
}
```

The existing `TeamHandler.HandleCoverage` is repurposed: it now constructs the snapshot from existing queries (team members + their types + their moves + era type efficacy), respects `mega_form_id` via `effectiveTypes`, invokes `coverage.Rate(snapshot)`, and renders the new `TeamScorecard` partial (which internally renders the matrix as its expandable detail). No new route or handler is introduced.

Status moves (power = 0) are excluded from the snapshot entirely. They are defensively noted as a deliberate non-feature in v1.

### C.3 Offensive axis

**Question:** Of the `numTypes` defender types, how many can your team hit super-effectively, and with STAB?

For each defender type `T`:

| Best outcome against T | Points |
|---|---|
| ≥2× **with STAB** | 3 |
| ≥2× without STAB | 2 |
| 1× (neutral) | 1 |
| <1× or immune | 0 |

`offensive = sum(points) / (3 × numTypes) × 100`.

**Issues flagged** (severity `critical`): any defender type scoring 0 → "No damaging move hits Rock".

### C.4 Defensive axis

**Question:** When an attacker uses type `T`, what's the team's *best* switch-in?

For each attacker type `T`, compute each member's incoming multiplier (product of `T → type1` and `T → type2`; if type2 is null, just type1). Take the team's **minimum** multiplier against `T`.

| Team's best vs T | Points |
|---|---|
| 0× (immune) | 4 |
| 0.25× (double resist) | 3 |
| 0.5× (resist) | 2 |
| 1× (neutral) | 1 |
| ≥2× (everyone weak) | 0 |

`defensive = sum(points) / (4 × numTypes) × 100`.

**Issues flagged** (severity `critical`): any attacker type where **3 or more** team members take ≥2× → "Shared weakness to Ice: 4 members take ≥2×".

### C.5 Diversity axis

**Question:** Are your damaging moves and your team types spread out, or stacked?

Two sub-scores averaged:

- **Move-type spread:** `unique_damaging_move_types / min(total_damaging_moves, numTypes)`.
- **Team-type spread:** `unique_team_types / min(2 × teamSize, numTypes)`.

`diversity = ((moveSpread + teamSpread) / 2) × 100`.

**Issues flagged** (severity `warning`): any move type appearing on ≥3 move slots → "4 of your moves are Fire". Any team type shared by ≥3 members → "Three members are Water-type".

### C.6 Overall grade

```
overall = round(0.40 * offensive + 0.40 * defensive + 0.20 * diversity)
```

| Overall | Grade |
|---|---|
| 90–100 | A |
| 80–89 | B |
| 70–79 | C |
| 60–69 | D |
| 0–59 | F |

Rationale for 40/40/20: Offense and defense are the real combat axes; diversity is a tiebreaker that prevents two teams with identical O/D profiles from looking identical.

### C.7 UI — scorecard component

New partial `TeamScorecard` replaces `TeamCoveragePartial` as the default team-builder analytics view. Existing 18×18 gap matrix becomes an expandable "See full coverage matrix" section below.

Rough layout:

```
┌─────────────────────────────────────────┐
│ Team Rating: B+ (83)                    │
├─────────────────────────────────────────┤
│ Offensive    ████████████░░  78         │
│ Defensive    █████████████░  86         │
│ Diversity    ███████████░░░  72         │
├─────────────────────────────────────────┤
│ Biggest issues                          │
│  ● No super-effective move vs Rock      │
│  ● 3 members weak to Ice                │
│  ○ 4 of your moves are Fire             │
└─────────────────────────────────────────┘
  ▸ See full coverage matrix
```

HTMX-driven partial, re-rendered on the same `hx-trigger` signals the current coverage partial listens to.

### C.8 Edge cases

- **Empty team (0 members):** render "Add Pokémon to see your rating"; skip scoring.
- **Member with no damaging moves:** contributes to defense, not offense. If 3+ members are in this state, flag as an issue.
- **Pre-Fairy era:** `numTypes = 17`; scoring denominator adjusts automatically.
- **All-status team:** Offensive = 0, Diversity move-spread denominator = 0 (guard against div/zero by treating as 0).

### C.9 Testing

Table-driven tests in `coverage_test.go` with hand-crafted snapshots:

- Known-bad team (6 mono-Water + only Water moves): very low offensive and very low diversity; high shared-weakness flags.
- Known-good balanced team: A or high B.
- Single-member team: valid, low diversity.
- Empty team: returns empty scorecard without panicking.
- Era-specific: same team under `pre_fairy` vs `post_fairy` — defensive scores differ where Steel resistances changed and where Fairy exists.

### C.10 Not in Phase C

- Scoring utility from status moves (Toxic, Spore, Stealth Rock).
- Speed tier analysis.
- Ability-effect-aware scoring (Levitate immunity, Flash Fire immunity, etc.). Folding abilities into the rating is its own design problem; the schema has ability data but v1 scores purely from types + moves.
- Coach-style suggestions ("swap X for Y") — `internal/suggest/` already covers this territory.

---

## Cross-Phase Concerns

### Regression protection

Existing tests (`internal/handler/team_test.go`, `internal/suggest/suggest_test.go`) may implicitly assume 17 types. Before merging Phase A, audit for hardcoded type counts or ID ranges and make them data-driven.

Phase C's `internal/coverage/` tests are pure functions — fast, deterministic, run on every `go test ./...`.

### Verification (end-to-end, after all three phases land)

- Start a fresh XY playthrough → game switcher shows "Pokémon X" alongside FRLG / HGSS.
- Detail page for Greninja (#658): base stats, abilities, evolution chain (Froakie → Frogadier → Greninja), XY-filtered learnable moves, catch locations.
- Team builder: add Charizard, pick Charizardite X → scorecard recomputes with Charizard scored as Fire/Dragon defensively and offensively.
- Battle helper: Viola matchup uses post-Fairy efficacy.
- Type chart with XY active: 18×18 with Fairy.
- Switch to an FRLG playthrough: 17×17 chart, no Mega dropdown, scorecard denominator = 17.

### Rollout — three PRs

1. **`feat/xy-game-support`** (Phase A): migration 011, badge JSON, trainer JSON, import refactor. Merges first; user can play XY with existing tooling immediately.
2. **`feat/mega-evolution`** (Phase B): migration 012, Mega import, UI picker, `effectiveTypes` helper. Merges second.
3. **`feat/coverage-rating`** (Phase C): `internal/coverage/` package, scorecard component, matrix becomes expandable detail. Merges third.

Each PR is independently shippable and individually reviewable.

---

## Explicit Non-Features

- ORAS, SM, USUM, or any post-XY 3DS game.
- Kalos regional Pokédex numbering / sub-dex filtering.
- Primal Reversion (Groudon / Kyogre).
- Trainer Mega Evolutions in the battle helper.
- Team Flare admin trainers and Lysandre.
- Ability-effect-aware coverage scoring.
- Status-move utility scoring.
- Speed-tier analysis.
- Cross-generation move data overrides (deferred per the existing multi-game spec).

---

## Critical Files

### New

- `db/migrations/011_xy_game.sql`
- `db/migrations/012_mega_forms.sql`
- `cmd/import/badges/xy.json`
- `db/seed/xy_trainers.json`
- `db/queries/mega_forms.sql`
- `internal/coverage/{coverage,offensive,defensive,diversity,grade,coverage_test}.go`
- `internal/view/team_scorecard.templ`

### Modified

- `cmd/import/main.go` — slug-driven seed-file discovery.
- `cmd/import/importer.go` — add `ImportMegaForms` step (only for generation ≥ 6).
- `internal/handler/team.go` — `effectiveTypes` helper; rating handler; scorecard rendering.
- `internal/view/team.templ` — swap gap matrix for scorecard; matrix becomes expandable detail.
- `README.md` — list XY as supported.
