-- +goose Up
CREATE TYPE job_state AS ENUM (
  'queued',
  'processing',
  'fail',
  'retry',
  'success'
);

-- +goose Down
DROP TYPE job_type;
