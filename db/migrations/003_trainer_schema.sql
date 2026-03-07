-- +goose Up

-- Named trainers (gym leaders, elite four, champion, rival)
CREATE TABLE trainers (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    trainer_class   TEXT NOT NULL CHECK (trainer_class IN ('gym_leader', 'elite_four', 'champion', 'rival')),
    game_version_id INT NOT NULL REFERENCES game_versions(id),
    badge_number    SMALLINT NOT NULL DEFAULT 0, -- 0 for pre-badge encounters, 1-8 for gym leaders, 9 for E4/champion
    specialty_type  TEXT, -- e.g. "rock", "water" for gym leaders
    sprite_url      TEXT NOT NULL DEFAULT '',
    encounter_name  TEXT NOT NULL DEFAULT '' -- e.g. "Route 22 (early)", "Indigo Plateau" for rival/champion
);

CREATE INDEX idx_trainers_game ON trainers(game_version_id);
CREATE INDEX idx_trainers_badge ON trainers(badge_number);

-- Pokémon on a trainer's team
CREATE TABLE trainer_pokemon (
    id          SERIAL PRIMARY KEY,
    trainer_id  INT NOT NULL REFERENCES trainers(id) ON DELETE CASCADE,
    pokemon_id  INT NOT NULL REFERENCES pokemon(id),
    level       SMALLINT NOT NULL,
    position    SMALLINT NOT NULL CHECK (position BETWEEN 1 AND 6)
);

CREATE INDEX idx_trainer_pokemon_trainer ON trainer_pokemon(trainer_id);

-- Moves known by each trainer's Pokémon
CREATE TABLE trainer_pokemon_moves (
    id                  SERIAL PRIMARY KEY,
    trainer_pokemon_id  INT NOT NULL REFERENCES trainer_pokemon(id) ON DELETE CASCADE,
    move_id             INT NOT NULL REFERENCES moves(id),
    slot                SMALLINT NOT NULL CHECK (slot BETWEEN 1 AND 4)
);

CREATE INDEX idx_trainer_pokemon_moves_tp ON trainer_pokemon_moves(trainer_pokemon_id);

-- +goose Down
DROP TABLE IF EXISTS trainer_pokemon_moves;
DROP TABLE IF EXISTS trainer_pokemon;
DROP TABLE IF EXISTS trainers;
