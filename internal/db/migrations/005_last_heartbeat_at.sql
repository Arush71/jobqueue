-- +goose Up
ALTER TABLE jobs
ADD COLUMN last_heartbeat_at TIMESTAMP;

-- +goose Down
ALTER TABLE jobs
DROP COLUMN last_heartbeat_at;
