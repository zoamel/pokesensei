-- +goose Up

-- Pokémon type system (18 types)
CREATE TABLE types (
    id         INT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    slug       TEXT NOT NULL UNIQUE
);

-- 18×18 type efficacy matrix
CREATE TABLE type_efficacy (
    attacking_type_id INT NOT NULL REFERENCES types(id),
    defending_type_id INT NOT NULL REFERENCES types(id),
    damage_factor     SMALLINT NOT NULL, -- 0, 50, 100, or 200
    PRIMARY KEY (attacking_type_id, defending_type_id)
);

-- Game versions
CREATE TABLE game_versions (
    id   INT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE
);

-- Core Pokémon data
CREATE TABLE pokemon (
    id           INT PRIMARY KEY, -- national dex number
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    generation   SMALLINT NOT NULL,
    sprite_url   TEXT NOT NULL DEFAULT '',
    base_hp      SMALLINT NOT NULL DEFAULT 0,
    base_attack  SMALLINT NOT NULL DEFAULT 0,
    base_defense SMALLINT NOT NULL DEFAULT 0,
    base_sp_atk  SMALLINT NOT NULL DEFAULT 0,
    base_sp_def  SMALLINT NOT NULL DEFAULT 0,
    base_speed   SMALLINT NOT NULL DEFAULT 0
);

-- Pokémon type assignments (slot 1 and optional slot 2)
CREATE TABLE pokemon_types (
    pokemon_id INT NOT NULL REFERENCES pokemon(id),
    type_id    INT NOT NULL REFERENCES types(id),
    slot       SMALLINT NOT NULL CHECK (slot IN (1, 2)),
    PRIMARY KEY (pokemon_id, slot)
);

-- Abilities
CREATE TABLE abilities (
    id          INT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

-- Pokémon ability assignments
CREATE TABLE pokemon_abilities (
    pokemon_id INT NOT NULL REFERENCES pokemon(id),
    ability_id INT NOT NULL REFERENCES abilities(id),
    is_hidden  BOOLEAN NOT NULL DEFAULT FALSE,
    slot       SMALLINT NOT NULL,
    PRIMARY KEY (pokemon_id, slot)
);

-- Natures (25 total)
CREATE TABLE natures (
    id             INT PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    slug           TEXT NOT NULL UNIQUE,
    increased_stat TEXT, -- NULL for neutral natures
    decreased_stat TEXT  -- NULL for neutral natures
);

-- Moves
CREATE TABLE moves (
    id           INT PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    type_id      INT REFERENCES types(id),
    power        SMALLINT, -- NULL for status moves
    accuracy     SMALLINT, -- NULL for moves that never miss
    pp           SMALLINT NOT NULL DEFAULT 0,
    damage_class TEXT NOT NULL CHECK (damage_class IN ('physical', 'special', 'status'))
);

-- Pokémon move learnsets (per version group + learn method)
CREATE TABLE pokemon_moves (
    pokemon_id       INT NOT NULL REFERENCES pokemon(id),
    move_id          INT NOT NULL REFERENCES moves(id),
    version_group_id INT NOT NULL,
    learn_method     TEXT NOT NULL, -- level-up, machine, tutor, egg
    level_learned_at SMALLINT NOT NULL DEFAULT 0,
    PRIMARY KEY (pokemon_id, move_id, version_group_id, learn_method, level_learned_at)
);

CREATE INDEX idx_pokemon_moves_pokemon ON pokemon_moves(pokemon_id);
CREATE INDEX idx_pokemon_moves_version ON pokemon_moves(pokemon_id, version_group_id);

-- Evolution chains
CREATE TABLE evolution_chains (
    id INT PRIMARY KEY
);

-- Evolution steps within a chain
CREATE TABLE evolution_steps (
    id                 SERIAL PRIMARY KEY,
    chain_id           INT NOT NULL REFERENCES evolution_chains(id),
    pokemon_id         INT NOT NULL REFERENCES pokemon(id),
    evolves_from_id    INT REFERENCES pokemon(id),
    evolution_trigger  TEXT, -- level-up, trade, use-item, etc.
    min_level          SMALLINT,
    trigger_item       TEXT,
    trade_required     BOOLEAN NOT NULL DEFAULT FALSE,
    position           SMALLINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_evolution_steps_chain ON evolution_steps(chain_id);
CREATE INDEX idx_evolution_steps_pokemon ON evolution_steps(pokemon_id);

-- Locations per game version
CREATE TABLE locations (
    id              SERIAL PRIMARY KEY,
    pokeapi_id      INT NOT NULL,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    game_version_id INT NOT NULL REFERENCES game_versions(id),
    area_name       TEXT NOT NULL DEFAULT '',
    UNIQUE (pokeapi_id, game_version_id, area_name)
);

-- Pokémon encounters at locations
CREATE TABLE encounters (
    id              SERIAL PRIMARY KEY,
    pokemon_id      INT NOT NULL REFERENCES pokemon(id),
    location_id     INT NOT NULL REFERENCES locations(id),
    game_version_id INT NOT NULL REFERENCES game_versions(id),
    method          TEXT NOT NULL, -- walk, surf, fish, gift, static, etc.
    chance          SMALLINT NOT NULL DEFAULT 0,
    min_level       SMALLINT NOT NULL DEFAULT 0,
    max_level       SMALLINT NOT NULL DEFAULT 0,
    badge_required  SMALLINT NOT NULL DEFAULT 0 CHECK (badge_required BETWEEN 0 AND 8)
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
