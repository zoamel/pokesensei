-- name: ListTypes :many
SELECT id, name, slug
FROM types
ORDER BY id;

-- name: GetTypeByID :one
SELECT id, name, slug
FROM types
WHERE id = ?1;

-- name: GetTypeEfficacy :many
SELECT attacking_type_id, defending_type_id, damage_factor
FROM type_efficacy
ORDER BY attacking_type_id, defending_type_id;

-- name: GetTypeEfficacyForAttacker :many
SELECT defending_type_id, damage_factor
FROM type_efficacy
WHERE attacking_type_id = ?1;
