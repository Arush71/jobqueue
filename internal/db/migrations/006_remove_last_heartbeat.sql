-- +goose Up
ALTER TABLE jobs
DROP COLUMN last_heartbeat_at;

-- +goose Down
ALTER TABLE jobs
ADD COLUMN last_heartbeat_at TIMESTAMP;
