-- +goose Up

-- Bridge game_version_id → version_group_id for move lookups.
ALTER TABLE game_versions ADD COLUMN version_group_id INTEGER;
UPDATE game_versions SET version_group_id = 7 WHERE id IN (10, 11);
UPDATE game_versions SET version_group_id = 10 WHERE id IN (15, 16);

-- Extend team_members with nature and ability.
ALTER TABLE team_members ADD COLUMN nature_id INTEGER REFERENCES natures(id);
ALTER TABLE team_members ADD COLUMN ability_id INTEGER REFERENCES abilities(id);

-- Move slots (up to 4 per team member).
CREATE TABLE team_member_moves (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    team_member_id INTEGER NOT NULL REFERENCES team_members(id) ON DELETE CASCADE,
    move_id        INTEGER NOT NULL REFERENCES moves(id),
    slot           INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    UNIQUE (team_member_id, slot),
    UNIQUE (team_member_id, move_id)
);

-- +goose Down
DROP TABLE IF EXISTS team_member_moves;
-- SQLite does not support DROP COLUMN before 3.35; these columns are harmless if left.
