ALTER TABLE manual_edits ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX manual_edits_user_idempotency_uq ON manual_edits(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;