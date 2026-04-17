-- +goose Up
-- +goose StatementBegin
INSERT INTO version_groups (id, name, slug, generation, max_pokedex, type_chart_era, max_badges)
VALUES (16, 'X / Y', 'xy', 6, 721, 'post_fairy', 8);

INSERT INTO game_versions (id, version_group_id, name, slug)
VALUES (23, 16, 'X', 'x'),
       (24, 16, 'Y', 'y');

-- Starters for X and Y: Kalos trio (Chespin #650, Fennekin #653, Froakie #656)
-- plus the Kanto gift starters from Professor Sycamore at Lumiose City
-- (Bulbasaur #1, Charmander #4, Squirtle #7). Both sets are obtainable early.
INSERT INTO starter_groups (game_version_id, pokemon_id) VALUES
    (23, 1), (23, 4), (23, 7), (23, 650), (23, 653), (23, 656),
    (24, 1), (24, 4), (24, 7), (24, 650), (24, 653), (24, 656);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM starter_groups WHERE game_version_id IN (23, 24);
DELETE FROM game_versions WHERE id IN (23, 24);
DELETE FROM version_groups WHERE id = 16;
-- +goose StatementEnd
