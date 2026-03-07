-- name: GetPokemonByID :one
SELECT p.id, p.name, p.slug, p.generation, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense,
       p.base_sp_atk, p.base_sp_def, p.base_speed
FROM pokemon p
WHERE p.id = $1;

-- name: ListPokemonByType :many
SELECT p.id, p.name, p.slug, p.generation, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense,
       p.base_sp_atk, p.base_sp_def, p.base_speed
FROM pokemon p
JOIN pokemon_types pt ON pt.pokemon_id = p.id
WHERE pt.type_id = $1
ORDER BY p.id;

-- name: ListPokemonTypes :many
SELECT pt.pokemon_id, pt.type_id, pt.slot, t.name AS type_name, t.slug AS type_slug
FROM pokemon_types pt
JOIN types t ON t.id = pt.type_id
WHERE pt.pokemon_id = $1
ORDER BY pt.slot;

-- name: ListAllPokemon :many
SELECT id, name, slug, generation, sprite_url,
       base_hp, base_attack, base_defense,
       base_sp_atk, base_sp_def, base_speed
FROM pokemon
ORDER BY id;

-- name: SearchPokemonByName :many
SELECT id, name, slug, generation, sprite_url,
       base_hp, base_attack, base_defense,
       base_sp_atk, base_sp_def, base_speed
FROM pokemon
WHERE name ILIKE '%' || $1 || '%'
ORDER BY id
LIMIT 50;

-- name: SearchPokemonFiltered :many
SELECT DISTINCT p.id, p.name, p.slug, p.generation, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense,
       p.base_sp_atk, p.base_sp_def, p.base_speed
FROM pokemon p
LEFT JOIN pokemon_types pt ON pt.pokemon_id = p.id
LEFT JOIN encounters e ON e.pokemon_id = p.id AND e.game_version_id = sqlc.narg('game_version_id')::int
WHERE (sqlc.narg('name')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('name') || '%')
  AND (sqlc.narg('type_id')::int IS NULL OR pt.type_id = sqlc.narg('type_id'))
  AND (sqlc.narg('max_badge')::smallint IS NULL OR e.badge_required <= sqlc.narg('max_badge'))
ORDER BY p.id
LIMIT 60;

-- name: GetPokemonWithTypes :many
SELECT p.id, p.name, p.slug, p.generation, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense,
       p.base_sp_atk, p.base_sp_def, p.base_speed,
       t.id AS type_id, t.name AS type_name, t.slug AS type_slug, pt.slot AS type_slot
FROM pokemon p
JOIN pokemon_types pt ON pt.pokemon_id = p.id
JOIN types t ON t.id = pt.type_id
WHERE p.id = $1
ORDER BY pt.slot;
