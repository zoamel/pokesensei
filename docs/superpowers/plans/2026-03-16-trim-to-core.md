# Trim to Core Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip PokéSensei down to a focused team composition and battle helper, removing 10+ guide pages and replacing deleted content with Bulbapedia links.

**Architecture:** Work proceeds in dependency order — create new files before deleting old ones (the type chart template must exist before `guide.templ` is deleted). SQL cleanup must happen before the `pokemon.go` handler trim (so the interface deletion doesn't reference removed generated types). Each chunk ends with a passing build.

**Tech Stack:** Go 1.25, templ (HTML templates compiled to Go), HTMX, Alpine.js, SQLite/sqlc, goose. Code generation: `templ generate` (`.templ` → `_templ.go`), `sqlc generate` (`.sql` → `db/generated/*.go`). Run `make generate` to run both, or run them separately.

---

## File Map

| File | Action | What changes |
|---|---|---|
| `internal/view/helpers.go` | **Create** | `bulbapediaURL` helper function |
| `internal/view/typechart.templ` | **Create** | `TypeChartPage` template + 3 helpers migrated from `guide.templ` |
| `internal/handler/battle.go` | **Modify** | Add `ListTypes` to `BattleStore`, add `HandleTypeChart` method |
| `cmd/server/main.go` | **Modify** | Remove `guideHandler`, remove 14 guide routes, add `/battle/types` + redirect |
| `internal/view/layout.templ` | **Modify** | Remove Guide nav link |
| `internal/handler/guide.go` | **Delete** | Entire file |
| `internal/view/guide.templ` | **Delete** | Entire file |
| `internal/view/guide_pages.templ` | **Delete** | Entire file |
| `db/queries/abilities.sql` | **Modify** | Delete `ListAbilities` + `SearchAbilities`, keep `ListPokemonAbilities` |
| `db/queries/natures.sql` | **Delete** | Entire file (`ListNatures` was its only query) |
| `db/generated/` | **Regenerate** | Run `sqlc generate` after query deletions |
| `internal/view/types.go` | **Modify** | Remove 3 fields from `PokemonDetail`, delete 3 dead helper functions |
| `internal/handler/pokemon.go` | **Modify** | Remove 3 methods from `PokemonStore` interface + their call sites |
| `internal/view/pokemon.templ` | **Modify** | Remove evo/encounter/move sections; add Bulbapedia button + ability links |
| `internal/view/team.templ` | **Modify** | Add Bulbapedia icon to Pokémon names in team slots |
| `internal/view/battle.templ` | **Modify** | Add Type Chart card; add Bulbapedia icons to Pokémon names |
| `internal/view/dashboard.templ` | **Modify** | Remove "Getting Started" section; add coverage HTMX partial; add footer link |

---

## Chunk 1: Type Chart Migration + Bulbapedia Helper

Create the new files before any deletions. This is the foundation everything else depends on.

### Task 1: Create `bulbapediaURL` helper

**Files:**
- Create: `internal/view/helpers.go`

The `view` package already has `internal/view/types.go` for display helpers. Create a second helper file in the same package.

- [ ] **Step 1.1: Write the file**

```go
// internal/view/helpers.go
package view

import "strings"

// bulbapediaURL builds a Bulbapedia wiki URL for the given name and category.
// category is typically "Pokémon" or "Ability".
// Spaces are replaced with underscores; all other characters preserved as-is.
func bulbapediaURL(name, category string) string {
	slug := strings.ReplaceAll(name, " ", "_")
	return "https://bulbapedia.bulbagarden.net/wiki/" + slug + "_(" + category + ")"
}
```

- [ ] **Step 1.2: Build to verify no issues**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 1.3: Commit**

```bash
git add internal/view/helpers.go
git commit -m "feat: add bulbapediaURL helper to view package"
```

---

### Task 2: Create `typechart.templ` — migrate type chart out of `guide.templ`

**Files:**
- Create: `internal/view/typechart.templ`

`guide.templ` currently contains:
- `GuideTypesPage` template (lines 71–116) — needs to become `TypeChartPage` with `activePage = "battle"` instead of `"guide"` and without the "← Back to Guide" link
- `typeAbbrev`, `efficacyClass`, `efficacySymbol` functions (lines 310–341) — move verbatim
- `formatStatName` (lines 343–358) — do **not** migrate (only used by deleted natures page)

- [ ] **Step 2.1: Write the file**

```go
// internal/view/typechart.templ
package view

import (
	"fmt"
	"zoamel/pokesensei/db/generated"
)

templ TypeChartPage(types []generated.Type, matrix map[int64]map[int64]int64) {
	@AppShell("Type Chart", "battle") {
		<div class="guide">
			<h1>Type Chart</h1>
			<div class="guide-content">
				<p>Every Pokémon and move has a type. When attacking, your move's type vs the defender's type determines damage: super effective (2x), not very effective (0.5x), or immune (0x). Master a few key matchups — Water beats Fire, Fire beats Grass, Grass beats Water — and build from there.</p>
			</div>
			<p class="guide-intro">Click a row to highlight that attacking type's matchups.</p>
			<div class="type-chart-wrapper" x-data="{ activeRow: null }">
				<table class="type-chart">
					<thead>
						<tr>
							<th class="type-chart-corner">Atk ↓ / Def →</th>
							for _, t := range types {
								<th class="type-chart-header">
									<span class={ fmt.Sprintf("type-label type-%s", t.Slug) }>
										{ typeAbbrev(t.Name) }
									</span>
								</th>
							}
						</tr>
					</thead>
					<tbody>
						for _, atk := range types {
							<tr
								x-on:click={ fmt.Sprintf("activeRow = activeRow === %d ? null : %d", atk.ID, atk.ID) }
								:class={ fmt.Sprintf("activeRow === %d && 'type-chart-active'", atk.ID) }
								class="type-chart-row"
							>
								<td class="type-chart-label">
									<span class={ fmt.Sprintf("type-label type-%s", atk.Slug) }>{ atk.Name }</span>
								</td>
								for _, def := range types {
									<td class={ "type-chart-cell", efficacyClass(matrix[atk.ID][def.ID]) }>
										{ efficacySymbol(matrix[atk.ID][def.ID]) }
									</td>
								}
							</tr>
						}
					</tbody>
				</table>
			</div>
		</div>
	}
}

func typeAbbrev(name string) string {
	if len(name) <= 3 {
		return name
	}
	return name[:3]
}

func efficacyClass(factor int64) string {
	switch factor {
	case 200:
		return "eff-super"
	case 50:
		return "eff-not"
	case 0:
		return "eff-immune"
	default:
		return "eff-normal"
	}
}

func efficacySymbol(factor int64) string {
	switch factor {
	case 200:
		return "2×"
	case 50:
		return "½"
	case 0:
		return "0"
	default:
		return ""
	}
}
```

- [ ] **Step 2.2: Generate templ**

```bash
templ generate
```
Expected: generates `internal/view/typechart_templ.go`. No errors.

- [ ] **Step 2.3: Build**

```bash
go build ./...
```
Expected: no errors. (Both `GuideTypesPage` from `guide.templ` and `TypeChartPage` from `typechart.templ` coexist until guide files are deleted.)

- [ ] **Step 2.4: Commit**

```bash
git add internal/view/typechart.templ internal/view/typechart_templ.go
git commit -m "feat: migrate type chart template to typechart.templ"
```

---

### Task 3: Add `HandleTypeChart` to `BattleHandler`

**Files:**
- Modify: `internal/handler/battle.go`

`BattleStore` already has `GetTypeEfficacy`. Add `ListTypes` and the new handler method.

- [ ] **Step 3.1: Add `ListTypes` to `BattleStore` interface**

In `internal/handler/battle.go`, find the `BattleStore` interface (lines 15–27). Add `ListTypes` after `GetTypeEfficacy`:

```go
type BattleStore interface {
	GetGameState(ctx context.Context) (generated.GameState, error)
	ListTeamMembers(ctx context.Context, gameStateID int64) ([]generated.ListTeamMembersRow, error)
	ListTrainersByGame(ctx context.Context, gameVersionID int64) ([]generated.Trainer, error)
	GetTrainerByID(ctx context.Context, id int64) (generated.Trainer, error)
	ListTrainerPokemon(ctx context.Context, trainerID int64) ([]generated.ListTrainerPokemonRow, error)
	ListTrainerPokemonMoves(ctx context.Context, trainerPokemonID int64) ([]generated.ListTrainerPokemonMovesRow, error)
	GetPokemonByID(ctx context.Context, id int64) (generated.Pokemon, error)
	GetPokemonWithTypes(ctx context.Context, id int64) ([]generated.GetPokemonWithTypesRow, error)
	ListPokemonMovesAtLevel(ctx context.Context, arg generated.ListPokemonMovesAtLevelParams) ([]generated.ListPokemonMovesAtLevelRow, error)
	GetTypeEfficacy(ctx context.Context) ([]generated.TypeEfficacy, error)
	ListTypes(ctx context.Context) ([]generated.Type, error)
	SearchPokemonFiltered(ctx context.Context, arg generated.SearchPokemonFilteredParams) ([]generated.Pokemon, error)
}
```

- [ ] **Step 3.2: Add `HandleTypeChart` method**

At the end of `internal/handler/battle.go`, add:

```go
// GET /battle/types — interactive type chart
func (h *BattleHandler) HandleTypeChart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	types, err := h.store.ListTypes(ctx)
	if err != nil {
		h.log.Error("failed to list types", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	efficacy, err := h.store.GetTypeEfficacy(ctx)
	if err != nil {
		h.log.Error("failed to get efficacy", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	matrix := make(map[int64]map[int64]int64)
	for _, e := range efficacy {
		if matrix[e.AttackingTypeID] == nil {
			matrix[e.AttackingTypeID] = make(map[int64]int64)
		}
		matrix[e.AttackingTypeID][e.DefendingTypeID] = e.DamageFactor
	}

	if err := view.TypeChartPage(types, matrix).Render(ctx, w); err != nil {
		h.log.Error("failed to render type chart", "error", err)
	}
}
```

- [ ] **Step 3.3: Build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 3.4: Commit**

```bash
git add internal/handler/battle.go
git commit -m "feat: add HandleTypeChart to BattleHandler"
```

---

## Chunk 2: Route Cleanup and Navigation

### Task 4: Update routes in `cmd/server/main.go`

**Files:**
- Modify: `cmd/server/main.go`

Two changes in one edit: (1) remove the `guideHandler` variable, and (2) replace the guide routes block. Both must happen in the same step — removing the variable but leaving the route registrations would leave an unused variable and break `go build`.

- [ ] **Step 4.1: Remove `guideHandler` instantiation AND all guide route registrations in one edit**

**Delete** line 69 (the variable):
```go
guideHandler := handler.NewGuide(queries, log)
```

**Replace** the entire `// Basics Guide` block (lines 112–126) — all 14 route registrations — with the redirect + new route:

```go
// Type chart (moved from /guide/types; all other /guide/* routes removed)
srv.Handle("GET /guide/types", http.RedirectHandler("/battle/types", http.StatusMovedPermanently))
srv.Handle("GET /battle/types", http.HandlerFunc(battleHandler.HandleTypeChart))
```

The 12 other guide routes (`/guide`, `/guide/natures`, `/guide/abilities`, `/guide/abilities/search`, `/guide/evs-ivs`, `/guide/status`, `/guide/moves`, `/guide/basics`, `/guide/catching`, `/guide/gym-tips`, `/guide/recommended`, `/guide/items`, `/guide/mechanics/{game}`) are simply deleted — they will return 404 from Go's ServeMux, which is the intended behavior.

- [ ] **Step 4.2: Build**

```bash
go build ./...
```
Expected: no errors. `guide.go` still exists as a compilable file in the handler package; nothing in `main.go` references `GuideHandler` anymore, so the build passes.

- [ ] **Step 4.3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: replace guide routes with /battle/types redirect"
```

---

### Task 5: Remove Guide nav link from `layout.templ`

**Files:**
- Modify: `internal/view/layout.templ`

Delete line 30 — the Guide nav link:
```go
<a href="/guide" class={ "app-nav-link", templ.KV("active", activePage == "guide") }>Guide</a>
```

- [ ] **Step 5.1: Delete the Guide nav link from `AppShell`**

After the edit, the `app-nav-links` div in `AppShell` should contain only:
```html
<a href="/dashboard" ...>Dashboard</a>
<a href="/pokemon" ...>Pokémon</a>
<a href="/team" ...>Team</a>
<a href="/battle" ...>Battle</a>
<a href="/settings" ...>Settings</a>
```

- [ ] **Step 5.2: Generate templ and build**

```bash
templ generate && go build ./...
```
Expected: no errors.

- [ ] **Step 5.3: Commit**

```bash
git add internal/view/layout.templ internal/view/layout_templ.go
git commit -m "feat: remove Guide nav item from app shell"
```

---

### Task 6: Delete guide handler and templates

**Files:**
- Delete: `internal/handler/guide.go`
- Delete: `internal/view/guide.templ`
- Delete: `internal/view/guide_pages.templ`
- Delete: `internal/view/guide_templ.go` (generated from guide.templ)
- Delete: `internal/view/guide_pages_templ.go` (generated from guide_pages.templ)

- [ ] **Step 6.1: Delete the files**

```bash
rm internal/handler/guide.go \
   internal/view/guide.templ \
   internal/view/guide_pages.templ \
   internal/view/guide_templ.go \
   internal/view/guide_pages_templ.go
```

- [ ] **Step 6.2: Build**

```bash
go build ./...
```
Expected: no errors. The `GuideHandler` type no longer exists, and `main.go` no longer references it.

- [ ] **Step 6.3: Run tests**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 6.4: Commit**

```bash
git add -u
git commit -m "feat: delete guide handler and template files"
```

---

## Chunk 3: SQL Cleanup

### Task 7: Remove guide-only SQL queries and regenerate

**Prerequisite:** Task 6 must be complete. `internal/handler/guide.go` must already be deleted — it was the only caller of `ListNatures`, `ListAbilities`, and `SearchAbilities`. Once guide.go is gone, no Go code references these generated functions, so removing them from the query files will not break the build.

**Files:**
- Modify: `db/queries/abilities.sql`
- Delete: `db/queries/natures.sql`

- [ ] **Step 7.1: Edit `db/queries/abilities.sql`**

Delete the `ListAbilities` and `SearchAbilities` queries, keeping only `ListPokemonAbilities`.

The file should contain only:

```sql
-- name: ListPokemonAbilities :many
SELECT pa.ability_id, a.name, a.slug, a.description, pa.is_hidden, pa.slot
FROM pokemon_abilities pa
JOIN abilities a ON a.id = pa.ability_id
WHERE pa.pokemon_id = ?1
ORDER BY pa.slot;
```

- [ ] **Step 7.2: Delete `db/queries/natures.sql`**

```bash
rm db/queries/natures.sql
```

- [ ] **Step 7.3: Regenerate sqlc**

```bash
sqlc generate
```
Expected: `db/generated/` is updated. The `ListAbilities`, `SearchAbilities`, and `ListNatures` functions are removed from the generated code. `ListPokemonAbilities` remains.

- [ ] **Step 7.4: Build**

```bash
go build ./...
```
Expected: no errors. Since `guide.go` is already deleted, nothing references the removed generated functions.

- [ ] **Step 7.5: Commit**

```bash
git add db/queries/abilities.sql db/generated/
git rm db/queries/natures.sql db/generated/natures.sql.go 2>/dev/null || true
git commit -m "feat: remove guide-only SQL queries (ListAbilities, SearchAbilities, ListNatures)"
```

---

## Chunk 4: Pokémon Detail Trim

### Task 8: Trim `PokemonDetail` struct and dead helpers in `types.go`

**Files:**
- Modify: `internal/view/types.go`

The `PokemonDetail` struct (lines 32–39) currently has 6 fields. Remove `Encounters`, `EvoChain`, and `Moves`. Also delete the three dead helper functions: `formatEncounterMethod` + `encounterMethods` map, `formatLearnMethod` + `learnMethods` map, and `learnMethodTip` + `learnMethodTips` map.

- [ ] **Step 8.1: Update `PokemonDetail` struct**

Replace the struct:
```go
// PokemonDetail holds all data for the Pokémon detail page.
type PokemonDetail struct {
	Pokemon   generated.Pokemon
	Types     []TypeInfo
	Abilities []generated.ListPokemonAbilitiesRow
}
```

- [ ] **Step 8.2: Delete dead helper functions**

Delete from `types.go`:
- The `encounterMethods` var and `formatEncounterMethod` function (lines 55–74)
- The `learnMethods` var and `formatLearnMethod` function (lines 76–88)
- The `learnMethodTips` var and `learnMethodTip` function (lines 90–102)

Keep: `titleCase` — it is still used by other helpers.

After edits, `types.go` should import only `"strings"` and `"zoamel/pokesensei/db/generated"`.

- [ ] **Step 8.3: Build** (will fail — template still references removed fields)

```bash
go build ./...
```
Expected: compile errors from `pokemon.templ` referencing `detail.EvoChain`, `detail.Encounters`, `detail.Moves`. This is expected — we fix the template in Task 10.

- [ ] **Step 8.4: Commit the struct/helper changes only (don't worry about build failure)**

```bash
git add internal/view/types.go
git commit -m "feat: trim PokemonDetail struct and delete dead encounter/move helpers"
```

---

### Task 9: Trim `PokemonHandler` store interface and call sites in `pokemon.go`

**Files:**
- Modify: `internal/handler/pokemon.go`

- [ ] **Step 9.1: Read `pokemon.go` to see the current interface and handler**

Check lines 14–30 for the `PokemonStore` interface and lines 110–155 for the `HandleDetail` method.

- [ ] **Step 9.2: Remove three methods from `PokemonStore` interface**

Delete from the interface:
- `ListEncountersByPokemon(...)`
- `ListPokemonMoves(...)`
- `GetEvolutionChainByPokemon(...)` — remove from `PokemonHandler` only; `SuggestionsHandler` keeps its own copy

- [ ] **Step 9.3: Remove call sites in `HandleDetail`**

In `HandleDetail`, delete:
- The `encounters` variable and its `ListEncountersByPokemon` call
- The `moves` variable and its `ListPokemonMoves` call
- The `evoChain` variable and its `GetEvolutionChainByPokemon` call

- [ ] **Step 9.4: Update the `PokemonDetail` struct construction**

In `HandleDetail`, when building `detail`, remove the `Encounters`, `EvoChain`, and `Moves` fields from the struct literal.

- [ ] **Step 9.5: Build** (will still fail on template — that's next)

```bash
go build ./...
```
Expected: errors only from `pokemon_templ.go` — the handler itself should compile cleanly.

- [ ] **Step 9.6: Commit**

```bash
git add internal/handler/pokemon.go
git commit -m "feat: remove encounter/move/evo queries from PokemonHandler"
```

---

### Task 10: Trim `pokemon.templ` — remove three sections, add Bulbapedia links

**Files:**
- Modify: `internal/view/pokemon.templ`

Three sections to remove from `PokemonDetailPage`:
1. Evolution chain: `if len(detail.EvoChain) > 1 { ... }` block (lines 172–192)
2. Encounter locations: `if len(detail.Encounters) > 0 { ... }` block (lines 193–236)
3. Move learnset: `if len(detail.Moves) > 0 { ... }` block (lines 237–~310)

Two things to add:
- "View on Bulbapedia" button (after the "Add to Team" button)
- Bulbapedia link icon on each ability name

- [ ] **Step 10.1: Remove the three detail sections**

Delete the evolution chain, encounter locations, and move learnset `<section>` blocks from `PokemonDetailPage`.

- [ ] **Step 10.2: Add "View on Bulbapedia" button**

After the existing "Add to Team" button in the `detail-header`, add:

```html
<a
	href={ templ.SafeURL(bulbapediaURL(detail.Pokemon.Name, "Pokémon")) }
	target="_blank"
	rel="noopener"
	class="btn btn-secondary"
>
	View on Bulbapedia ↗
</a>
```

- [ ] **Step 10.3: Add Bulbapedia icon to ability names**

In the abilities section, update each `<li>` to link the ability name:

```html
for _, a := range detail.Abilities {
	<li>
		<a
			href={ templ.SafeURL(bulbapediaURL(a.Name, "Ability")) }
			target="_blank"
			rel="noopener"
			class="ability-link"
		>
			<strong>{ a.Name }</strong> ↗
		</a>
		if a.IsHidden != 0 {
			<span class="badge-hidden">Hidden</span>
		}
		if a.Description != "" {
			<p>{ a.Description }</p>
		}
	</li>
}
```

- [ ] **Step 10.4: Generate templ and build**

```bash
templ generate && go build ./...
```
Expected: no errors. This resolves all the compile failures from Tasks 8 and 9.

- [ ] **Step 10.5: Run tests**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 10.6: Commit**

```bash
git add internal/view/pokemon.templ internal/view/pokemon_templ.go
git commit -m "feat: trim pokemon detail page — remove evo/encounter/moves, add Bulbapedia links"
```

---

## Chunk 5: Bulbapedia Links + Dashboard + Battle Updates

### Task 11: Add Bulbapedia icons to team slots in `team.templ`

**Files:**
- Modify: `internal/view/team.templ`

In `teamBuilderSlot`, the Pokémon name is currently a plain `<a>` link to `/pokemon/{id}`. Add a small Bulbapedia external link next to it.

- [ ] **Step 11.1: Update `teamBuilderSlot` — add Bulbapedia link beside Pokémon name**

Find the `builder-member-name` anchor (around line 56) and add an external link after it:

```html
<div class="builder-member-name-row">
	<a href={ templ.SafeURL(fmt.Sprintf("/pokemon/%d", slot.Member.PokemonID)) } class="builder-member-name">
		{ slot.Member.PokemonName }
	</a>
	<a
		href={ templ.SafeURL(bulbapediaURL(slot.Member.PokemonName, "Pokémon")) }
		target="_blank"
		rel="noopener"
		class="bulba-link"
		title="View on Bulbapedia"
	>↗</a>
</div>
```

- [ ] **Step 11.2: Generate templ and build**

```bash
templ generate && go build ./...
```
Expected: no errors.

- [ ] **Step 11.3: Commit**

```bash
git add internal/view/team.templ internal/view/team_templ.go
git commit -m "feat: add Bulbapedia links to team builder slots"
```

---

### Task 12: Update `battle.templ` — Type Chart card + Bulbapedia icons

**Files:**
- Modify: `internal/view/battle.templ`

Two changes:
1. Add a "Type Chart" card/link above the trainer list in the battle sidebar
2. Add Bulbapedia icon to Pokémon names in trainer roster and matchup displays

- [ ] **Step 12.1: Add Type Chart link to battle sidebar**

In `BattlePage`, before `<h2>Trainers</h2>` in the sidebar, add:

```html
<div class="battle-tools">
	<a href="/battle/types" class="btn btn-secondary">Type Chart ↗</a>
</div>
```

- [ ] **Step 12.2: Read the rest of `battle.templ` to locate Pokémon name display**

```bash
# Read internal/view/battle.templ past line 60 to find trainer roster and matchup templates
```

- [ ] **Step 12.3: Add Bulbapedia icons to Pokémon names in matchup partials**

Find the `BattleMatchupPartial` or trainer roster template that renders individual Pokémon names. After each Pokémon name, add:

```html
<a
	href={ templ.SafeURL(bulbapediaURL(pokemon.Name, "Pokémon")) }
	target="_blank"
	rel="noopener"
	class="bulba-link"
	title="View on Bulbapedia"
>↗</a>
```

- [ ] **Step 12.4: Generate templ and build**

```bash
templ generate && go build ./...
```
Expected: no errors.

- [ ] **Step 12.5: Commit**

```bash
git add internal/view/battle.templ internal/view/battle_templ.go
git commit -m "feat: add Type Chart link and Bulbapedia icons to battle page"
```

---

### Task 13: Update `dashboard.templ` — trim dead sections, add coverage and footer

**Files:**
- Modify: `internal/view/dashboard.templ`

Three changes:
1. Remove the `"Getting Started"` `<section>` (lines 20–42 in current file — the conditional block that shows when `gs.BadgeCount <= 2 || len(team) < 3`)
2. Remove the "Basics Guide" shortcut card from the "Tools" section (the `<a href="/guide"` card)
3. Add type coverage HTMX partial container
4. Add footer link to Bulbapedia

- [ ] **Step 13.1: Remove "Getting Started" section entirely**

Delete the entire `if gs.BadgeCount <= 2 || len(team) < 3 { ... }` block (lines 20–42).

- [ ] **Step 13.2: Remove Guide shortcut card from Tools section**

In the `"tool-shortcuts"` section, delete:
```html
<a href="/guide" class="shortcut-card">
    <h3>Basics Guide</h3>
    <p>Learn game mechanics</p>
</a>
```

- [ ] **Step 13.3: Add type coverage HTMX section**

After the `<section class="team-overview">` block (after the 6 team slots), add:

```html
<section class="coverage-overview">
	<h2>Type Coverage</h2>
	<div
		hx-get="/team/coverage"
		hx-trigger="load"
		hx-swap="innerHTML"
	>
		<p class="loading-text">Loading coverage...</p>
	</div>
</section>
```

- [ ] **Step 13.4: Update badge count display and add footer**

The badge count `<div class="game-info">` is currently at the bottom. Keep it but also add a Bulbapedia reference footer after it:

```html
<footer class="dashboard-footer">
	<a href="https://bulbapedia.bulbagarden.net" target="_blank" rel="noopener" class="bulba-footer-link">
		Game mechanics reference → Bulbapedia ↗
	</a>
</footer>
```

- [ ] **Step 13.5: Generate templ and build**

```bash
templ generate && go build ./...
```
Expected: no errors.

- [ ] **Step 13.6: Run all tests**

```bash
go test ./...
```
Expected: all pass.

- [ ] **Step 13.7: Commit**

```bash
git add internal/view/dashboard.templ internal/view/dashboard_templ.go
git commit -m "feat: trim dashboard — remove guide links, add coverage summary and Bulbapedia footer"
```

---

## Final Verification

- [ ] **V1: Full build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **V2: All tests pass**

```bash
go test ./...
```
Expected: all pass.

- [ ] **V3: No guide routes except the redirect**

```bash
grep -r "guide" cmd/server/main.go
```
Expected: only the redirect line (`/guide/types` → `/battle/types`).

- [ ] **V4: No orphan guide links in non-deleted templates**

```bash
grep -r "/guide/" internal/view/
```
Expected: no matches (all guide links were in deleted files or `dashboard.templ` which we cleaned).

- [ ] **V5: sqlc generate still clean**

```bash
sqlc generate && go build ./...
```
Expected: no errors or changes.

- [ ] **V6: templ generate still clean**

```bash
templ generate && go build ./...
```
Expected: no errors or changes.
