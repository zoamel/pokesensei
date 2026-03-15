# PokéSensei — Trim to Core Design Spec

**Date:** 2026-03-15
**Status:** Approved

## Goal

Reduce PokéSensei from a broad Pokémon reference app to a focused **team composition and battle helper** for players new to Pokémon games. Content that belongs in a reference wiki is replaced with direct Bulbapedia links. Everything that remains serves one of three jobs: help the user build a well-composed team, understand their Pokémon's battle role, or prepare for upcoming battles.

---

## Navigation (After Trim)

Four top-level sections replace the current flat nav:

| Route | Label | Purpose |
|---|---|---|
| `/dashboard` | Dashboard | At-a-glance team state and quick links |
| `/pokemon` | Pokédex | Find and inspect Pokémon |
| `/team` | My Team | Build team, view coverage, get suggestions |
| `/battle` | Battle | Trainer matchups and type chart |
| `/settings` | Settings | Game/starter/badge/trading config |

The `/guide` section is removed entirely (see Deletions below). The type chart moves from `/guide/types` to `/battle/types`.

---

## Pages

### Dashboard (`/dashboard`)

**Keep:**
- 6-slot team display with sprites and levels
- Badge progress displayed prominently (e.g. `Badges: 3/8`)
- Three quick-link cards: Pokédex, My Team, Battle

**Add:**
- Compact read-only type coverage summary — shows which types the current team covers and where gaps exist, giving immediately actionable info without navigating to the team builder

**Remove:**
- "Getting Started" shortcut section (linked to deleted guide pages)
- "Basics Guide" shortcut card
- `/guide/gym-tips` shortcut card

**Add (footer):**
- Small "Game mechanics reference → Bulbapedia" external link as a replacement for deleted guide shortcuts

---

### Pokédex — List (`/pokemon`)

No changes. Filters (name search, type, badge level, game version) stay as-is.

---

### Pokédex — Detail (`/pokemon/{id}`)

**Keep:**
- Pokémon name, sprite, types (with type badges)
- Base stats (HP, Atk, Def, SpA, SpD, Spe)
- Abilities (name only, with Bulbapedia link per ability)

**Remove:**
- Encounter locations (where to find in-game)
- Full move learnset (level-up, TM/HM, tutor, egg moves)
- Evolution chain display

**Add:**
- Prominent "View on Bulbapedia" button linking to `https://bulbapedia.bulbagarden.net/wiki/{Name}_(Pokémon)`
- Bulbapedia link icon on each ability name linking to `https://bulbapedia.bulbagarden.net/wiki/{AbilityName}_(Ability)`
- Small external link icon on Pokémon name in team slots and battle matchup lists throughout the app

---

### My Team (`/team`)

No changes to functionality. Keeps:
- 6-slot team builder (add/remove/update level/lock)
- Type coverage matrix (18×18)
- Team suggestions (current + planned)

Bulbapedia link icon added to each Pokémon name in team slots.

---

### Battle (`/battle`)

No changes to functionality. Keeps:
- Trainer list filtered by current game and badge count
- Trainer detail: roster with matchup rankings against your team
- Pokémon search for wild Pokémon matchups

**Add:**
- "Type Chart" link in the Battle section nav/header pointing to `/battle/types`
- Bulbapedia link icon on Pokémon names in trainer rosters and matchup lists

---

### Type Chart (`/battle/types`)

Moved from `/guide/types`. No functional changes — interactive 18×18 efficacy matrix stays as-is. Route changes from `/guide/types` to `/battle/types`.

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

All Bulbapedia links open in a new tab (`target="_blank" rel="noopener"`).

---

## Deletions

### Handler files
- `internal/handler/guide.go` — entire file deleted

### View/template files
- `internal/view/guide.templ` — entire file deleted
- `internal/view/guide_pages.templ` — entire file deleted

### Routes (removed from server routing)
- `GET /guide` (index)
- `GET /guide/types` → replaced by `GET /battle/types`
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

### SQL queries (no longer called, removed from query files)
- `ListAbilities` — no longer displayed in guide
- `SearchAbilities` — no longer needed (guide/abilities/search removed)
- `ListNatures` — no longer displayed in guide

### Pokémon detail handler/view (trimmed, not fully deleted)
- Remove: encounter location queries and display (`ListEncountersByPokemon`)
- Remove: move learnset queries and display (`ListPokemonMoves`, `ListPokemonMovesAtLevel`)
- Remove: evolution chain queries and display (`GetEvolutionChainByPokemon`)

### Database schema
No migration needed. Tables for encounters, moves, evolution chains, abilities, and natures remain in the database — they are still used by the suggestion engine and battle helper internally. Only the display layer is removed.

---

## What Does NOT Change

- Onboarding flow (3-step: game → starter → badges)
- Team builder functionality
- Team suggestion algorithm
- Battle matchup ranking algorithm
- Type coverage matrix
- Settings page
- Database schema and all underlying data
- HTMX/Alpine.js interactivity patterns
- CSS architecture (`@layer`/`@import`)

---

## Success Criteria

1. A user can complete the full flow (onboarding → build team → prepare for battle) without hitting a dead link or missing page
2. Every Pokémon detail page has a working Bulbapedia link
3. No `/guide/*` routes exist except `/battle/types` (moved)
4. The dashboard coverage summary correctly reflects the current team's type gaps
5. The type chart is accessible from the Battle section
