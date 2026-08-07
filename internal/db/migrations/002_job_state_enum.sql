-- +goose Up
CREATE TYPE job_state AS ENUM ('queued', 'processing', 'fail', 'success');

-- +goose Down
DROP TYPE job_state;
