CREATE TABLE asset_library_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  asset_type TEXT NOT NULL,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'active',
  current_version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT asset_library_name_not_blank CHECK (btrim(name) <> ''),
  CONSTRAINT asset_library_type_check CHECK (asset_type IN ('character','object','product','logo','symbol','background','texture','material','mask','image','other')),
  CONSTRAINT asset_library_status_check CHECK (status IN ('active','archived')),
  CONSTRAINT asset_library_version_positive CHECK (current_version > 0)
);

CREATE INDEX asset_library_items_user_idx ON asset_library_items(user_id, updated_at DESC);
CREATE INDEX asset_library_items_type_idx ON asset_library_items(user_id, asset_type, updated_at DESC);
CREATE INDEX asset_library_items_tags_idx ON asset_library_items USING GIN(tags);

CREATE TABLE asset_library_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  library_item_id UUID NOT NULL REFERENCES asset_library_items(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  source_asset_id UUID REFERENCES assets(id) ON DELETE SET NULL,
  source_project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  storage_key TEXT NOT NULL,
  preview_key TEXT,
  thumbnail_key TEXT,
  mime_type TEXT NOT NULL,
  size BIGINT NOT NULL,
  width INTEGER NOT NULL,
  height INTEGER NOT NULL,
  checksum CHAR(64) NOT NULL,
  rights_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT asset_library_versions_version_positive CHECK (version > 0),
  CONSTRAINT asset_library_versions_size_check CHECK (size > 0),
  CONSTRAINT asset_library_versions_dimensions_check CHECK (width > 0 AND height > 0),
  CONSTRAINT asset_library_versions_checksum_check CHECK (checksum ~ '^[0-9a-fA-F]{64}$'),
  UNIQUE(library_item_id, version)
);

CREATE INDEX asset_library_versions_item_idx ON asset_library_versions(library_item_id, version DESC);
CREATE INDEX asset_library_versions_source_asset_idx ON asset_library_versions(source_asset_id);
CREATE INDEX asset_library_versions_checksum_idx ON asset_library_versions(checksum);

CREATE TABLE asset_library_usages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  library_version_id UUID NOT NULL REFERENCES asset_library_versions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  rights_status TEXT NOT NULL DEFAULT 'inherited',
  rights_notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT asset_library_usage_rights_check CHECK (rights_status IN ('inherited','verified','unverified','restricted','unknown')),
  UNIQUE(library_version_id, project_id)
);

CREATE INDEX asset_library_usages_project_idx ON asset_library_usages(project_id, created_at DESC);
CREATE INDEX asset_library_usages_version_idx ON asset_library_usages(library_version_id, created_at DESC);

CREATE TABLE asset_library_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  library_item_id UUID REFERENCES asset_library_items(id) ON DELETE SET NULL,
  library_version_id UUID REFERENCES asset_library_versions(id) ON DELETE SET NULL,
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX asset_library_events_user_idx ON asset_library_events(user_id, created_at DESC);
CREATE INDEX asset_library_events_item_idx ON asset_library_events(library_item_id, created_at DESC);
