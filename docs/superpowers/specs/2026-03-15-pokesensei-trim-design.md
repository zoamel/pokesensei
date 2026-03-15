# PokéSensei — Trim to Core Design Spec

**Date:** 2026-03-15
**Status:** Approved

## Goal

Reduce PokéSensei from a broad Pokémon reference app to a focused **team composition and battle helper** for players new to Pokémon games. Content that belongs in a reference wiki is replaced with direct Bulbapedia links. Everything that remains serves one of three jobs: help the user build a well-composed team, understand their Pokémon's battle role, or prepare for upcoming battles.

---

## Navigation (After Trim)

Five top-level sections replace the current flat nav (Guide removed):

| Route | Label | Purpose |
|---|---|---|
| `/dashboard` | Dashboard | At-a-glance team state and quick links |
| `/pokemon` | Pokédex | Find and inspect Pokémon |
| `/team` | My Team | Build team, view coverage, get suggestions |
| `/battle` | Battle | Trainer matchups and type chart |
| `/settings` | Settings | Game/starter/badge/trading config |

The `/guide` section is removed entirely. The type chart moves from `/guide/types` to `/battle/types`. A permanent 301 redirect from `/guide/types` → `/battle/types` is added so existing bookmarks continue to work.

**Files affected:** `internal/server/server.go` (routes), `internal/view/layout.templ` (nav links).

---

## Pages

### Dashboard (`/dashboard`)

**Keep:**
- 6-slot team display with sprites and levels
- Badge progress displayed prominently (e.g. `Badges: 3/8`)
- Three quick-link cards: Pokédex, My Team, Battle

**Add:**
- Compact read-only type coverage summary, loaded via HTMX GET to `/team/coverage` on page load (`hx-get="/team/coverage" hx-trigger="load"`). The `/team/coverage` endpoint is fully self-contained — it reads game state and team from the database internally. No new data fetching code is needed in the dashboard handler. The summary renders the same `TeamCoveragePartial` already used by `/team`, showing covered vs. uncovered types.

**Remove:**
- The entire "Getting Started" `<section>` (contains the two `/guide/*` links in `dashboard.templ`)
- The "Basics Guide" and "Gym Strategy Tips" shortcut cards from the "Tools" section

**Add (footer):**
- Small "Game mechanics reference → Bulbapedia" external link (`https://bulbapedia.bulbagarden.net`)

---

### Pokédex — List (`/pokemon`)

No changes. Filters (name search, type, badge level, game version) stay as-is.

---

### Pokédex — Detail (`/pokemon/{id}`)

**Keep:**
- Pokémon name, sprite, types (with type badges)
- Base stats (HP, Atk, Def, SpA, SpD, Spe)
- Abilities (name only) — fetched via `ListPokemonAbilities` (per-Pokémon query; distinct from the guide-only `ListAbilities`)

**Remove from `PokemonHandler` store interface, handler call sites, and `PokemonDetailPage` template:**
- Encounter locations: remove the `<table class="encounter-table">` section from `internal/view/pokemon.templ`; remove `ListEncountersByPokemon` from `PokemonHandler`'s store interface (only called from `pokemon.go`)
- Move learnset: remove the move learnset section from `internal/view/pokemon.templ`; remove `ListPokemonMoves` from `PokemonHandler`'s store interface (only called from `pokemon.go`). **Do not touch** `ListPokemonMovesAtLevel` — separate query, used by `battle.go`
- Evolution chain: remove the `if len(detail.EvoChain) > 1 { ... }` block from `PokemonDetailPage` in `internal/view/pokemon.templ`; remove `GetEvolutionChainByPokemon` from `PokemonHandler`'s store interface and its call in `pokemon.go` only. **Do not delete this query** — `SuggestionsHandler` has its own separate store interface that retains it; `internal/handler/suggestions.go` is untouched.

**Add:**
- Prominent "View on Bulbapedia" button
- Bulbapedia link icon on each ability name

**Add throughout the app (team slots, battle matchup lists):**
- Small external link icon on each Pokémon name linking to its Bulbapedia page

---

### My Team (`/team`)

No changes to functionality or data fetching. Keeps:
- 6-slot team builder (add/remove/update level/lock)
- Type coverage matrix (18×18)
- Team suggestions (current + planned)

**Add:** Bulbapedia link icon on each Pokémon name in team slots.

---

### Battle (`/battle`)

No changes to functionality or data fetching. Keeps:
- Trainer list filtered by current game and badge count
- Trainer detail: roster with matchup rankings against your team
- Pokémon search for wild Pokémon matchups

**Add:**
- "Type Chart" link card on the `/battle` index page pointing to `/battle/types`
- Bulbapedia link icon on Pokémon names in trainer rosters and matchup lists

---

### Type Chart (`/battle/types`)

Moved from `/guide/types`. No functional changes.

**Handler change:** The type chart logic moves from `internal/handler/guide.go` into `internal/handler/battle.go`. The `BattleHandler` struct gains a `HandleTypeChart` method. The `GuideHandler`'s type chart queries (`ListTypes` + `GetTypeEfficacy`) transfer to `BattleHandler`:

- `BattleHandler` already has `GetTypeEfficacy` in its store interface — no change needed there
- `BattleHandler` does **not** currently have `ListTypes` — add it to `BattleStore` interface and wire it in `cmd/server/main.go`

**Route change in `internal/server/server.go`:**
```go
// Redirect old bookmark URL
mux.Handle("GET /guide/types", http.RedirectHandler("/battle/types", http.StatusMovedPermanently))
// New route
mux.HandleFunc("GET /battle/types", battleHandler.HandleTypeChart)
```
Note: registering `/guide/types` as an exact-path redirect does not create a prefix catch-all for other `/guide/*` paths in Go 1.22+ ServeMux. All other `/guide/*` paths remain unregistered and return 404.

---

## Bulbapedia Link Strategy

Links appear inline wherever content was removed or where external reference adds value:

| Location | Link target | Style |
|---|---|---|
| Pokémon detail page | `bulbapedia.bulbagarden.net/wiki/{Name}_(Pokémon)` | Prominent button |
| Ability names (detail page) | `bulbapedia.bulbagarden.net/wiki/{AbilityName}_(Ability)` | Small icon link |
| Pokémon names in team slots | Pokémon's Bulbapedia page | Small external icon |
| Pokémon names in battle matchups | Pokémon's Bulbapedia page | Small external icon |
| Dashboard footer | `bulbapedia.bulbagarden.net` main portal | Text link |

All Bulbapedia links: `target="_blank" rel="noopener"`.

**URL construction — `bulbapediaURL` helper:**

Add to `internal/view/helpers.go` (new file in the `view` package, alongside existing `types.go`). Being unexported is intentional — templ templates in the same package call it directly.

```go
package view

import "strings"

// bulbapediaURL builds a Bulbapedia wiki URL for the given name and category.
// category is "Pokémon" or "Ability".
// Spaces are replaced with underscores; all other characters preserved as-is.
func bulbapediaURL(name, category string) string {
    slug := strings.ReplaceAll(name, " ", "_")
    return "https://bulbapedia.bulbagarden.net/wiki/" + slug + "_(" + category + ")"
}
```

**Known edge cases (Gen I–IV scope):**
- Hyphenated names (`Ho-Oh`): preserved as-is — Bulbapedia uses hyphens
- Multi-word names (`Mr. Mime` → `Mr._Mime`): handled by space→underscore rule
- `Nidoran♀` / `Nidoran♂`: symbols passed through as-is; Bulbapedia handles them. Acceptable for MVP if a rare broken link occurs.
- Apostrophes (`Farfetch'd`): preserved as-is

---

## Deletions

### Handler files
- `internal/handler/guide.go` — entire file deleted; type chart handler migrates to `battle.go`

### View/template files
- `internal/view/guide.templ` — entire file deleted **after** migrating the type chart template (see below)
- `internal/view/guide_pages.templ` — entire file deleted

#### Type chart template migration (required before deleting `guide.templ`)

`guide.templ` contains both the `GuideTypesPage` template and three helper functions it depends on:
- `typeAbbrev(name string) string`
- `efficacyClass(factor int64) string`
- `efficacySymbol(factor int64) string`

(A fourth helper, `formatStatName`, is only used by `GuideNaturesPage` which is deleted — it does not need to migrate.)

**Create `internal/view/typechart.templ`** with:
1. The type chart template, renamed from `GuideTypesPage` to `TypeChartPage`, with the `AppShell` active page argument changed from `"guide"` to `"battle"`
2. The three helper functions moved verbatim from `guide.templ`

The `HandleTypeChart` method on `BattleHandler` calls `view.TypeChartPage(...)` (not the old `view.GuideTypesPage`).

### Navigation
- `internal/view/layout.templ` — remove Guide nav item
- `internal/view/dashboard.templ` — remove "Getting Started" `<section>` and guide shortcut cards (confirmed by grep: these are the only `/guide/*` links outside of the deleted files)

### Routes — `internal/server/server.go`

**Removed (unregistered → 404 via Go's net/http ServeMux):**
- `GET /guide` (index)
- `GET /guide/natures`
- `GET /guide/abilities`
- `GET /guide/abilities/search`
- `GET /guide/evs-ivs`
- `GET /guide/status`
- `GET /guide/moves`
- `GET /guide/mechanics/{game}`
- `GET /guide/basics`
- `GET /guide/catching`
- `GET /guide/gym-tips`
- `GET /guide/recommended`
- `GET /guide/items`

**Changed:**
- `GET /guide/types` → `http.RedirectHandler("/battle/types", http.StatusMovedPermanently)` via `mux.Handle`
- `GET /battle/types` → `BattleHandler.HandleTypeChart` via `mux.HandleFunc` (new)

### SQL query files — guide-only queries

Only called from `guide.go`; safe to remove entirely:

- `db/queries/abilities.sql`: delete `ListAbilities` and `SearchAbilities` queries. Keep `ListPokemonAbilities` (used by `pokemon.go`).
- `db/queries/natures.sql`: `ListNatures` is the only query in this file — **delete the entire file**.

After deletions, run `sqlc generate` to update `db/generated/`. The generated symbols for the deleted queries will be removed; verify no remaining code references them.

### Pokémon detail — `internal/handler/pokemon.go` and `internal/view/pokemon.templ`

Remove from `PokemonHandler` store interface and handler call sites:
- `ListEncountersByPokemon`
- `ListPokemonMoves`
- `GetEvolutionChainByPokemon` — remove from `PokemonHandler` interface only; `SuggestionsHandler` has its own separate store interface and retains it

Remove from `PokemonDetailPage` in `internal/view/pokemon.templ`:
- The `<table class="encounter-table">` section (encounter locations)
- The move learnset section
- The `if len(detail.EvoChain) > 1 { ... }` block (evolution chain)

Remove from the `PokemonDetail` display struct in `internal/view/types.go`:
- `EvoChain` field (and any associated display types no longer needed)
- `Encounters` field
- `Moves` / `MoveRows` fields

Remove dead helper functions from `internal/view/types.go` (only used by the deleted template sections):
- `formatEncounterMethod` (and its `encounterMethods` map) — only used by the encounter-table section
- `formatLearnMethod` (and its `learnMethods` map) — only used by the move learnset section
- `learnMethodTip` (and its `learnMethodTips` map) — only used by the move learnset section

### Database schema
No migration. All tables remain intact.

---

## What Does NOT Change

- Onboarding flow
- Team builder functionality and `TeamHandler`
- Team suggestion algorithm — `internal/handler/suggestions.go` untouched, retains `GetEvolutionChainByPokemon` in its store interface
- Battle matchup ranking — `battle.go` data fetching untouched except adding `ListTypes` to `BattleStore` and `HandleTypeChart` method
- `ListPokemonMovesAtLevel` query and its use in `battle.go`
- `ListPokemonAbilities` query and its use in `pokemon.go`
- Type coverage matrix on `/team`
- Settings page
- Database schema and all underlying data
- HTMX/Alpine.js interactivity patterns
- CSS architecture

---

## Success Criteria

1. Full flow works without dead links: onboarding → build team → prepare for battle
2. Every Pokémon detail page has a working "View on Bulbapedia" button
3. `/guide/types` returns 301 to `/battle/types`; all other `/guide/*` paths return 404
4. Dashboard coverage summary loads via HTMX from `/team/coverage` and renders type badges
5. Type chart accessible at `/battle/types`
6. `sqlc generate` runs cleanly after query deletions
7. `go build ./...` passes with no unused imports or interface mismatches
