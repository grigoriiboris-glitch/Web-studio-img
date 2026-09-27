ALTER TABLE composition_mutations ADD COLUMN IF NOT EXISTS idempotency_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS composition_mutations_user_idempotency_uq
  ON composition_mutations(user_id,idempotency_key)
  WHERE idempotency_key IS NOT NULL;
