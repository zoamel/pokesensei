# Team Member Detail Configuration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add moves (4 slots), ability, and nature configuration per team member via a dedicated detail page.

**Architecture:** New migration extends `team_members` + adds `team_member_moves` table. New sqlc queries for CRUD. New handler methods on `TeamHandler` with HTMX partial responses. New templ components for the detail page. CSS additions for the detail page layout.

**Tech Stack:** Go 1.25, SQLite (modernc.org/sqlite), sqlc, goose, templ, HTMX, Alpine.js

**Spec:** `docs/superpowers/specs/2026-03-20-team-member-details-design.md`

---

### Task 1: Database Migration

**Files:**
- Create: `db/migrations/006_team_member_details.sql`

- [ ] **Step 1: Create the migration file**

```sql
-- +goose Up

-- Bridge game_version_id → version_group_id for move lookups.
ALTER TABLE game_versions ADD COLUMN version_group_id INTEGER;
UPDATE game_versions SET version_group_id = 7 WHERE id IN (10, 11);
UPDATE game_versions SET version_group_id = 10 WHERE id IN (15, 16);

-- Extend team_members with nature and ability.
ALTER TABLE team_members ADD COLUMN nature_id INTEGER REFERENCES natures(id);
ALTER TABLE team_members ADD COLUMN ability_id INTEGER REFERENCES abilities(id);

-- Move slots (up to 4 per team member).
CREATE TABLE team_member_moves (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    team_member_id INTEGER NOT NULL REFERENCES team_members(id) ON DELETE CASCADE,
    move_id        INTEGER NOT NULL REFERENCES moves(id),
    slot           INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    UNIQUE (team_member_id, slot),
    UNIQUE (team_member_id, move_id)
);

-- +goose Down
DROP TABLE IF EXISTS team_member_moves;
-- SQLite does not support DROP COLUMN before 3.35; these columns are harmless if left.
```

- [ ] **Step 2: Run the migration**

Run: `make migrate`
Expected: Migration 006 applies without errors.

- [ ] **Step 3: Verify schema**

Run: `sqlite3 data/pokesensei.db ".schema team_members" && sqlite3 data/pokesensei.db ".schema team_member_moves" && sqlite3 data/pokesensei.db "SELECT id, name, version_group_id FROM game_versions;"`
Expected: `team_members` has `nature_id` and `ability_id` columns. `team_member_moves` table exists. `game_versions` shows version_group_id values (7 for FRLG versions, 10 for HGSS versions).

- [ ] **Step 4: Commit**

```bash
git add db/migrations/006_team_member_details.sql
git commit -m "feat: add migration for team member details (nature, ability, moves)"
```

---

### Task 2: SQL Queries for Team Member Details

**Files:**
- Modify: `db/queries/team_members.sql`
- Modify: `db/queries/game_versions.sql` (add `version_group_id` to existing queries)
- Create: `db/queries/natures.sql`

- [ ] **Step 1: Update existing game_versions queries**

The migration adds `version_group_id` to `game_versions`. Existing queries in `db/queries/game_versions.sql` select only `id, name, slug`. After `sqlc generate`, the `GameVersion` model gains `VersionGroupID sql.NullInt64`, but the existing queries won't select it. Update `db/queries/game_versions.sql` to select all columns:

```sql
-- name: ListGameVersions :many
SELECT id, name, slug, version_group_id
FROM game_versions
ORDER BY id;

-- name: GetGameVersionBySlug :one
SELECT id, name, slug, version_group_id
FROM game_versions
WHERE slug = ?1;

-- name: GetVersionGroupIDByGameVersion :one
SELECT version_group_id
FROM game_versions
WHERE id = ?1;
```

The new `GetVersionGroupIDByGameVersion` query takes a `game_version_id` directly (available from the already-fetched `GameState.GameVersionID`), avoiding a redundant join through `game_state`.

- [ ] **Step 2: Add new queries to `db/queries/team_members.sql`**

Append these queries:

```sql
-- name: GetTeamMemberDetail :one
SELECT tm.id, tm.pokemon_id, tm.level, tm.slot, tm.is_locked,
       tm.nature_id, tm.ability_id,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense, p.base_sp_atk, p.base_sp_def, p.base_speed,
       n.name AS nature_name, n.increased_stat, n.decreased_stat,
       a.name AS ability_name
FROM team_members tm
JOIN pokemon p ON p.id = tm.pokemon_id
LEFT JOIN natures n ON n.id = tm.nature_id
LEFT JOIN abilities a ON a.id = tm.ability_id
WHERE tm.id = ?1;

-- name: SetTeamMemberNature :exec
UPDATE team_members SET nature_id = ?1 WHERE id = ?2;

-- name: SetTeamMemberAbility :exec
UPDATE team_members SET ability_id = ?1 WHERE id = ?2;

-- name: ListTeamMemberMoves :many
SELECT tmm.id, tmm.slot, tmm.move_id,
       m.name AS move_name, m.slug AS move_slug, m.power, m.accuracy, m.pp,
       m.damage_class, m.effect,
       t.name AS type_name, t.slug AS type_slug
FROM team_member_moves tmm
JOIN moves m ON m.id = tmm.move_id
LEFT JOIN types t ON t.id = m.type_id
WHERE tmm.team_member_id = ?1
ORDER BY tmm.slot;

-- name: AddTeamMemberMove :one
INSERT INTO team_member_moves (team_member_id, move_id, slot)
VALUES (?1, ?2, ?3)
RETURNING id, team_member_id, move_id, slot;

-- name: RemoveTeamMemberMove :exec
DELETE FROM team_member_moves WHERE id = ?1;

-- name: ListAvailableMoves :many
SELECT DISTINCT m.id, m.name, m.slug, m.power, m.accuracy, m.pp,
       m.damage_class, m.effect, pm.learn_method, pm.level_learned_at,
       t.name AS type_name, t.slug AS type_slug
FROM pokemon_moves pm
JOIN moves m ON m.id = pm.move_id
LEFT JOIN types t ON t.id = m.type_id
WHERE pm.pokemon_id = ?1
  AND pm.version_group_id = ?2
  AND (
    (pm.learn_method = 'level-up' AND pm.level_learned_at <= ?3)
    OR pm.learn_method != 'level-up'
  )
ORDER BY m.name;
```

**Note on `ListAvailableMoves`:** The `DISTINCT` with `pm.learn_method` and `pm.level_learned_at` means the same move may appear multiple times if learnable by different methods (e.g., both level-up and TM). This is intentional — it shows all ways to learn a move. The template deduplicates visually by disabling "Add" for already-assigned moves via the `assignedIDs` set.

- [ ] **Step 3: Create `db/queries/natures.sql`**

```sql
-- name: ListNatures :many
SELECT id, name, slug, increased_stat, decreased_stat
FROM natures
ORDER BY name;
```

- [ ] **Step 4: Run sqlc generate**

Run: `sqlc generate`
Expected: No errors. New files/types appear in `db/generated/`. Existing `GameVersion` model gains `VersionGroupID sql.NullInt64`. New `TeamMemberMove` model generated from the new table.

- [ ] **Step 5: Verify generated code compiles**

Run: `go build ./...`
Expected: Clean build. If any existing code references `GameVersion` fields that changed, fix those (unlikely since existing queries already use Row types, but verify).

- [ ] **Step 6: Commit**

```bash
git add db/queries/team_members.sql db/queries/game_versions.sql db/queries/natures.sql db/generated/
git commit -m "feat: add sqlc queries for team member details"
```

---

### Task 3: View Types and Templ Components

**Files:**
- Modify: `internal/view/types.go` (add `TeamMemberDetailData`)
- Create: `internal/view/team_detail.templ` (detail page + partials)

- [ ] **Step 1: Add view data types to `internal/view/types.go`**

Add these types:

```go
// TeamMemberDetailData holds all data for the team member detail page.
type TeamMemberDetailData struct {
	Member    generated.GetTeamMemberDetailRow
	Types     []TypeInfo
	Natures   []generated.ListNaturesRow
	Abilities []generated.ListPokemonAbilitiesRow
	Moves     []generated.ListTeamMemberMovesRow
	Available []generated.ListAvailableMovesRow
	AssignedMoveIDs map[int64]bool // set of move_ids already assigned, for disabling in UI
}

// MoveSlotData holds data for rendering a single move slot.
type MoveSlotData struct {
	SlotNum int
	Move    *generated.ListTeamMemberMovesRow // nil if slot is empty
}
```

- [ ] **Step 2: Create `internal/view/team_detail.templ`**

Create the detail page template with these components:

```
templ TeamMemberDetailPage(data TeamMemberDetailData)
  - AppShell wrapper with title "[Pokemon Name] — Team Config" and activePage "team"
  - "← Back to Team" link to /team
  - Header: sprite, name, type pills, level input (PATCH /team/members/{id}), Bulbapedia link
  - Nature picker section (id="nature-section")
  - Ability picker section (id="ability-section")
  - Moves section (id="moves-section") containing:
    - TeamMemberMoveSlotsPartial (the 4 move slots)
    - Available moves list with search filter (Alpine.js x-data for filtering)
  - Stat summary section (id="stat-summary")

templ TeamMemberMoveSlotsPartial(memberID int64, slots []MoveSlotData)
  - 4 move slots, each showing move name + type pill + power/acc/class + "×" remove button
  - Empty slots show "Empty" label

templ TeamMemberAvailableMovesPartial(memberID int64, moves []generated.ListAvailableMovesRow, assignedIDs map[int64]bool, moveCount int)
  - Table/list of available moves with: name, type pill, power, accuracy, class, learn method
  - "Add" button per row: POST /team/members/{memberID}/moves with move_id as form value
  - Button disabled if moveCount >= 4 or move already assigned (in assignedIDs)

templ TeamMemberStatSummary(member generated.GetTeamMemberDetailRow)
  - Base stats bar chart or list
  - If nature is set, highlight boosted stat (green) and reduced stat (red)
  - Stat name mapping: "attack"→base_attack, "defense"→base_defense,
    "special-attack"→base_sp_atk, "special-defense"→base_sp_def, "speed"→base_speed
```

**Nullable field handling:** The `GetTeamMemberDetailRow` will have `sql.NullInt64` for `NatureID`/`AbilityID` and `sql.NullString` for `NatureName`, `IncreasedStat`, `DecreasedStat`, `AbilityName` (from LEFT JOINs). In templ templates, use `.Valid` checks before accessing `.String`/`.Int64` values. For example, the nature dropdown should check `data.Member.NatureID.Valid` to set the selected option, and the stat summary should check `data.Member.IncreasedStat.Valid` before highlighting.

HTMX behavior:
- Nature `<select>` → `hx-patch="/team/members/{id}/nature"` `hx-trigger="change"` `hx-target="#stat-summary"` `hx-swap="innerHTML"` (swaps stat summary to show nature effect)
- Ability `<select>` → `hx-patch="/team/members/{id}/ability"` `hx-trigger="change"` `hx-swap="none"`
- Move "Add" button → `hx-post="/team/members/{id}/moves"` `hx-target="#moves-section"` `hx-swap="innerHTML"` (refreshes both slots and available list)
- Move "×" button → `hx-delete="/team/members/{id}/moves/{tmMoveId}"` `hx-target="#moves-section"` `hx-swap="innerHTML"`
- Level input → `hx-patch="/team/members/{id}"` `hx-trigger="change"` `hx-swap="none"` (same as team page); the available moves partial listens for `team-updated from:body` to refresh

- [ ] **Step 3: Run templ generate**

Run: `templ generate`
Expected: `internal/view/team_detail_templ.go` generated without errors.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Clean build. (Handler doesn't use these yet, but types and templates compile.)

- [ ] **Step 5: Commit**

```bash
git add internal/view/types.go internal/view/team_detail.templ internal/view/team_detail_templ.go
git commit -m "feat: add templ components for team member detail page"
```

---

### Task 4: Handler Methods

**Files:**
- Modify: `internal/handler/team.go` (expand `TeamStore` interface, add handler methods)

- [ ] **Step 1: Expand the `TeamStore` interface**

Add these methods to the `TeamStore` interface in `internal/handler/team.go`:

```go
GetTeamMemberDetail(ctx context.Context, id int64) (generated.GetTeamMemberDetailRow, error)
ListNatures(ctx context.Context) ([]generated.ListNaturesRow, error)
ListPokemonAbilities(ctx context.Context, pokemonID int64) ([]generated.ListPokemonAbilitiesRow, error)
ListTeamMemberMoves(ctx context.Context, teamMemberID int64) ([]generated.ListTeamMemberMovesRow, error)
ListAvailableMoves(ctx context.Context, arg generated.ListAvailableMovesParams) ([]generated.ListAvailableMovesRow, error)
GetVersionGroupIDByGameVersion(ctx context.Context, id int64) (sql.NullInt64, error)
SetTeamMemberNature(ctx context.Context, arg generated.SetTeamMemberNatureParams) error
SetTeamMemberAbility(ctx context.Context, arg generated.SetTeamMemberAbilityParams) error
AddTeamMemberMove(ctx context.Context, arg generated.AddTeamMemberMoveParams) (generated.TeamMemberMove, error)
RemoveTeamMemberMove(ctx context.Context, id int64) error
```

Note: `GetVersionGroupIDByGameVersion` returns `sql.NullInt64` because `version_group_id` is nullable. `TeamMemberMove` is a new model generated from the `team_member_moves` table after running `sqlc generate` in Task 2.

- [ ] **Step 2: Add `HandleDetail` method (GET /team/members/{id})**

```go
func (h *TeamHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	member, err := h.store.GetTeamMemberDetail(ctx, id)
	if err != nil {
		h.log.Error("failed to get team member detail", "error", err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Fetch all data needed for the detail page.
	// Log errors but render page with partial data.
	types, err := h.store.GetPokemonWithTypes(ctx, member.PokemonID)
	if err != nil {
		h.log.Error("failed to get pokemon types", "error", err)
	}
	natures, err := h.store.ListNatures(ctx)
	if err != nil {
		h.log.Error("failed to list natures", "error", err)
	}
	abilities, err := h.store.ListPokemonAbilities(ctx, member.PokemonID)
	if err != nil {
		h.log.Error("failed to list abilities", "error", err)
	}
	moves, err := h.store.ListTeamMemberMoves(ctx, id)
	if err != nil {
		h.log.Error("failed to list member moves", "error", err)
	}

	// Get version group for available moves using the game state's version ID.
	gs, _ := h.store.GetGameState(ctx)
	var available []generated.ListAvailableMovesRow
	if gs.GameVersionID.Valid {
		vgID, err := h.store.GetVersionGroupIDByGameVersion(ctx, gs.GameVersionID.Int64)
		if err != nil {
			h.log.Error("failed to get version group", "error", err)
		}
		if vgID.Valid {
			available, _ = h.store.ListAvailableMoves(ctx, generated.ListAvailableMovesParams{
				PokemonID:      member.PokemonID,
				VersionGroupID: vgID.Int64,
				LevelLearnedAt: member.Level,
			})
		}
	}

	// Build assigned move IDs set
	assignedIDs := make(map[int64]bool)
	for _, m := range moves {
		assignedIDs[m.MoveID] = true
	}

	// Build type info
	var typeInfos []view.TypeInfo
	for _, t := range types {
		typeInfos = append(typeInfos, view.TypeInfo{ID: t.TypeID, Name: t.TypeName, Slug: t.TypeSlug})
	}

	data := view.TeamMemberDetailData{
		Member:          member,
		Types:           typeInfos,
		Natures:         natures,
		Abilities:       abilities,
		Moves:           moves,
		Available:       available,
		AssignedMoveIDs: assignedIDs,
	}

	view.TeamMemberDetailPage(data).Render(ctx, w)
}
```

Adapt this pseudocode to handle nullable `vgID` based on the actual generated type.

- [ ] **Step 3: Add `HandleSetNature` method (PATCH /team/members/{id}/nature)**

```go
func (h *TeamHandler) HandleSetNature(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Parse nature_id — empty string means clear
	var natureID sql.NullInt64
	if v := r.FormValue("nature_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			http.Error(w, "Invalid nature ID", http.StatusBadRequest)
			return
		}
		natureID = sql.NullInt64{Int64: n, Valid: true}
	}

	if err := h.store.SetTeamMemberNature(ctx, generated.SetTeamMemberNatureParams{
		NatureID: natureID,
		ID:       id,
	}); err != nil {
		h.log.Error("failed to set nature", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Re-fetch member and render stat summary partial
	member, _ := h.store.GetTeamMemberDetail(ctx, id)
	view.TeamMemberStatSummary(member).Render(ctx, w)
}
```

- [ ] **Step 4: Add `HandleSetAbility` method (PATCH /team/members/{id}/ability)**

Same pattern as nature — parse `ability_id` from form, call `SetTeamMemberAbility`, return `hx-trigger: team-updated` or just 200 OK with `hx-swap="none"`.

- [ ] **Step 5: Add `HandleAddMove` method (POST /team/members/{id}/moves)**

```go
func (h *TeamHandler) HandleAddMove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	moveID, err := strconv.ParseInt(r.FormValue("move_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid move ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Find lowest available slot
	existingMoves, _ := h.store.ListTeamMemberMoves(ctx, memberID)
	occupied := make(map[int]bool)
	for _, m := range existingMoves {
		occupied[int(m.Slot)] = true
	}
	slot := 0
	for s := 1; s <= 4; s++ {
		if !occupied[s] {
			slot = s
			break
		}
	}
	if slot == 0 {
		http.Error(w, "All 4 move slots are full", http.StatusConflict)
		return
	}

	if _, err := h.store.AddTeamMemberMove(ctx, generated.AddTeamMemberMoveParams{
		TeamMemberID: memberID,
		MoveID:       moveID,
		Slot:         int64(slot),
	}); err != nil {
		h.log.Error("failed to add move", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return updated moves section partial
	h.renderMovesSection(w, r, memberID)
}
```

- [ ] **Step 6: Add `HandleRemoveMove` method (DELETE /team/members/{id}/moves/{tmMoveId})**

```go
func (h *TeamHandler) HandleRemoveMove(w http.ResponseWriter, r *http.Request) {
	tmMoveID, err := strconv.ParseInt(r.PathValue("tmMoveId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid move ID", http.StatusBadRequest)
		return
	}

	memberID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err := h.store.RemoveTeamMemberMove(r.Context(), tmMoveID); err != nil {
		h.log.Error("failed to remove move", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderMovesSection(w, r, memberID)
}
```

- [ ] **Step 7: Add `renderMovesSection` helper**

Private helper that re-fetches moves + available moves and renders the moves section partial. Used by both `HandleAddMove` and `HandleRemoveMove` to return the updated partial.

```go
func (h *TeamHandler) renderMovesSection(w http.ResponseWriter, r *http.Request, memberID int64) {
	ctx := r.Context()
	member, _ := h.store.GetTeamMemberDetail(ctx, memberID)
	moves, _ := h.store.ListTeamMemberMoves(ctx, memberID)

	gs, _ := h.store.GetGameState(ctx)
	var available []generated.ListAvailableMovesRow
	if gs.GameVersionID.Valid {
		vgID, _ := h.store.GetVersionGroupIDByGameVersion(ctx, gs.GameVersionID.Int64)
		if vgID.Valid {
			available, _ = h.store.ListAvailableMoves(ctx, generated.ListAvailableMovesParams{
				PokemonID:      member.PokemonID,
				VersionGroupID: vgID.Int64,
				LevelLearnedAt: member.Level,
			})
		}
	}

	assignedIDs := make(map[int64]bool)
	for _, m := range moves {
		assignedIDs[m.MoveID] = true
	}

	// Build MoveSlotData for 4 slots
	slots := buildMoveSlots(moves)

	// Render the moves section partial (slots + available list)
	view.TeamMemberMoveSlotsPartial(memberID, slots).Render(ctx, w)
	view.TeamMemberAvailableMovesPartial(memberID, available, assignedIDs, len(moves)).Render(ctx, w)
}
```

- [ ] **Step 8: Verify build**

Run: `go build ./...`
Expected: Clean build. Adapt any type mismatches from generated code.

- [ ] **Step 9: Commit**

```bash
git add internal/handler/team.go
git commit -m "feat: add handler methods for team member detail page"
```

---

### Task 5: Route Registration

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add new routes after the existing team routes**

In `cmd/server/main.go`, after line 102 (`srv.Handle("PATCH /team/members/{id}", ...)`), add:

```go
// Team Member Detail
srv.Handle("GET /team/members/{id}", http.HandlerFunc(teamHandler.HandleDetail))
srv.Handle("PATCH /team/members/{id}/nature", http.HandlerFunc(teamHandler.HandleSetNature))
srv.Handle("PATCH /team/members/{id}/ability", http.HandlerFunc(teamHandler.HandleSetAbility))
srv.Handle("POST /team/members/{id}/moves", http.HandlerFunc(teamHandler.HandleAddMove))
srv.Handle("DELETE /team/members/{id}/moves/{tmMoveId}", http.HandlerFunc(teamHandler.HandleRemoveMove))
```

**Note:** `GET /team/members/{id}` coexists with the existing `DELETE /team/members/{id}` and `PATCH /team/members/{id}` because Go 1.22+ ServeMux differentiates routes by HTTP method. No conflict.

- [ ] **Step 2: Verify build and server starts**

Run: `go build ./cmd/server/ && echo "build ok"`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register team member detail routes"
```

---

### Task 6: Team Overview Page — Add Configure Links

**Files:**
- Modify: `internal/view/team.templ`

- [ ] **Step 1: Add "Configure" link to filled team slots**

In the `teamBuilderSlot` component, add a "Configure" link next to the Pokémon name or below the controls. After the Bulbapedia link (around line 66), add:

```
<a href={ templ.SafeURL(fmt.Sprintf("/team/members/%d", slot.Member.ID)) } class="btn btn-outline btn-sm">Configure</a>
```

- [ ] **Step 2: Run templ generate**

Run: `templ generate`
Expected: No errors.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 4: Commit**

```bash
git add internal/view/team.templ internal/view/team_templ.go
git commit -m "feat: add configure link to team builder slots"
```

---

### Task 7: CSS Styles for Detail Page

**Files:**
- Modify: `static/css/components.css` (or create `static/css/team-detail.css` and import it in `main.css`)

- [ ] **Step 1: Check existing CSS structure**

Read `static/css/main.css` to understand the `@import` and `@layer` pattern. Add styles in the same pattern.

- [ ] **Step 2: Add detail page styles**

Add styles for:
- `.member-detail` — page layout container
- `.member-detail-header` — sprite + name + types + level row
- `.member-detail-section` — reusable section wrapper (nature, ability, moves, stats)
- `.nature-select`, `.ability-select` — dropdown styling (likely inherit from existing form styles)
- `.move-slots` — 4-slot grid/list for assigned moves
- `.move-slot` — individual move slot with type pill, stats, remove button
- `.move-slot-empty` — empty slot state
- `.available-moves` — scrollable list of available moves
- `.available-move-row` — individual available move with add button
- `.move-search` — search/filter input for moves
- `.stat-bar` — base stat bar for the stat summary
- `.stat-boosted`, `.stat-reduced` — nature highlight colors

Follow existing CSS conventions (use `@layer components` if that's the pattern).

- [ ] **Step 3: Commit**

```bash
git add static/css/
git commit -m "feat: add CSS styles for team member detail page"
```

---

### Task 8: Integration Testing & Polish

**Files:**
- All previously modified files

- [ ] **Step 1: Start dev server and test manually**

Run: `make dev`

Test the flow:
1. Go to `/team` — verify existing team page still works
2. Click "Configure" on a team member — verify detail page loads at `/team/members/{id}`
3. Select a nature — verify stat summary updates
4. Select an ability — verify it saves
5. Add a move — verify it appears in slots and becomes disabled in available list
6. Add 4 moves — verify "Add" buttons are disabled
7. Remove a move — verify slot clears and move becomes available again
8. Change level — verify available moves list refreshes

- [ ] **Step 2: Fix any issues found during manual testing**

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: All existing tests pass. No regressions.

- [ ] **Step 4: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: polish team member detail page"
```

---

### Task 9: Handler Tests

**Files:**
- Create: `internal/handler/team_test.go`

- [ ] **Step 1: Create mock `TeamStore`**

Create a `mockTeamStore` struct in `internal/handler/team_test.go` that implements the full `TeamStore` interface. Use the same pattern as `health_test.go` — struct fields for controlling return values.

- [ ] **Step 2: Write test for `HandleDetail` (GET)**

Test cases:
- Valid member ID → 200, renders page (check response body contains pokemon name)
- Invalid member ID (non-numeric) → 400
- Member not found → 404

- [ ] **Step 3: Write test for `HandleSetNature` (PATCH)**

Test cases:
- Valid nature_id → 200
- Empty nature_id (clear) → 200
- Invalid nature_id → 400

- [ ] **Step 4: Write test for `HandleAddMove` (POST)**

Test cases:
- Valid move_id with available slot → 200 (or 201)
- All 4 slots full → 409

- [ ] **Step 5: Write test for `HandleRemoveMove` (DELETE)**

Test cases:
- Valid tmMoveId → 200
- Invalid tmMoveId → 400

- [ ] **Step 6: Run tests**

Run: `go test ./internal/handler/ -v`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/handler/team_test.go
git commit -m "test: add handler tests for team member detail"
```
