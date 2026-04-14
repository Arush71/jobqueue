-- +goose Up
CREATE TYPE job_type AS ENUM ('resize', 'compress', 'grayscale');

-- +goose Down
DROP TYPE job_type;
