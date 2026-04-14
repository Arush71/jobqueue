-- name: CreateJob :one

INSERT INTO jobs(type, state,image_path,params)
VALUES ($1,$2,$3,$4)
RETURNING id;

