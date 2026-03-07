-- name: ListTeamMembers :many
SELECT tm.id, tm.game_state_id, tm.pokemon_id, tm.level, tm.slot, tm.is_locked,
       p.name AS pokemon_name, p.slug AS pokemon_slug, p.sprite_url
FROM team_members tm
JOIN pokemon p ON p.id = tm.pokemon_id
WHERE tm.game_state_id = $1
ORDER BY tm.slot;

-- name: AddTeamMember :one
INSERT INTO team_members (game_state_id, pokemon_id, level, slot, is_locked)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, game_state_id, pokemon_id, level, slot, is_locked;

-- name: UpdateTeamMemberLevel :exec
UPDATE team_members SET level = $1 WHERE id = $2;

-- name: UpdateTeamMemberLock :exec
UPDATE team_members SET is_locked = $1 WHERE id = $2;

-- name: RemoveTeamMember :exec
DELETE FROM team_members WHERE id = $1;

-- name: ClearTeam :exec
DELETE FROM team_members WHERE game_state_id = $1;
