-- name: GetMoveByID :one
SELECT id, name, slug, type_id, power, accuracy, pp, damage_class
FROM moves
WHERE id = $1;

-- name: ListPokemonMoves :many
SELECT pm.move_id, m.name, m.slug, m.type_id, m.power, m.accuracy, m.pp,
       m.damage_class, pm.learn_method, pm.level_learned_at
FROM pokemon_moves pm
JOIN moves m ON m.id = pm.move_id
WHERE pm.pokemon_id = $1
  AND pm.version_group_id = $2
ORDER BY pm.learn_method, pm.level_learned_at, m.name;

-- name: ListPokemonMovesAtLevel :many
SELECT pm.move_id, m.name, m.slug, m.type_id, m.power, m.accuracy, m.pp,
       m.damage_class, pm.learn_method, pm.level_learned_at
FROM pokemon_moves pm
JOIN moves m ON m.id = pm.move_id
WHERE pm.pokemon_id = $1
  AND pm.version_group_id = $2
  AND pm.learn_method = 'level-up'
  AND pm.level_learned_at <= $3
ORDER BY pm.level_learned_at DESC, m.name;
