-- name: ListTrainersByGame :many
SELECT t.id, t.name, t.trainer_class, t.game_version_id, t.badge_number,
       t.specialty_type, t.sprite_url, t.encounter_name
FROM trainers t
WHERE t.game_version_id = ?1
ORDER BY t.badge_number, t.name;

-- name: GetTrainerByID :one
SELECT id, name, trainer_class, game_version_id, badge_number,
       specialty_type, sprite_url, encounter_name
FROM trainers
WHERE id = ?1;

-- name: ListTrainerPokemon :many
SELECT tp.id, tp.trainer_id, tp.pokemon_id, tp.level, tp.position,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url
FROM trainer_pokemon tp
JOIN pokemon p ON p.id = tp.pokemon_id
WHERE tp.trainer_id = ?1
ORDER BY tp.position;

-- name: ListTrainerPokemonMoves :many
SELECT tpm.id, tpm.trainer_pokemon_id, tpm.move_id, tpm.slot,
       m.name AS move_name, m.slug AS move_slug, m.type_id, m.power, m.accuracy, m.damage_class
FROM trainer_pokemon_moves tpm
JOIN moves m ON m.id = tpm.move_id
WHERE tpm.trainer_pokemon_id = ?1
ORDER BY tpm.slot;

-- name: ListTrainersByBadge :many
SELECT id, name, trainer_class, game_version_id, badge_number,
       specialty_type, sprite_url, encounter_name
FROM trainers
WHERE game_version_id = ?1
  AND badge_number = ?2
ORDER BY name;
