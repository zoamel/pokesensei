-- name: ListGameVersions :many
SELECT id, name, slug
FROM game_versions
ORDER BY id;

-- name: GetGameVersionBySlug :one
SELECT id, name, slug
FROM game_versions
WHERE slug = ?1;
