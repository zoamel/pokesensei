-- name: GetVersionGroup :one
SELECT id, name, slug, generation, max_pokedex, type_chart_era, max_badges
FROM version_groups
WHERE id = ?1;

-- name: ListVersionGroups :many
SELECT id, name, slug, generation, max_pokedex, type_chart_era, max_badges
FROM version_groups
ORDER BY generation, id;
