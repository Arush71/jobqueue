-- name: CreateJob :one

INSERT INTO jobs(job_type, state,payload)
VALUES ($1,$2,$3)
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

-- name: UpdateRetryCounterAndChangeState :exec

UPDATE jobs
SET state = 'queued',
updated_at = NOW(), retry_counter = retry_counter +1
WHERE id = $1;


-- name: RemoveJobsFromDBForTest :exec

TRUNCATE TABLE jobs RESTART IDENTITY;


-- name: GetResultFromJob :one

SELECT state, results, error,completed_at FROM jobs
WHERE id = $1;

-- name: SuccessJobWithResult :exec

UPDATE jobs
SET state = 'success' , updated_at = NOW(), results = $2, completed_at = NOW()
WHERE id = $1;

-- name: FailJobWithError :exec

UPDATE jobs
SET state = 'fail' , updated_at = NOW(), error = $2, completed_at = NOW()
WHERE id = $1;
