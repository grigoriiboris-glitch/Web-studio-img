-- name: CreateGeneration :one
INSERT INTO generations (
    project_id, iteration_id, provider, provider_job_id, model, model_version,
    prompt, negative_prompt, seed, aspect_ratio, parameters, status, estimated_cost
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetGeneration :one
SELECT * FROM generations
WHERE id = $1
LIMIT 1;

-- name: UpdateGenerationStatus :one
UPDATE generations
SET status = $2,
    error = $3,
    actual_cost = $4,
    completed_at = CASE
        WHEN $2 IN ('completed', 'failed', 'cancelled') THEN now()
        ELSE completed_at
    END
WHERE id = $1
RETURNING *;

-- name: ListProjectGenerations :many
SELECT * FROM generations
WHERE project_id = $1
ORDER BY created_at DESC, id DESC;
