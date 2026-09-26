-- name: CreateProject :one
INSERT INTO projects (user_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects
WHERE id = $1
LIMIT 1;

-- name: ListProjectsByUser :many
SELECT * FROM projects
WHERE user_id = $1
  AND status <> 'deleted'
ORDER BY updated_at DESC, id DESC;

-- name: UpdateProject :one
UPDATE projects
SET name = $2,
    description = $3,
    status = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveProject :one
UPDATE projects
SET status = 'archived',
    updated_at = now()
WHERE id = $1
RETURNING *;
