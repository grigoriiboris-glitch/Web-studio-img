
CREATE TABLE visual_dna_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT 'Visual DNA',
  version INTEGER NOT NULL CHECK (version > 0),
  algorithm_version TEXT NOT NULL,
  signals JSONB NOT NULL DEFAULT '{}'::jsonb,
  summary JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_assets JSONB NOT NULL DEFAULT '[]'::jsonb,
  source_iterations JSONB NOT NULL DEFAULT '[]'::jsonb,
  uncertainty JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT visual_dna_name_not_blank CHECK (btrim(name) <> ''),
  UNIQUE (user_id, project_id, version)
);

CREATE INDEX visual_dna_project_version_idx
  ON visual_dna_profiles(project_id, version DESC);

CREATE INDEX visual_dna_user_idx
  ON visual_dna_profiles(user_id, created_at DESC);
