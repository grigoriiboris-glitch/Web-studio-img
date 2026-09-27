DROP INDEX IF EXISTS composition_mutations_user_idempotency_uq;
ALTER TABLE composition_mutations DROP COLUMN IF EXISTS idempotency_key;
