CREATE TABLE IF NOT EXISTS card_type_definitions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  current_version INTEGER NOT NULL DEFAULT 1 CHECK (current_version > 0),
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT card_type_definitions_key_not_blank CHECK (btrim(key) <> ''),
  CONSTRAINT card_type_definitions_name_not_blank CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS card_type_definitions_project_key_uq
  ON card_type_definitions(project_id, key);
CREATE INDEX IF NOT EXISTS card_type_definitions_project_active_idx
  ON card_type_definitions(project_id, archived_at, name);

CREATE TABLE IF NOT EXISTS card_type_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  card_type_id UUID NOT NULL REFERENCES card_type_definitions(id) ON DELETE CASCADE,
  version INTEGER NOT NULL CHECK (version > 0),
  schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  production_defaults JSONB NOT NULL DEFAULT '{}'::jsonb,
  default_recipe_id TEXT,
  default_recipe_version INTEGER CHECK (default_recipe_version IS NULL OR default_recipe_version > 0),
  default_template_id TEXT,
  default_template_version INTEGER CHECK (default_template_version IS NULL OR default_template_version > 0),
  prompt_rules JSONB NOT NULL DEFAULT '{}'::jsonb,
  reject_reason_profile_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT card_type_versions_unique_version UNIQUE(card_type_id, version)
);

CREATE INDEX IF NOT EXISTS card_type_versions_type_idx
  ON card_type_versions(card_type_id, version DESC);

