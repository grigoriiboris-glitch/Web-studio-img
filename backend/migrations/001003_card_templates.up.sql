CREATE TABLE IF NOT EXISTS card_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  current_version INTEGER NOT NULL DEFAULT 1 CHECK (current_version > 0),
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT card_templates_project_key_uq UNIQUE (project_id, key)
);

CREATE TABLE IF NOT EXISTS card_template_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  card_template_id UUID NOT NULL REFERENCES card_templates(id) ON DELETE CASCADE,
  version INTEGER NOT NULL CHECK (version > 0),
  spec JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT card_template_versions_template_version_uq UNIQUE (card_template_id, version)
);

CREATE INDEX IF NOT EXISTS card_templates_project_idx ON card_templates(project_id);
CREATE INDEX IF NOT EXISTS card_templates_active_idx ON card_templates(project_id, archived_at);
CREATE INDEX IF NOT EXISTS card_template_versions_template_idx ON card_template_versions(card_template_id, version DESC);
