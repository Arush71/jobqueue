-- +goose Up
CREATE TABLE jobs (
  id BIGSERIAL PRIMARY KEY,
  type job_type NOT NULL,
  state job_state NOT NULL DEFAULT 'queued',
  image_path TEXT NOT NULL,
  params JSONB NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW (),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW (),
  retry_counter SMALLINT NOT NULL DEFAULT 0 CHECK (retry_counter >= 0)
);

-- +goose Down
DROP TABLE jobs;
