-- name: GetActiveGameState :one
SELECT gs.id, gs.game_version_id, gs.starter_pokemon_id, gs.badge_count,
       gs.trading_enabled, gs.is_active, gs.created_at, gs.updated_at
FROM game_state gs
WHERE gs.is_active = 1
LIMIT 1;

-- name: GetActiveGameContext :one
SELECT gs.id, gs.game_version_id, gs.badge_count, gs.trading_enabled,
       gv.version_group_id,
       vg.generation, vg.max_pokedex, vg.type_chart_era, vg.max_badges
FROM game_state gs
JOIN game_versions gv ON gv.id = gs.game_version_id
JOIN version_groups vg ON vg.id = gv.version_group_id
WHERE gs.is_active = 1
LIMIT 1;

-- name: GetGameStateForVersion :one
SELECT id, game_version_id, starter_pokemon_id, badge_count,
       trading_enabled, is_active, created_at, updated_at
FROM game_state
WHERE game_version_id = ?1;

-- name: ListGameStates :many
SELECT gs.id, gs.game_version_id, gs.starter_pokemon_id, gs.badge_count,
       gs.trading_enabled, gs.is_active, gs.created_at, gs.updated_at,
       gv.name AS game_name, gv.slug AS game_slug
FROM game_state gs
JOIN game_versions gv ON gv.id = gs.game_version_id
ORDER BY gs.updated_at DESC;

-- name: CreateGameState :one
INSERT INTO game_state (game_version_id, starter_pokemon_id, badge_count, trading_enabled, is_active)
VALUES (?1, ?2, ?3, ?4, 1)
RETURNING id, game_version_id, starter_pokemon_id, badge_count, trading_enabled, is_active, created_at, updated_at;

-- name: DeactivateAllGameStates :exec
UPDATE game_state SET is_active = 0 WHERE is_active = 1;

-- name: ActivateGameState :exec
UPDATE game_state SET is_active = 1, updated_at = datetime('now') WHERE id = ?1;

-- name: SwitchActiveGameState :exec
-- Atomically deactivate all game states and activate the target one.
UPDATE game_state
SET is_active = CASE WHEN id = sqlc.arg(target_id) THEN 1 ELSE 0 END,
    updated_at = CASE WHEN id = sqlc.arg(target_id) THEN datetime('now') ELSE updated_at END;

-- name: UpdateGameVersion :exec
UPDATE game_state SET game_version_id = ?1, updated_at = datetime('now') WHERE id = ?2;

-- name: UpdateStarter :exec
UPDATE game_state SET starter_pokemon_id = ?1, updated_at = datetime('now') WHERE id = ?2;

-- name: UpdateBadgeCount :exec
UPDATE game_state SET badge_count = ?1, updated_at = datetime('now') WHERE id = ?2;

-- name: UpdateTradingEnabled :exec
UPDATE game_state SET trading_enabled = ?1, updated_at = datetime('now') WHERE id = ?2;

-- name: DeleteGameState :exec
DELETE FROM game_state WHERE id = ?1;
