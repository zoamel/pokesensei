-- +goose Up

-- Pokémon type system (18 types)
CREATE TABLE types (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    slug       TEXT NOT NULL UNIQUE
);

-- 18×18 type efficacy matrix
CREATE TABLE type_efficacy (
    attacking_type_id INTEGER NOT NULL REFERENCES types(id),
    defending_type_id INTEGER NOT NULL REFERENCES types(id),
    damage_factor     INTEGER NOT NULL, -- 0, 50, 100, or 200
    PRIMARY KEY (attacking_type_id, defending_type_id)
);

-- Game versions
CREATE TABLE game_versions (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE
);

-- Core Pokémon data
CREATE TABLE pokemon (
    id           INTEGER PRIMARY KEY, -- national dex number
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    generation   INTEGER NOT NULL,
    sprite_url   TEXT NOT NULL DEFAULT '',
    base_hp      INTEGER NOT NULL DEFAULT 0,
    base_attack  INTEGER NOT NULL DEFAULT 0,
    base_defense INTEGER NOT NULL DEFAULT 0,
    base_sp_atk  INTEGER NOT NULL DEFAULT 0,
    base_sp_def  INTEGER NOT NULL DEFAULT 0,
    base_speed   INTEGER NOT NULL DEFAULT 0
);

-- Pokémon type assignments (slot 1 and optional slot 2)
CREATE TABLE pokemon_types (
    pokemon_id INTEGER NOT NULL REFERENCES pokemon(id),
    type_id    INTEGER NOT NULL REFERENCES types(id),
    slot       INTEGER NOT NULL CHECK (slot IN (1, 2)),
    PRIMARY KEY (pokemon_id, slot)
);

-- Abilities
CREATE TABLE abilities (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

-- Pokémon ability assignments
CREATE TABLE pokemon_abilities (
    pokemon_id INTEGER NOT NULL REFERENCES pokemon(id),
    ability_id INTEGER NOT NULL REFERENCES abilities(id),
    is_hidden  INTEGER NOT NULL DEFAULT 0,
    slot       INTEGER NOT NULL,
    PRIMARY KEY (pokemon_id, slot)
);

-- Natures (25 total)
CREATE TABLE natures (
    id             INTEGER PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    slug           TEXT NOT NULL UNIQUE,
    increased_stat TEXT, -- NULL for neutral natures
    decreased_stat TEXT  -- NULL for neutral natures
);

-- Moves
CREATE TABLE moves (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    type_id      INTEGER REFERENCES types(id),
    power        INTEGER, -- NULL for status moves
    accuracy     INTEGER, -- NULL for moves that never miss
    pp           INTEGER NOT NULL DEFAULT 0,
    damage_class TEXT NOT NULL CHECK (damage_class IN ('physical', 'special', 'status'))
);

-- Pokémon move learnsets (per version group + learn method)
CREATE TABLE pokemon_moves (
    pokemon_id       INTEGER NOT NULL REFERENCES pokemon(id),
    move_id          INTEGER NOT NULL REFERENCES moves(id),
    version_group_id INTEGER NOT NULL,
    learn_method     TEXT NOT NULL, -- level-up, machine, tutor, egg
    level_learned_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (pokemon_id, move_id, version_group_id, learn_method, level_learned_at)
);

CREATE INDEX idx_pokemon_moves_pokemon ON pokemon_moves(pokemon_id);
CREATE INDEX idx_pokemon_moves_version ON pokemon_moves(pokemon_id, version_group_id);

-- Evolution chains
CREATE TABLE evolution_chains (
    id INTEGER PRIMARY KEY
);

-- Evolution steps within a chain
CREATE TABLE evolution_steps (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    chain_id           INTEGER NOT NULL REFERENCES evolution_chains(id),
    pokemon_id         INTEGER NOT NULL REFERENCES pokemon(id),
    evolves_from_id    INTEGER REFERENCES pokemon(id),
    evolution_trigger  TEXT, -- level-up, trade, use-item, etc.
    min_level          INTEGER,
    trigger_item       TEXT,
    trade_required     INTEGER NOT NULL DEFAULT 0,
    position           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_evolution_steps_chain ON evolution_steps(chain_id);
CREATE INDEX idx_evolution_steps_pokemon ON evolution_steps(pokemon_id);

-- Locations per game version
CREATE TABLE locations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pokeapi_id      INTEGER NOT NULL,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    game_version_id INTEGER NOT NULL REFERENCES game_versions(id),
    area_name       TEXT NOT NULL DEFAULT '',
    UNIQUE (pokeapi_id, game_version_id, area_name)
);

-- Pokémon encounters at locations
CREATE TABLE encounters (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    pokemon_id      INTEGER NOT NULL REFERENCES pokemon(id),
    location_id     INTEGER NOT NULL REFERENCES locations(id),
    game_version_id INTEGER NOT NULL REFERENCES game_versions(id),
    method          TEXT NOT NULL, -- walk, surf, fish, gift, static, etc.
    chance          INTEGER NOT NULL DEFAULT 0,
    min_level       INTEGER NOT NULL DEFAULT 0,
    max_level       INTEGER NOT NULL DEFAULT 0,
    badge_required  INTEGER NOT NULL DEFAULT 0 CHECK (badge_required BETWEEN 0 AND 8)
);

CREATE INDEX idx_encounters_pokemon ON encounters(pokemon_id);
CREATE INDEX idx_encounters_location ON encounters(location_id);
CREATE INDEX idx_encounters_game ON encounters(game_version_id);

-- +goose Down
DROP TABLE IF EXISTS encounters;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS evolution_steps;
DROP TABLE IF EXISTS evolution_chains;
DROP TABLE IF EXISTS pokemon_moves;
DROP TABLE IF EXISTS moves;
DROP TABLE IF EXISTS natures;
DROP TABLE IF EXISTS pokemon_abilities;
DROP TABLE IF EXISTS abilities;
DROP TABLE IF EXISTS pokemon_types;
DROP TABLE IF EXISTS pokemon;
DROP TABLE IF EXISTS game_versions;
DROP TABLE IF EXISTS type_efficacy;
DROP TABLE IF EXISTS types;
