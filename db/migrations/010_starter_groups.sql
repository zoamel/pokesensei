-- +goose Up
-- +goose StatementBegin
CREATE TABLE starter_groups (
    game_version_id INTEGER NOT NULL REFERENCES game_versions(id),
    pokemon_id      INTEGER NOT NULL REFERENCES pokemon(id),
    PRIMARY KEY (game_version_id, pokemon_id)
);
-- +goose StatementEnd

-- FireRed (10) & LeafGreen (11): Kanto starters — Bulbasaur, Charmander, Squirtle
INSERT INTO starter_groups (game_version_id, pokemon_id) VALUES
    (10, 1), (10, 4), (10, 7),
    (11, 1), (11, 4), (11, 7);

-- HeartGold (15) & SoulSilver (16): Johto starters — Chikorita, Cyndaquil, Totodile
INSERT INTO starter_groups (game_version_id, pokemon_id) VALUES
    (15, 152), (15, 155), (15, 158),
    (16, 152), (16, 155), (16, 158);

-- +goose Down
-- +goose StatementBegin
DROP TABLE starter_groups;
-- +goose StatementEnd
