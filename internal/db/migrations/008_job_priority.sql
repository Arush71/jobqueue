-- +goose Up
ALTER TABLE jobs
ADD COLUMN job_priority queue_priority NOT NULL DEFAULT 'normal';

-- +goose Down
ALTER TABLE jobs
DROP COLUMN IF EXISTS job_priority;

