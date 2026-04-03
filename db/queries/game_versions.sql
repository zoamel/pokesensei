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

-- name: ListGameVersionsByVersionGroup :many
SELECT id, name, slug, version_group_id
FROM game_versions
WHERE version_group_id = ?1
ORDER BY id;
