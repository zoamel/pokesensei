-- name: ListNatures :many
SELECT id, name, slug, increased_stat, decreased_stat
FROM natures
ORDER BY name;
