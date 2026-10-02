CREATE TABLE IF NOT EXISTS variant_mutation_idempotency (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  operation TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  entity_id UUID,
  response JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT variant_mutation_idempotency_key_check CHECK (btrim(idempotency_key) <> ''),
  CONSTRAINT variant_mutation_idempotency_operation_check CHECK (btrim(operation) <> ''),
  UNIQUE(user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS variant_mutation_idempotency_project_idx
  ON variant_mutation_idempotency(project_id, created_at DESC);
