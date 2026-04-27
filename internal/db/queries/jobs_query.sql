-- name: CreateJob :one

INSERT INTO jobs(type, state,image_path,params)
VALUES ($1,$2,$3,$4)
RETURNING id;

-- name: GetJobById :one

SELECT *
FROM jobs WHERE id = $1;

-- name: UpdateJobId :exec

UPDATE jobs
SET state = $1, updated_at = NOW()
WHERE id = $2;

-- name: GetJobIfQueued :one

UPDATE jobs
SET state = 'processing',
updated_at = NOW()
WHERE id = $1 AND state = 'queued'
RETURNING *;

-- name: GetLeftJobs :many

SELECT id
FROM jobs
WHERE state = 'queued' 
ORDER BY created_at;

-- name: UpdateJobStateAtRestart :exec

UPDATE jobs
SET state = 'queued',
updated_at = NOW()
WHERE state = 'processing';
