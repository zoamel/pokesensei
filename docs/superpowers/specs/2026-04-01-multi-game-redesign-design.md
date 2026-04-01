# PokeSensei Multi-Game Redesign

## Context

PokeSensei currently supports 4 games (FireRed, LeafGreen, HeartGold, SoulSilver) with a single global game state. The goal is to transform it into a tool focused on 4 pillars:

1. **Pokedex** — Game-relevant detail (stats, types, abilities, catch locations, evolution chains, learnable moves)
2. **Team Manager** — One playthrough per game with full team customization
3. **Battle Helper** — Lookup tool for trainer matchups and wild Pokemon analysis
4. **Type Chart** — Generation-aware interactive reference

The app should support GBA, DS, and 3DS games (Gen III-VII), delivered incrementally starting with the existing FRLG/HGSS data. Adding a new game should require zero Go code changes.

---

## Architecture: Version-Group Scoped

### Core Principle

Mirror PokeAPI's two-level hierarchy:
- **Version group** (e.g., FRLG = group 7) — scopes shared mechanics: moves, learnsets, type chart era
- **Game version** (e.g., FireRed = 10) — scopes version-specific content: encounters, trainers

### Schema Changes

#### Migration 007: Version Groups & Multi-Game State

**New `version_groups` table:**

```sql
CREATE TABLE version_groups (
    id              INTEGER PRIMARY KEY,    -- matches PokeAPI ID
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    generation      INTEGER NOT NULL,
    max_pokedex     INTEGER NOT NULL,
    type_chart_era  TEXT NOT NULL CHECK (type_chart_era IN ('pre_fairy', 'post_fairy')),
    max_badges      INTEGER NOT NULL DEFAULT 8
);

-- Seed initial data
INSERT INTO version_groups (id, name, slug, generation, max_pokedex, type_chart_era) VALUES
    (7,  'FireRed / LeafGreen',    'frlg', 3, 386, 'pre_fairy'),
    (10, 'HeartGold / SoulSilver', 'hgss', 4, 493, 'pre_fairy');
```

**Update `game_versions`** — add FK to version_groups (column `version_group_id` already exists from migration 006):

```sql
-- No schema change needed, column exists. Just ensure data is correct.
```

**Update `game_state`** — multi-game support:

```sql
ALTER TABLE game_state ADD COLUMN is_active INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX idx_game_state_version ON game_state(game_version_id);
```

Behavior:
- One `game_state` row per `game_version_id` (enforced by unique index)
- Exactly one row has `is_active = 1` at any time
- Switching games: set current to `is_active = 0`, target to `is_active = 1`
- If target game has no row yet, redirect to onboarding for that game

#### Migration 008: Generation-Aware Type Efficacy

```sql
ALTER TABLE type_efficacy ADD COLUMN era TEXT NOT NULL DEFAULT 'pre_fairy';

-- Add Fairy type if not present (id = 18 in PokeAPI)
INSERT OR IGNORE INTO types (id, name, slug) VALUES (18, 'Fairy', 'fairy');

-- Import post_fairy efficacy matrix (separate INSERT statements)
-- Key differences from pre_fairy:
--   Fairy added: super effective vs Dragon/Dark/Fighting, weak vs Poison/Steel, immune to Dragon
--   Steel: no longer resists Ghost or Dark
```

Two sets of rows in `type_efficacy`:
- `era = 'pre_fairy'`: 17x17 matrix (current data, no Fairy)
- `era = 'post_fairy'`: 18x18 matrix (with Fairy, updated Steel)

### Game Context Middleware

New package `internal/gamecontext/`:

```go
type GameContext struct {
    GameStateID    int64
    GameVersionID  int64
    VersionGroupID int64
    Generation     int
    TypeChartEra   string  // "pre_fairy" or "post_fairy"
    BadgeCount     int
    TradingEnabled bool
    MaxPokedex     int
}

// FromRequest extracts GameContext from request context.
func FromRequest(r *http.Request) (GameContext, bool)

// Middleware loads the active game state and injects GameContext.
// Returns 302 to /onboarding if no active game state exists.
func Middleware(store Store) func(http.Handler) http.Handler
```

Routes that use this middleware: `/dashboard`, `/pokemon/*`, `/team/*`, `/battle/*`, `/settings`.
Routes that skip it: `/health`, `/static/*`, `/onboarding` (creates game state).

### Query Changes

**game_state.sql** — replace `GetGameState` with:
- `GetActiveGameState` — `WHERE is_active = 1 LIMIT 1`
- `GetGameStateForVersion(gameVersionID)` — for switching
- `CreateGameState(gameVersionID, starterID, badgeCount, trading)` — unchanged except sets `is_active = 1`
- `SwitchActiveGame(gameStateID)` — deactivate all, activate target
- `ListGameStates` — all playthroughs with game version info (for game switcher UI)
- Keep existing update/delete queries

**types.sql** — add era parameter:
- `GetTypeEfficacy(era)` — filter by era
- `GetTypeEfficacyForAttacker(typeID, era)` — filter by era
- `ListTypesByEra(era)` — exclude Fairy for pre_fairy

**pokemon.sql** — add max_pokedex filter:
- `ListAllPokemon(maxDex)` — `WHERE id <= ?`
- `SearchPokemonFiltered` — add `AND p.id <= ?` condition

No changes needed for: moves, abilities, natures, encounters, evolution, team_members (already scoped by game_state_id).

---

## Feature 1: Pokedex (Game-Relevant Detail)

### Current State
- Pokemon finder with search/filter by name, type, badge
- Detail page: base stats, abilities, Bulbapedia link

### Changes
Restore and enhance the detail page with game-relevant information:

**Detail page sections:**
1. **Header** — Sprite, name, national dex #, type pills
2. **Base Stats** — Bar chart (unchanged)
3. **Abilities** — Name, description, hidden ability indicator (unchanged)
4. **Evolution Chain** — Visual chain with trigger methods (level, trade, item). Data from `evolution_steps` table. Highlight the current Pokemon in the chain.
5. **Learnable Moves** — Table filtered by current game's version group. Columns: move name, type, power, accuracy, PP, damage class, learn method, level. Grouped by learn method (level-up, TM/HM, tutor, egg).
6. **Catch Locations** — Where to find this Pokemon in the current game. From `encounters` table. Columns: location, method, chance %, level range, badge required.
7. **Bulbapedia Link** — External reference (unchanged)

**Queries to add/restore:**
- `GetEvolutionChainByPokemon(pokemonID)` — already exists
- `ListPokemonMoves(pokemonID, versionGroupID)` — already exists
- `ListEncountersByPokemon(pokemonID, gameVersionID)` — already exists

These queries exist but their results aren't shown on the detail page (removed during the trim). Restore the templ components.

**Finder page** — add `maxPokedex` filter so only Pokemon available in the current game's generation appear.

---

## Feature 2: Team Manager (Per-Game Playthroughs)

### Current State
- 6 slots with level, nature, ability, 4 moves
- Type coverage matrix with gap warnings
- Suggestions engine (current vs planned)
- Single game state

### Changes

**Game Switcher UI:**
- New component in the navigation or dashboard: dropdown/list showing all games with existing playthroughs
- Each entry shows: game name, team size (e.g., "3/6"), badge count
- "Start New Game" option leads to onboarding for a new game version
- Switching to a game **with an existing playthrough**: updates `is_active` and reloads the page (instant)
- Switching to a game **without a playthrough**: redirects to onboarding flow for that game (choose starter, set badge count)

**Team builder** — no functional changes. Already scoped by `game_state_id`. The middleware ensures the correct game state is loaded.

**Badge count** — currently limited to 0-8 via `CHECK (badge_count BETWEEN 0 AND 8)`. Some games (HGSS) have 16 badges (Johto + Kanto). Migration 007 should relax this to `CHECK (badge_count BETWEEN 0 AND 16)`. The `version_groups` table should include a `max_badges` column so the UI renders the correct number of badge slots per game.

### No Schema Changes
The existing `team_members` and `team_member_moves` tables are already scoped by `game_state_id`, which is per-game. Multi-game support comes from the game_state changes in Migration 007.

---

## Feature 3: Battle Helper (Lookup Tool)

### Current State
- Trainer list filtered by game
- Matchup analysis: ranks team members vs opponent
- STAB and type effectiveness considered
- Pokemon search for arbitrary opponents
- 68 trainers (FRLG + HGSS gym leaders, E4, champions, rivals)

### Changes

**Polish, not redesign.** The battle helper already does what's needed. Changes:

1. **Generation-aware matchups** — Use `type_chart_era` from GameContext when calculating effectiveness. Currently uses the global (single-era) efficacy matrix.

2. **Move recommendations** — Currently shows the best move per team member. Ensure move power/accuracy reflect the correct game version (for Gen III-V this is identical; becomes important at Gen VI+ if `move_overrides` table is added later).

3. **Trainer organization** — Group trainers by progression: Gym Leaders (by badge #), Elite Four, Champion, Rival. Already mostly done but ensure consistent ordering.

4. **Random opponent** — Add a "Random Pokemon" button that picks a random Pokemon from the current game's available pool (filtered by badge count). Useful for testing team coverage against unexpected encounters.

---

## Feature 4: Type Effectiveness Chart (Per-Game Reference)

### Current State
- Interactive 18x18 type chart at `/battle/types`
- Click to highlight attacking/defending matchups

### Changes

1. **Era-aware rendering** — Query `type_efficacy WHERE era = ?` and `types` filtered by era
   - Pre-Fairy games: 17x17 grid (no Fairy row/column)
   - Post-Fairy games: 18x18 grid

2. **Visual indicator** — Show which game/generation the chart applies to (e.g., "Type Chart — Gen III (FireRed)")

3. **Legend** — Add a legend explaining the symbols/colors (super effective, not very effective, no effect, neutral). Already has color coding but an explicit legend helps as a learning reference.

4. **Standalone access** — The chart should work without an active game state (default to the latest era). Useful as a quick reference.

---

## Data-Driven Import Pipeline

### Current State
- Hardcoded `VersionGroups` map in `cmd/import/gamedata.go`
- Hardcoded `BadgeForRouteFRLG` and `BadgeForRouteHGSS` Go maps
- CLI flags: `--games`, `--max-dex`, `--seed-trainers`
- Imports from PokeAPI: Pokemon, types, moves, learnsets, encounters, evolution chains
- Seeds trainers from JSON files in `db/seed/`

### Changes

**Version groups from DB:**
- The importer reads `version_groups` table instead of the hardcoded Go map
- `game_versions` table provides the PokeAPI version IDs
- `max_pokedex` from `version_groups` replaces `--max-dex` flag

**Badge maps to JSON:**
- Move `BadgeForRouteFRLG` and `BadgeForRouteHGSS` from Go code to JSON files:
  - `cmd/import/badges/frlg.json`
  - `cmd/import/badges/hgss.json`
- Format: `{"pallet-town": 0, "kanto-route-1": 0, ...}`
- The importer loads the appropriate badge file by version group slug

**Adding a new game (e.g., Ruby/Sapphire/Emerald):**
1. SQL: Insert into `version_groups` (id=5, slug='rse', gen=3, max_dex=386, pre_fairy)
2. SQL: Insert into `game_versions` for Ruby (id=7), Sapphire (id=8), Emerald (id=9)
3. Create `cmd/import/badges/rse.json` with route-to-badge mapping
4. Create `db/seed/rse_trainers.json` with gym leaders, E4, champion data
5. Run `go run ./cmd/import --games=rse --seed-trainers`

**Zero Go code changes required.**

### Future: Move Overrides (Gen V+)

When adding Gen V+ games, some moves changed power/accuracy/type across generations. A `move_overrides` table can handle this:

```sql
CREATE TABLE move_overrides (
    move_id          INTEGER NOT NULL REFERENCES moves(id),
    version_group_id INTEGER NOT NULL REFERENCES version_groups(id),
    power            INTEGER,
    accuracy         INTEGER,
    type_id          INTEGER REFERENCES types(id),
    PRIMARY KEY (move_id, version_group_id)
);
```

This is **deferred** — not needed for the initial FRLG/HGSS scope where move stats are identical. Add when the first Gen V+ game is imported.

---

## Delivery Phases

### Phase 1: Multi-Game Foundation
- Migration 007 (version_groups, multi-game state)
- Migration 008 (era-aware type efficacy)
- Game context middleware
- Update all handlers to use GameContext
- Game switcher UI
- Update queries (game state, types, pokemon)

### Phase 2: Pokedex Enhancement
- Restore evolution chain component on detail page
- Restore learnable moves table (filtered by version group)
- Restore catch locations (filtered by game version)
- Add max_pokedex filter to finder

### Phase 3: Battle Helper Polish
- Generation-aware matchup calculations
- Random opponent button
- Trainer organization improvements

### Phase 4: Type Chart & Reference
- Era-aware type chart rendering
- Chart legend
- Standalone access without active game

### Phase 5: Data Pipeline Refactor
- Version groups from DB
- Badge maps to JSON files
- Update import CLI

### Phase 6+: New Games
- Add RSE (Ruby/Sapphire/Emerald)
- Add DPPt (Diamond/Pearl/Platinum)
- Add BW/B2W2 (Black/White/Black 2/White 2)
- Add XY (first post_fairy game)
- Add ORAS, SM, USUM

---

## Verification

### Multi-Game State
- Create game state for FireRed, switch to HeartGold, switch back — team/badges preserved
- Start onboarding for a new game — creates new game_state row
- Only one `is_active = 1` row at any time

### Pokedex
- Detail page shows evolution chain, moves (filtered by game), catch locations
- Finder only shows Pokemon up to current game's max pokedex number
- Switching games changes which moves/locations appear

### Battle Helper
- Trainer matchups use correct type chart era
- "Random Pokemon" picks from current game's available pool
- Move recommendations show correct move data

### Type Chart
- FRLG shows 17x17 chart (no Fairy)
- Future Gen VI game would show 18x18 chart
- Chart works without active game state

### Import Pipeline
- `go run ./cmd/import --games=frlg` reads version_groups from DB
- Adding a new game requires only data files + SQL inserts
- Badge maps load from JSON instead of Go code

---

## Critical Files

### Schema & Queries
- `db/migrations/004_game_state.sql` — current game state schema
- `db/migrations/006_team_member_details.sql` — version_group_id on game_versions
- `db/queries/game_state.sql` — all game state queries (need multi-game versions)
- `db/queries/types.sql` — type efficacy queries (need era parameter)
- `db/queries/pokemon.sql` — pokemon queries (need max_pokedex filter)

### Handlers
- `internal/handler/root.go` — redirect logic (needs active game awareness)
- `internal/handler/dashboard.go` — dashboard (needs game switcher)
- `internal/handler/battle.go` — matchup calculations (needs era-aware efficacy)
- `internal/handler/team.go` — team builder (already game-scoped, minimal changes)
- `internal/handler/pokemon.go` — pokedex (needs detail page enhancements)
- `internal/handler/settings.go` — settings (needs multi-game awareness)
- `internal/handler/onboarding.go` — onboarding (needs per-game flow)

### Views
- `internal/view/pokemon_detail.templ` — detail page (add evolution, moves, locations)
- `internal/view/battle.templ` — battle page (random opponent button)
- `internal/view/dashboard.templ` — dashboard (game switcher component)
- `internal/view/layout.templ` — nav (active game indicator)

### Import Pipeline
- `cmd/import/gamedata.go` — hardcoded maps (replace with DB/JSON)
- `cmd/import/importer.go` — import orchestration (read version_groups from DB)
- `cmd/import/main.go` — CLI flags (update for new pipeline)

### New Files
- `internal/gamecontext/context.go` — GameContext struct and helpers
- `internal/gamecontext/middleware.go` — middleware implementation
- `cmd/import/badges/frlg.json` — badge route mapping
- `cmd/import/badges/hgss.json` — badge route mapping
