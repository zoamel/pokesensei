-- +goose Up

UPDATE game_versions
SET version_group_id = 7
WHERE (id IN (10, 11) OR slug IN ('firered', 'leafgreen'))
  AND (version_group_id IS NULL OR version_group_id != 7);

UPDATE game_versions
SET version_group_id = 10
WHERE (id IN (15, 16) OR slug IN ('heartgold', 'soulsilver'))
  AND (version_group_id IS NULL OR version_group_id != 10);

-- +goose Down

UPDATE game_versions
SET version_group_id = NULL
WHERE id IN (10, 11, 15, 16)
   OR slug IN ('firered', 'leafgreen', 'heartgold', 'soulsilver');
