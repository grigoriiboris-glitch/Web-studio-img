CREATE TABLE print_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  current_version INTEGER NOT NULL DEFAULT 1,
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT print_profiles_project_key_uq UNIQUE(project_id, key)
);

CREATE TABLE print_profile_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  print_profile_id UUID NOT NULL REFERENCES print_profiles(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  spec JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT print_profile_versions_profile_version_uq UNIQUE(print_profile_id, version)
);

CREATE INDEX print_profiles_project_idx ON print_profiles(project_id);
CREATE INDEX print_profile_versions_profile_idx ON print_profile_versions(print_profile_id, version DESC);
