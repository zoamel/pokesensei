-- +goose Up

-- Single-user game state (MVP: no auth, one row)
CREATE TABLE game_state (
    id                 SERIAL PRIMARY KEY,
    game_version_id    INT REFERENCES game_versions(id),
    starter_pokemon_id INT REFERENCES pokemon(id),
    badge_count        SMALLINT NOT NULL DEFAULT 0 CHECK (badge_count BETWEEN 0 AND 8),
    trading_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Team members for the current game state
CREATE TABLE team_members (
    id            SERIAL PRIMARY KEY,
    game_state_id INT NOT NULL REFERENCES game_state(id) ON DELETE CASCADE,
    pokemon_id    INT NOT NULL REFERENCES pokemon(id),
    level         SMALLINT NOT NULL DEFAULT 5 CHECK (level BETWEEN 1 AND 100),
    slot          SMALLINT NOT NULL CHECK (slot BETWEEN 1 AND 6),
    is_locked     BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (game_state_id, slot)
);

CREATE INDEX idx_team_members_game_state ON team_members(game_state_id);

-- +goose Down
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS game_state;
