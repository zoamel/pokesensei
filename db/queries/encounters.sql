-- name: ListEncountersByPokemon :many
SELECT e.id, e.pokemon_id, e.location_id, e.game_version_id,
       e.method, e.chance, e.min_level, e.max_level, e.badge_required,
       l.name AS location_name, l.area_name
FROM encounters e
JOIN locations l ON l.id = e.location_id
WHERE e.pokemon_id = $1
  AND e.game_version_id = $2
ORDER BY e.badge_required, l.name;

-- name: ListEncountersByLocation :many
SELECT e.id, e.pokemon_id, e.game_version_id,
       e.method, e.chance, e.min_level, e.max_level, e.badge_required,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url
FROM encounters e
JOIN pokemon p ON p.id = e.pokemon_id
WHERE e.location_id = $1
ORDER BY e.chance DESC;

-- name: GetMinBadgeByPokemon :many
SELECT pokemon_id, MIN(badge_required)::smallint AS min_badge
FROM encounters
WHERE game_version_id = $1
GROUP BY pokemon_id;

-- name: ListLocationsByGame :many
SELECT id, pokeapi_id, name, slug, game_version_id, area_name
FROM locations
WHERE game_version_id = $1
ORDER BY name, area_name;
