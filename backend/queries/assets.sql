-- name: CreateAsset :one
INSERT INTO assets (
    project_id, generation_id, type, storage_key, mime_type, size,
    width, height, sha256, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetAsset :one
SELECT * FROM assets
WHERE id = $1
LIMIT 1;

-- name: ListProjectAssets :many
SELECT * FROM assets
WHERE project_id = $1
ORDER BY created_at DESC, id DESC;

-- name: FindAssetBySHA256 :many
SELECT * FROM assets
WHERE sha256 = $1
ORDER BY created_at DESC;
