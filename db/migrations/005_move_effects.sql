-- +goose Up
ALTER TABLE moves ADD COLUMN effect TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE moves DROP COLUMN effect;
