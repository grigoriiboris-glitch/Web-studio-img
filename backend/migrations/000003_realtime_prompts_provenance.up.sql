
ALTER TABLE generations
  ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
  ADD COLUMN IF NOT EXISTS error_code TEXT,
  ADD COLUMN IF NOT EXISTS error_message TEXT,
  ADD COLUMN IF NOT EXISTS cost NUMERIC(12,4),
  ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;

UPDATE generations g SET user_id = p.user_id FROM projects p WHERE g.project_id = p.id AND g.user_id IS NULL;
UPDATE generations SET status = 'succeeded' WHERE status = 'completed';
ALTER TABLE generations ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE generations ALTER COLUMN iteration_id DROP NOT NULL;
ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_status_check;
ALTER TABLE generations ADD CONSTRAINT generations_status_check CHECK (status IN ('queued','running','succeeded','failed','cancelled'));
ALTER TABLE generations ADD CONSTRAINT generations_cost_check_v2 CHECK (cost IS NULL OR cost >= 0);
CREATE UNIQUE INDEX IF NOT EXISTS generations_user_id_idempotency_uq ON generations(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

ALTER TABLE assets
  ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS preview_key TEXT,
  ADD COLUMN IF NOT EXISTS thumbnail_key TEXT,
  ADD COLUMN IF NOT EXISTS checksum CHAR(64),
  ADD COLUMN IF NOT EXISTS exif JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS lifecycle_status TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
UPDATE assets a SET user_id = p.user_id FROM projects p WHERE a.project_id = p.id AND a.user_id IS NULL;
UPDATE assets SET checksum = sha256 WHERE checksum IS NULL AND sha256 IS NOT NULL;
ALTER TABLE assets ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE assets ADD CONSTRAINT assets_lifecycle_check CHECK (
  lifecycle_status IN ('pending','active','orphaned','deleted')
  AND (lifecycle_status IN ('pending','orphaned') AND expires_at IS NOT NULL OR lifecycle_status IN ('active','deleted'))
);
CREATE INDEX assets_lifecycle_expiry_idx ON assets(lifecycle_status, expires_at) WHERE expires_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS assets_user_checksum_uq ON assets(user_id, checksum) WHERE checksum IS NOT NULL;

CREATE TABLE prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  iteration_id UUID REFERENCES iterations(id) ON DELETE SET NULL,
  parent_prompt_id UUID REFERENCES prompts(id) ON DELETE RESTRICT,
  version INTEGER NOT NULL,
  original_text TEXT NOT NULL,
  ai_suggestions JSONB NOT NULL DEFAULT '[]'::jsonb,
  final_text TEXT,
  components JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by TEXT NOT NULL DEFAULT 'human',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT prompts_created_by_check CHECK (created_by IN ('human','ai','mixed')),
  CONSTRAINT prompts_version_positive CHECK (version > 0),
  CONSTRAINT prompts_original_not_blank CHECK (btrim(original_text) <> '')
);
CREATE UNIQUE INDEX prompts_project_version_uq ON prompts(project_id, version);
CREATE INDEX prompts_project_created_idx ON prompts(project_id, created_at DESC);

CREATE TABLE "references" (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  asset_id UUID REFERENCES assets(id) ON DELETE SET NULL,
  source_url TEXT,
  source_type TEXT NOT NULL DEFAULT 'unknown',
  license TEXT NOT NULL DEFAULT 'unknown',
  license_verified BOOLEAN NOT NULL DEFAULT FALSE,
  user_owned BOOLEAN NOT NULL DEFAULT FALSE,
  sha256 CHAR(64),
  notes TEXT,
  influence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT references_source_type_check CHECK (source_type IN ('inspiration','reference','direct_source','user_created','public_domain','unknown')),
  CONSTRAINT references_license_check CHECK (btrim(license) <> ''),
  CONSTRAINT references_sha256_check CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-fA-F]{64}$')
);
CREATE INDEX references_project_created_idx ON "references"(project_id, created_at DESC);
CREATE INDEX references_asset_idx ON "references"(asset_id);

CREATE TABLE human_actions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  iteration_id UUID REFERENCES iterations(id) ON DELETE SET NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  action_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  old_state JSONB,
  new_state JSONB,
  ai_influence JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT human_actions_type_check CHECK (action_type IN ('IDEA_CREATED','PROMPT_EDITED','PROMPT_APPROVED','REFERENCE_ADDED','REFERENCE_SELECTED','VARIANT_SELECTED','VARIANT_REJECTED','AI_RECOMMENDATION_REJECTED','COMPOSITION_CHANGED','MATERIAL_SELECTED','TEXTURE_SELECTED','MANUAL_EDIT','APPROVED','EXPORT_CREATED'))
);
CREATE INDEX human_actions_project_created_idx ON human_actions(project_id, created_at DESC);

CREATE TABLE provenance_events (
  sequence BIGSERIAL PRIMARY KEY,
  id UUID NOT NULL UNIQUE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  iteration_id UUID REFERENCES iterations(id) ON DELETE SET NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  action TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  parent_hash TEXT,
  hash CHAR(64) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX provenance_project_sequence_idx ON provenance_events(project_id, sequence);
CREATE INDEX provenance_user_sequence_idx ON provenance_events(user_id, sequence);

CREATE TABLE project_events (
  sequence BIGSERIAL PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id UUID NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX project_events_project_sequence_idx ON project_events(project_id, sequence);
