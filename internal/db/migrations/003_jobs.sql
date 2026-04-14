-- +goose Up
CREATE TABLE jobs (
  id BIGSERIAL PRIMARY KEY,
  type job_type NOT NULL,
  state job_state NOT NULL DEFAULT 'queued',
  image_path TEXT NOT NULL,
  params JSONB NOT NULL
);

-- +goose Down
DROP TABLE jobs;
