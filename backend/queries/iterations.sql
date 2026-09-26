-- name: CreateIteration :one
INSERT INTO iterations (project_id, parent_iteration_id, type, title, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIteration :one
SELECT * FROM iterations
WHERE id = $1
LIMIT 1;

-- name: ListProjectIterations :many
SELECT * FROM iterations
WHERE project_id = $1
ORDER BY created_at ASC, id ASC;
