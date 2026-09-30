CREATE TABLE IF NOT EXISTS recipes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  model_version TEXT,
  scope TEXT NOT NULL DEFAULT 'project',
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  preview_asset_id UUID REFERENCES assets(id) ON DELETE SET NULL,
  published BOOLEAN NOT NULL DEFAULT FALSE,
  current_version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT recipes_name_not_blank CHECK (btrim(name) <> ''),
  CONSTRAINT recipes_scope_check CHECK (scope IN ('project','global')),
  CONSTRAINT recipes_scope_project_check CHECK ((scope='global' AND project_id IS NULL) OR (scope='project' AND project_id IS NOT NULL)),
  UNIQUE(user_id, project_id, name)
);

CREATE INDEX IF NOT EXISTS recipes_user_updated_idx ON recipes(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS recipes_project_published_idx ON recipes(project_id, published, updated_at DESC);

CREATE TABLE IF NOT EXISTS recipe_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  workflow JSONB NOT NULL,
  input_mappings JSONB NOT NULL DEFAULT '{}'::jsonb,
  exposed_parameters JSONB NOT NULL DEFAULT '[]'::jsonb,
  default_parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
  workflow_hash CHAR(64) NOT NULL,
  created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(recipe_id, version)
);

CREATE INDEX IF NOT EXISTS recipe_versions_recipe_idx ON recipe_versions(recipe_id, version DESC);

ALTER TABLE generations
  ADD COLUMN IF NOT EXISTS recipe_id UUID REFERENCES recipes(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS recipe_version INTEGER,
  ADD COLUMN IF NOT EXISTS resolved_workflow_hash CHAR(64),
  ADD COLUMN IF NOT EXISTS resolved_workflow JSONB,
  ADD COLUMN IF NOT EXISTS final_parameters JSONB NOT NULL DEFAULT '{}'::jsonb;