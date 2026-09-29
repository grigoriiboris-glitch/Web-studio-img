CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'develop',
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS purge_after TIMESTAMPTZ;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_mode_check;
ALTER TABLE projects ADD CONSTRAINT projects_mode_check CHECK (mode IN ('explore','develop','finalize'));

ALTER TABLE assets ADD COLUMN IF NOT EXISTS purge_after TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS quota_accounts (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  daily_limit INTEGER NOT NULL DEFAULT 100,
  monthly_limit INTEGER NOT NULL DEFAULT 1000,
  project_limit INTEGER NOT NULL DEFAULT 1000,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (daily_limit >= 0 AND monthly_limit >= 0 AND project_limit >= 0)
);

CREATE TABLE IF NOT EXISTS project_quota_accounts (
  project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_limit INTEGER NOT NULL DEFAULT 1000,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (project_limit >= 0),
  UNIQUE(project_id,user_id)
);

CREATE TABLE IF NOT EXISTS usage_ledger (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  generation_id UUID NOT NULL REFERENCES generations(id) ON DELETE RESTRICT,
  idempotency_key TEXT NOT NULL,
  units INTEGER NOT NULL CHECK (units > 0),
  cost NUMERIC(12,4) NOT NULL CHECK (cost >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS usage_ledger_user_created_idx ON usage_ledger(user_id,created_at DESC);
CREATE INDEX IF NOT EXISTS usage_ledger_project_created_idx ON usage_ledger(project_id,created_at DESC);

CREATE INDEX IF NOT EXISTS projects_name_trgm_idx ON projects USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS prompts_text_trgm_idx ON prompts USING gin ((COALESCE(original_text,'') || ' ' || COALESCE(final_text,'')) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS iterations_text_trgm_idx ON iterations USING gin ((COALESCE(title,'') || ' ' || COALESCE(description,'')) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS assets_metadata_trgm_idx ON assets USING gin ((COALESCE(type,'') || ' ' || COALESCE(metadata::text,'')) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS references_text_trgm_idx ON "references" USING gin ((COALESCE(source_url,'') || ' ' || COALESCE(notes,'') || ' ' || COALESCE(source_type,'')) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS provenance_payload_trgm_idx ON provenance_events USING gin ((COALESCE(action,'') || ' ' || COALESCE(payload::text,'')) gin_trgm_ops);
