DROP INDEX IF EXISTS manual_edits_project_idempotency_uq;
CREATE UNIQUE INDEX manual_edits_user_idempotency_uq ON manual_edits(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
