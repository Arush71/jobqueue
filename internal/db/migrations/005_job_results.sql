-- +goose Up
ALTER TABLE jobs
ADD COLUMN results JSONB NULL;

-- +goose Down
ALTER TABLE jobs
DROP COLUMN IF EXISTS results;
