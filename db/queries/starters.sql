-- name: ListStartersByGame :many
SELECT pokemon_id
FROM starter_groups
WHERE game_version_id = ?1
ORDER BY pokemon_id;

-- name: ListStartersWithTypes :many
SELECT
    p.id AS pokemon_id,
    p.name,
    p.sprite_url,
    t.name AS type_name,
    pt.slot AS type_slot
FROM starter_groups sg
JOIN pokemon p ON p.id = sg.pokemon_id
JOIN pokemon_types pt ON pt.pokemon_id = p.id
JOIN types t ON t.id = pt.type_id
WHERE sg.game_version_id = ?1
ORDER BY p.id, pt.slot;
