-- name: ListTypes :many
SELECT id, name, slug FROM types ORDER BY id;

-- name: GetTypeByID :one
SELECT id, name, slug FROM types WHERE id = ?1;

-- name: ListTypesByEra :many
SELECT DISTINCT t.id, t.name, t.slug
FROM types t
JOIN type_efficacy te ON te.attacking_type_id = t.id
WHERE te.era = ?1
ORDER BY t.id;

-- name: GetTypeEfficacyByEra :many
SELECT attacking_type_id, defending_type_id, damage_factor
FROM type_efficacy
WHERE era = ?1
ORDER BY attacking_type_id, defending_type_id;

-- name: GetTypeEfficacyForAttackerByEra :many
SELECT defending_type_id, damage_factor
FROM type_efficacy
WHERE attacking_type_id = ?1 AND era = ?2;
