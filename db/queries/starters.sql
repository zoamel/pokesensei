-- name: ListStartersByGame :many
SELECT pokemon_id
FROM starter_groups
WHERE game_version_id = ?1
ORDER BY pokemon_id;
