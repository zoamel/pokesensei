-- +goose Up
-- +goose StatementBegin

-- Ensure Fairy type exists (id=18 from PokeAPI)
INSERT OR IGNORE INTO types (id, name, slug) VALUES (18, 'Fairy', 'fairy');

-- Recreate type_efficacy with era column in PK.
-- NOTE: The existing data already contains Fairy (18) rows and Gen VI Steel changes,
-- so the existing data represents the post_fairy era (18x18 = 324 rows).
CREATE TABLE type_efficacy_new (
    attacking_type_id INTEGER NOT NULL REFERENCES types(id),
    defending_type_id INTEGER NOT NULL REFERENCES types(id),
    damage_factor     INTEGER NOT NULL,
    era               TEXT NOT NULL DEFAULT 'pre_fairy',
    PRIMARY KEY (attacking_type_id, defending_type_id, era)
);

-- Copy existing data as post_fairy era (existing data is Gen VI / post_fairy)
INSERT INTO type_efficacy_new (attacking_type_id, defending_type_id, damage_factor, era)
SELECT attacking_type_id, defending_type_id, damage_factor, 'post_fairy'
FROM type_efficacy;

DROP TABLE type_efficacy;
ALTER TABLE type_efficacy_new RENAME TO type_efficacy;

-- Construct pre_fairy era: copy post_fairy excluding Fairy type rows
INSERT INTO type_efficacy (attacking_type_id, defending_type_id, damage_factor, era)
SELECT attacking_type_id, defending_type_id, damage_factor, 'pre_fairy'
FROM type_efficacy
WHERE era = 'post_fairy'
  AND attacking_type_id != 18
  AND defending_type_id != 18;

-- === Revert Gen VI Steel changes for pre_fairy era ===
-- Steel resisted Ghost (8) in Gen I-V: 100 → 50
UPDATE type_efficacy SET damage_factor = 50
WHERE attacking_type_id = 8 AND defending_type_id = 9 AND era = 'pre_fairy';

-- Steel resisted Dark (17) in Gen I-V: 100 → 50
UPDATE type_efficacy SET damage_factor = 50
WHERE attacking_type_id = 17 AND defending_type_id = 9 AND era = 'pre_fairy';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE type_efficacy_old (
    attacking_type_id INTEGER NOT NULL REFERENCES types(id),
    defending_type_id INTEGER NOT NULL REFERENCES types(id),
    damage_factor     INTEGER NOT NULL,
    PRIMARY KEY (attacking_type_id, defending_type_id)
);

-- Restore original data from post_fairy era (which matches the original imported data)
INSERT INTO type_efficacy_old (attacking_type_id, defending_type_id, damage_factor)
SELECT attacking_type_id, defending_type_id, damage_factor
FROM type_efficacy
WHERE era = 'post_fairy';

DROP TABLE type_efficacy;
ALTER TABLE type_efficacy_old RENAME TO type_efficacy;

DELETE FROM types WHERE id = 18;

-- +goose StatementEnd
