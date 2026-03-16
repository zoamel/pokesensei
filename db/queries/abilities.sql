-- name: ListPokemonAbilities :many
SELECT pa.ability_id, a.name, a.slug, a.description, pa.is_hidden, pa.slot
FROM pokemon_abilities pa
JOIN abilities a ON a.id = pa.ability_id
WHERE pa.pokemon_id = ?1
ORDER BY pa.slot;
