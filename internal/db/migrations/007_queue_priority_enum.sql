-- +goose Up
CREATE TYPE queue_priority AS ENUM ('high', 'normal', 'low');

-- +goose Down
DROP TYPE queue_priority;

