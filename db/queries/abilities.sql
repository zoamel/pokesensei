-- name: ListAbilities :many
SELECT id, name, slug, description
FROM abilities
ORDER BY name;

-- name: SearchAbilities :many
SELECT id, name, slug, description
FROM abilities
WHERE name LIKE '%' || CAST(?1 AS TEXT) || '%'
   OR description LIKE '%' || CAST(?1 AS TEXT) || '%'
ORDER BY name
LIMIT 50;

-- name: ListPokemonAbilities :many
SELECT pa.ability_id, a.name, a.slug, a.description, pa.is_hidden, pa.slot
FROM pokemon_abilities pa
JOIN abilities a ON a.id = pa.ability_id
WHERE pa.pokemon_id = ?1
ORDER BY pa.slot;
