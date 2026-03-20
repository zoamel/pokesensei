# Team Member Detail Configuration

**Date:** 2026-03-20
**Status:** Draft

## Summary

Extend the team builder to allow configuring **moves (4 slots)**, **ability**, and **nature** per team member. Each team member gets a dedicated detail page at `/team/members/{id}` with forms for these three fields.

## Decisions

- **Approach A** — extend `team_members` with `nature_id` and `ability_id` columns; new `team_member_moves` junction table for move slots.
- **Moves start empty** — no auto-fill; user picks each move manually.
- **Move availability** — level-up moves filtered to current level or below + all TM/tutor/egg moves always shown. Filtered by `version_group_id` derived from the game state's game version.
- **Dedicated detail page** — separate route `/team/members/{id}`, not inline expansion or modal.
- **No changes to coverage matrix or suggestion engine** — both remain type-based.
- **Level-change move retention** — if the user lowers a Pokémon's level, already-assigned moves remain (they were legitimately learned). Only the available move pool for new additions changes.

## Database Schema

New migration `006_team_member_details.sql`:

```sql
-- Add version_group_id to game_versions for move learnset lookups.
-- FRLG versions (10, 11) → version_group 7; HGSS versions (15, 16) → version_group 10.
ALTER TABLE game_versions ADD COLUMN version_group_id INTEGER;
UPDATE game_versions SET version_group_id = 7 WHERE id IN (10, 11);
UPDATE game_versions SET version_group_id = 10 WHERE id IN (15, 16);

-- Add nature and ability to team_members.
-- ability_id references the abilities lookup table; validation that the ability
-- is legal for the Pokémon happens at the handler layer.
ALTER TABLE team_members ADD COLUMN nature_id INTEGER REFERENCES natures(id);
ALTER TABLE team_members ADD COLUMN ability_id INTEGER REFERENCES abilities(id);

-- Move slots (up to 4 per team member).
CREATE TABLE team_member_moves (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    team_member_id INTEGER NOT NULL REFERENCES team_members(id) ON DELETE CASCADE,
    move_id        INTEGER NOT NULL REFERENCES moves(id),
    slot           INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    UNIQUE (team_member_id, slot),
    UNIQUE (team_member_id, move_id)  -- prevent same move assigned twice
);
```

- `version_group_id` on `game_versions` bridges `game_state.game_version_id` → `pokemon_moves.version_group_id`.
- `nature_id` and `ability_id` are nullable — existing team members stay valid, users configure at their own pace.
- `ability_id` references `abilities(id)` (the lookup table), not `pokemon_abilities` (the junction table which has no single `id` column).
- `team_member_moves` enforces max 4 moves via slot constraint and prevents duplicate moves.
- `ON DELETE CASCADE` cleans up moves when a team member is removed.

## SQL Queries (sqlc)

### New queries

| Query | Purpose |
|---|---|
| `GetTeamMemberDetail` | Fetch member with nature name/stats and ability name (single row; moves queried separately) |
| `ListNatures` | All 25 natures with `increased_stat` and `decreased_stat` for the dropdown |
| `ListAvailableMoves` | Learnable moves for a Pokémon: level-up at/below given level + TM/tutor/egg, filtered by `version_group_id` |
| `ListTeamMemberMoves` | The assigned moves (up to 4) for a team member, joined with move details |
| `SetTeamMemberNature` | Update `nature_id` on `team_members` |
| `SetTeamMemberAbility` | Update `ability_id` on `team_members` |
| `AddTeamMemberMove` | Insert into `team_member_moves` with slot |
| `RemoveTeamMemberMove` | Delete from `team_member_moves` by `team_member_moves.id` |

### Key query details

**`GetTeamMemberDetail`** — left-joins nature and ability since both are nullable:
```sql
SELECT tm.id, tm.pokemon_id, tm.level, tm.slot, tm.is_locked,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense, p.base_sp_atk, p.base_sp_def, p.base_speed,
       n.name AS nature_name, n.increased_stat, n.decreased_stat,
       a.name AS ability_name
FROM team_members tm
JOIN pokemon p ON p.id = tm.pokemon_id
LEFT JOIN natures n ON n.id = tm.nature_id
LEFT JOIN abilities a ON a.id = tm.ability_id
WHERE tm.id = ?1;
```

**`ListAvailableMoves`** — union of level-up moves at/below level and all non-level-up methods, deduplicated by move:
```sql
SELECT DISTINCT m.id, m.name, m.type_name, m.power, m.accuracy, m.pp,
       m.damage_class, m.effect, pm.learn_method, pm.level_learned_at
FROM pokemon_moves pm
JOIN moves m ON m.id = pm.move_id
WHERE pm.pokemon_id = ?1
  AND pm.version_group_id = ?2
  AND (
    (pm.learn_method = 'level-up' AND pm.level_learned_at <= ?3)
    OR pm.learn_method != 'level-up'
  )
ORDER BY m.name;
```

Already-assigned moves are not excluded at the SQL level. The template marks them as disabled (non-addable) based on the `ListTeamMemberMoves` result, keeping the queries independent and simple.

### Existing queries reused

- `ListPokemonAbilities` — for the ability picker dropdown (returns abilities for a given `pokemon_id` including hidden flag).

### FK enforcement note

SQLite does not enforce FK constraints added via `ALTER TABLE ADD COLUMN`. The `nature_id` and `ability_id` FKs in the migration are declarative documentation. Actual validation (nature exists, ability is legal for the Pokémon) is enforced at the handler layer.

## API Endpoints

All new endpoints live on the existing `TeamHandler` struct, which will gain new methods and an expanded `TeamStore` interface.

| Method | Route | Purpose |
|---|---|---|
| `GET` | `/team/members/{id}` | Render detail page (full page) |
| `PATCH` | `/team/members/{id}/nature` | Set nature (HTMX partial swap) |
| `PATCH` | `/team/members/{id}/ability` | Set ability (HTMX partial swap) |
| `POST` | `/team/members/{id}/moves` | Add move to next available slot (HTMX partial swap) |
| `DELETE` | `/team/members/{id}/moves/{tmMoveId}` | Remove a move; `{tmMoveId}` is `team_member_moves.id` |

All mutation endpoints return HTMX partials for in-page updates without full reload.

**Level change interaction:** The existing `PATCH /team/members/{id}` endpoint already handles level updates and emits `HX-Trigger: team-updated`. On the detail page, the moves list section will listen for `team-updated` via `hx-trigger="team-updated from:body"` to refresh the available moves pool.

## Detail Page UI (`/team/members/{id}`)

### Header
- Pokémon sprite, name, type pills, level (editable — same PATCH as team page), Bulbapedia link
- "Back to Team" navigation link

### Nature Picker
- `<select>` dropdown with all 25 natures
- Options show stat effect: "Adamant (+Atk / -Sp.Atk)"
- Neutral natures (e.g., "Hardy") show no stat modifier
- Null option: "— No nature —"
- HTMX PATCH on change → swaps the stat summary to reflect the new nature

**Stat name mapping:** `natures.increased_stat` / `decreased_stat` text values map to `pokemon` base stat columns:
- `"attack"` → `base_attack`, `"defense"` → `base_defense`, `"special-attack"` → `base_sp_atk`, `"special-defense"` → `base_sp_def`, `"speed"` → `base_speed`

### Ability Picker
- `<select>` dropdown with Pokémon's available abilities (typically 2-3)
- Hidden ability labeled distinctly: "Speed Boost (Hidden)"
- Null option: "— No ability —"
- HTMX PATCH on change

### Moves Section
- 4 move slots displayed as a list
- Each filled slot: move name, type pill, power, accuracy, damage class (physical/special/status), "×" remove button
- Below: searchable move list with text filter input
  - Level-up moves filtered to current level or below
  - TM/tutor/egg moves always shown
  - Each row shows: move name, type, power, accuracy, damage class, learn method
  - "Add" button per move (disabled if 4 slots full or move already assigned)
  - Move list loaded as HTMX partial for performance
  - Refreshes when level changes (listens for `team-updated` trigger)

### Stat Summary (read-only)
- Base stats displayed for reference
- When nature is set, boosted stat highlighted (e.g., green/up arrow), reduced stat highlighted (e.g., red/down arrow)
- Helps user make informed nature choices

## Team Overview Page Changes

- Each filled team slot gets a "Configure" link (or clickable name/sprite) → `/team/members/{id}`
- Small status indicators on each slot: move count badge (e.g., "2/4 moves"), nature/ability set indicators
- No changes to type coverage matrix or suggestion engine

## Design Notes

- **Move slot compaction:** When a move is removed (e.g., slot 2), the remaining slots are left sparse (1, 3, 4). New moves fill the lowest available slot. The UI shows 4 fixed slot positions regardless.
- **Version support:** The `version_group_id` migration only covers FRLG and HGSS — the two games currently supported. Future game imports will populate this column via the import tool.

## Out of Scope

- Move-based type coverage analysis (natural follow-up enhancement)
- Held items
- EVs/IVs
- Changes to suggestion engine
