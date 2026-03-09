-- name: GetEvolutionChainByPokemon :many
SELECT es.id, es.chain_id, es.pokemon_id, es.evolves_from_id,
       es.evolution_trigger, es.min_level, es.trigger_item,
       es.trade_required, es.position,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url
FROM evolution_steps es
JOIN pokemon p ON p.id = es.pokemon_id
WHERE es.chain_id = (
    SELECT es2.chain_id FROM evolution_steps es2 WHERE es2.pokemon_id = ?1 LIMIT 1
)
ORDER BY es.position;
