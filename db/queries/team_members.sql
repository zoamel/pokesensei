-- name: ListTeamMembers :many
SELECT tm.id, tm.game_state_id, tm.pokemon_id, tm.level, tm.slot, tm.is_locked,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url
FROM team_members tm
JOIN pokemon p ON p.id = tm.pokemon_id
WHERE tm.game_state_id = ?1
ORDER BY tm.slot;

-- name: AddTeamMember :one
INSERT INTO team_members (game_state_id, pokemon_id, level, slot, is_locked)
VALUES (?1, ?2, ?3, ?4, ?5)
RETURNING id, game_state_id, pokemon_id, level, slot, is_locked;

-- name: UpdateTeamMemberLevel :exec
UPDATE team_members SET level = ?1 WHERE id = ?2;

-- name: UpdateTeamMemberLock :exec
UPDATE team_members SET is_locked = ?1 WHERE id = ?2;

-- name: RemoveTeamMember :exec
DELETE FROM team_members WHERE id = ?1;

-- name: ClearTeam :exec
DELETE FROM team_members WHERE game_state_id = ?1;

-- name: GetTeamMemberDetail :one
SELECT tm.id, tm.pokemon_id, tm.level, tm.slot, tm.is_locked,
       tm.nature_id, tm.ability_id,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url,
       p.base_hp, p.base_attack, p.base_defense, p.base_sp_atk, p.base_sp_def, p.base_speed,
       n.name AS nature_name, n.increased_stat, n.decreased_stat,
       a.name AS ability_name
FROM team_members tm
JOIN pokemon p ON p.id = tm.pokemon_id
LEFT JOIN natures n ON n.id = tm.nature_id
LEFT JOIN abilities a ON a.id = tm.ability_id
WHERE tm.id = ?1;

-- name: SetTeamMemberNature :exec
UPDATE team_members SET nature_id = ?1 WHERE id = ?2;

-- name: SetTeamMemberAbility :exec
UPDATE team_members SET ability_id = ?1 WHERE id = ?2;

-- name: ListTeamMemberMoves :many
SELECT tmm.id, tmm.slot, tmm.move_id,
       m.name AS move_name, m.slug AS move_slug, m.power, m.accuracy, m.pp,
       m.damage_class, m.effect,
       t.name AS type_name, t.slug AS type_slug
FROM team_member_moves tmm
JOIN moves m ON m.id = tmm.move_id
LEFT JOIN types t ON t.id = m.type_id
WHERE tmm.team_member_id = ?1
ORDER BY tmm.slot;

-- name: AddTeamMemberMove :one
INSERT INTO team_member_moves (team_member_id, move_id, slot)
VALUES (?1, ?2, ?3)
RETURNING id, team_member_id, move_id, slot;

-- name: RemoveTeamMemberMove :exec
DELETE FROM team_member_moves WHERE id = ?1;

-- name: ListAvailableMoves :many
SELECT DISTINCT m.id, m.name, m.slug, m.power, m.accuracy, m.pp,
       m.damage_class, m.effect, pm.learn_method, pm.level_learned_at,
       t.name AS type_name, t.slug AS type_slug
FROM pokemon_moves pm
JOIN moves m ON m.id = pm.move_id
LEFT JOIN types t ON t.id = m.type_id
WHERE pm.pokemon_id = ?1
  AND pm.version_group_id = ?2
  AND (
    (pm.learn_method = 'level-up' AND pm.level_learned_at <= ?3)
    OR pm.learn_method != 'level-up'
  )
ORDER BY m.name;
