-- name: CreateUser :one
INSERT INTO users (email, name, avatar)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE lower(email) = lower($1)
LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET name = $2,
    avatar = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;
