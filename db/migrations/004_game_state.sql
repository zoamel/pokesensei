-- +goose Up

-- Single-user game state (MVP: no auth, one row)
CREATE TABLE game_state (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    game_version_id    INTEGER REFERENCES game_versions(id),
    starter_pokemon_id INTEGER REFERENCES pokemon(id),
    badge_count        INTEGER NOT NULL DEFAULT 0 CHECK (badge_count BETWEEN 0 AND 8),
    trading_enabled    INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Team members for the current game state
CREATE TABLE team_members (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    game_state_id INTEGER NOT NULL REFERENCES game_state(id) ON DELETE CASCADE,
    pokemon_id    INTEGER NOT NULL REFERENCES pokemon(id),
    level         INTEGER NOT NULL DEFAULT 5 CHECK (level BETWEEN 1 AND 100),
    slot          INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 6),
    is_locked     INTEGER NOT NULL DEFAULT 0,
    UNIQUE (game_state_id, slot)
);

CREATE INDEX idx_team_members_game_state ON team_members(game_state_id);

-- +goose Down
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS game_state;
