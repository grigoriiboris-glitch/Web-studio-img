DROP INDEX IF EXISTS manual_edits_user_idempotency_uq;
ALTER TABLE manual_edits DROP COLUMN IF EXISTS idempotency_key;