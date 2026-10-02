CREATE TABLE IF NOT EXISTS card_batches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  source_file TEXT NOT NULL DEFAULT '',
  sheet TEXT NOT NULL DEFAULT '',
  mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
  state JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT card_batches_name_not_blank CHECK (btrim(name) <> '')
);

CREATE INDEX IF NOT EXISTS card_batches_project_updated_idx
  ON card_batches(project_id, updated_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS card_batches_project_name_uq
  ON card_batches(project_id, name);