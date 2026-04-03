-- +goose Up
-- +goose StatementBegin

-- New version_groups table (mirrors PokeAPI's version-group concept)
CREATE TABLE version_groups (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL,
    slug           TEXT NOT NULL UNIQUE,
    generation     INTEGER NOT NULL,
    max_pokedex    INTEGER NOT NULL,
    type_chart_era TEXT NOT NULL CHECK (type_chart_era IN ('pre_fairy', 'post_fairy')),
    max_badges     INTEGER NOT NULL DEFAULT 8
);

INSERT INTO version_groups (id, name, slug, generation, max_pokedex, type_chart_era, max_badges) VALUES
    (7,  'FireRed / LeafGreen',    'frlg', 3, 386, 'pre_fairy', 8),
    (10, 'HeartGold / SoulSilver', 'hgss', 4, 493, 'pre_fairy', 16);

-- Recreate game_state with is_active flag and relaxed badge check (0-16 for HGSS Kanto badges).
-- SQLite can't ALTER CHECK constraints, so we recreate the table.
PRAGMA foreign_keys = OFF;

CREATE TABLE game_state_new (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    game_version_id    INTEGER REFERENCES game_versions(id),
    starter_pokemon_id INTEGER REFERENCES pokemon(id),
    badge_count        INTEGER NOT NULL DEFAULT 0 CHECK (badge_count BETWEEN 0 AND 16),
    trading_enabled    INTEGER NOT NULL DEFAULT 0,
    is_active          INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Copy existing data, mark existing row as active
INSERT INTO game_state_new (id, game_version_id, starter_pokemon_id, badge_count, trading_enabled, is_active, created_at, updated_at)
SELECT id, game_version_id, starter_pokemon_id, badge_count, trading_enabled, 1, created_at, updated_at
FROM game_state;

-- Recreate team_members referencing new table (preserves ON DELETE CASCADE)
CREATE TABLE team_members_new (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    game_state_id INTEGER NOT NULL REFERENCES game_state_new(id) ON DELETE CASCADE,
    pokemon_id    INTEGER NOT NULL REFERENCES pokemon(id),
    level         INTEGER NOT NULL DEFAULT 5 CHECK (level BETWEEN 1 AND 100),
    slot          INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 6),
    is_locked     INTEGER NOT NULL DEFAULT 0,
    nature_id     INTEGER REFERENCES natures(id),
    ability_id    INTEGER REFERENCES abilities(id),
    UNIQUE (game_state_id, slot)
);

INSERT INTO team_members_new (id, game_state_id, pokemon_id, level, slot, is_locked, nature_id, ability_id)
SELECT id, game_state_id, pokemon_id, level, slot, is_locked, nature_id, ability_id
FROM team_members;

-- Recreate team_member_moves referencing new team_members
CREATE TABLE team_member_moves_new (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    team_member_id INTEGER NOT NULL REFERENCES team_members_new(id) ON DELETE CASCADE,
    move_id        INTEGER NOT NULL REFERENCES moves(id),
    slot           INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    UNIQUE (team_member_id, slot),
    UNIQUE (team_member_id, move_id)
);

INSERT INTO team_member_moves_new (id, team_member_id, move_id, slot)
SELECT id, team_member_id, move_id, slot
FROM team_member_moves;

-- Drop old tables and rename
DROP TABLE team_member_moves;
DROP TABLE team_members;
DROP TABLE game_state;

ALTER TABLE team_member_moves_new RENAME TO team_member_moves;
ALTER TABLE team_members_new RENAME TO team_members;
ALTER TABLE game_state_new RENAME TO game_state;

-- Recreate indexes
CREATE INDEX idx_team_members_game_state ON team_members(game_state_id);
CREATE UNIQUE INDEX idx_game_state_version ON game_state(game_version_id);

PRAGMA foreign_keys = ON;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_game_state_version;

CREATE TABLE game_state_old (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    game_version_id    INTEGER REFERENCES game_versions(id),
    starter_pokemon_id INTEGER REFERENCES pokemon(id),
    badge_count        INTEGER NOT NULL DEFAULT 0 CHECK (badge_count BETWEEN 0 AND 8),
    trading_enabled    INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO game_state_old (id, game_version_id, starter_pokemon_id, badge_count, trading_enabled, created_at, updated_at)
SELECT id, game_version_id, starter_pokemon_id, badge_count, trading_enabled, created_at, updated_at
FROM game_state;

CREATE TABLE team_members_old (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    game_state_id INTEGER NOT NULL REFERENCES game_state_old(id) ON DELETE CASCADE,
    pokemon_id    INTEGER NOT NULL REFERENCES pokemon(id),
    level         INTEGER NOT NULL DEFAULT 5 CHECK (level BETWEEN 1 AND 100),
    slot          INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 6),
    is_locked     INTEGER NOT NULL DEFAULT 0,
    nature_id     INTEGER REFERENCES natures(id),
    ability_id    INTEGER REFERENCES abilities(id),
    UNIQUE (game_state_id, slot)
);

INSERT INTO team_members_old (id, game_state_id, pokemon_id, level, slot, is_locked, nature_id, ability_id)
SELECT id, game_state_id, pokemon_id, level, slot, is_locked, nature_id, ability_id
FROM team_members;

CREATE TABLE team_member_moves_old (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    team_member_id INTEGER NOT NULL REFERENCES team_members_old(id) ON DELETE CASCADE,
    move_id        INTEGER NOT NULL REFERENCES moves(id),
    slot           INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    UNIQUE (team_member_id, slot),
    UNIQUE (team_member_id, move_id)
);

INSERT INTO team_member_moves_old (id, team_member_id, move_id, slot)
SELECT id, team_member_id, move_id, slot
FROM team_member_moves;

DROP TABLE team_member_moves;
DROP TABLE team_members;
DROP TABLE game_state;

ALTER TABLE team_member_moves_old RENAME TO team_member_moves;
ALTER TABLE team_members_old RENAME TO team_members;
ALTER TABLE game_state_old RENAME TO game_state;

CREATE INDEX idx_team_members_game_state ON team_members(game_state_id);

DROP TABLE IF EXISTS version_groups;

PRAGMA foreign_keys = ON;
-- +goose StatementEnd
