-- name: GetGameState :one
SELECT gs.id, gs.game_version_id, gs.starter_pokemon_id, gs.badge_count,
       gs.trading_enabled, gs.created_at, gs.updated_at
FROM game_state gs
ORDER BY gs.id
LIMIT 1;

-- name: CreateGameState :one
INSERT INTO game_state (game_version_id, starter_pokemon_id, badge_count, trading_enabled)
VALUES (?1, ?2, ?3, ?4)
RETURNING id, game_version_id, starter_pokemon_id, badge_count, trading_enabled, created_at, updated_at;

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
