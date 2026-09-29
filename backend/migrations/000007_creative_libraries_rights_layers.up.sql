CREATE TABLE style_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
  version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
  prompt_influence BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT style_profiles_name_not_blank CHECK (btrim(name) <> '')
);
CREATE INDEX style_profiles_user_idx ON style_profiles(user_id, updated_at DESC);

CREATE TABLE asset_dna_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  features JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_iterations JSONB NOT NULL DEFAULT '[]'::jsonb,
  source_assets JSONB NOT NULL DEFAULT '[]'::jsonb,
  reusable BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT asset_dna_name_not_blank CHECK (btrim(name) <> '')
);
CREATE INDEX asset_dna_project_idx ON asset_dna_profiles(project_id, updated_at DESC);

CREATE TABLE visual_language_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  signals JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_iterations JSONB NOT NULL DEFAULT '[]'::jsonb,
  version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
  prompt_influence BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT visual_language_name_not_blank CHECK (btrim(name) <> '')
);
CREATE INDEX visual_language_user_idx ON visual_language_profiles(user_id, updated_at DESC);

CREATE TABLE rights_registry (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  target_type TEXT NOT NULL,
  target_id UUID NOT NULL,
  ownership TEXT NOT NULL DEFAULT 'unknown',
  license TEXT NOT NULL DEFAULT 'unknown',
  license_source TEXT,
  verification_state TEXT NOT NULL DEFAULT 'unknown',
  verification_date TIMESTAMPTZ,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT rights_target_type_check CHECK (target_type IN ('asset','reference')),
  CONSTRAINT rights_ownership_check CHECK (ownership IN ('user_owned','third_party','public_domain','unknown')),
  CONSTRAINT rights_verification_state_check CHECK (verification_state IN ('verified','unverified','unknown','restricted')),
  UNIQUE(user_id, target_type, target_id)
);
CREATE INDEX rights_registry_project_idx ON rights_registry(project_id, updated_at DESC);

CREATE TABLE do_not_use_constraints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  value TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT do_not_use_kind_check CHECK (kind IN ('artist','image','reference','motif','brand','composition','style')),
  CONSTRAINT do_not_use_value_not_blank CHECK (btrim(value) <> '')
);
CREATE INDEX do_not_use_user_project_idx ON do_not_use_constraints(user_id, project_id, active);

CREATE TABLE project_layers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  iteration_id UUID REFERENCES iterations(id) ON DELETE SET NULL,
  asset_id UUID REFERENCES assets(id) ON DELETE SET NULL,
  parent_layer_id UUID REFERENCES project_layers(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  layer_type TEXT NOT NULL,
  order_index INTEGER NOT NULL DEFAULT 0,
  visible BOOLEAN NOT NULL DEFAULT true,
  opacity DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK (opacity >= 0 AND opacity <= 1),
  blend_mode TEXT NOT NULL DEFAULT 'normal',
  source_kind TEXT NOT NULL DEFAULT 'derived',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT project_layers_name_not_blank CHECK (btrim(name) <> ''),
  CONSTRAINT project_layers_type_check CHECK (layer_type IN ('base','mask','image','manual_edit','adjustment','group')),
  CONSTRAINT project_layers_source_kind_check CHECK (source_kind IN ('human','ai','derived','imported'))
);
CREATE INDEX project_layers_project_order_idx ON project_layers(project_id, order_index, created_at);
CREATE INDEX project_layers_asset_idx ON project_layers(asset_id);

CREATE TABLE manual_edits (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  iteration_id UUID REFERENCES iterations(id) ON DELETE SET NULL,
  source_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
  mask_asset_id UUID REFERENCES assets(id) ON DELETE RESTRICT,
  result_asset_id UUID REFERENCES assets(id) ON DELETE SET NULL,
  operation TEXT NOT NULL,
  prompt TEXT,
  parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT manual_edits_operation_check CHECK (operation IN ('paint','erase','mask','composite')),
  CONSTRAINT manual_edits_status_check CHECK (status IN ('draft','applied','rejected'))
);
CREATE INDEX manual_edits_project_idx ON manual_edits(project_id, created_at DESC);
CREATE INDEX manual_edits_source_idx ON manual_edits(source_asset_id);

ALTER TABLE human_actions DROP CONSTRAINT IF EXISTS human_actions_type_check;
ALTER TABLE human_actions ADD CONSTRAINT human_actions_type_check CHECK (
  action_type IN (
    'IDEA_CREATED','PROMPT_EDITED','PROMPT_APPROVED','REFERENCE_ADDED','REFERENCE_SELECTED',
    'VARIANT_SELECTED','VARIANT_REJECTED','AI_RECOMMENDATION_REJECTED','COMPOSITION_CHANGED',
    'COMPOSITION_MUTATION_ACCEPTED','MATERIAL_SELECTED','TEXTURE_SELECTED','MANUAL_EDIT',
    'APPROVED','EXPORT_CREATED','MATERIAL_CREATED','MATERIAL_UPDATED','MATERIAL_DELETED',
    'TEXTURE_CREATED','TEXTURE_UPDATED','TEXTURE_DELETED','STYLE_PROFILE_CREATED',
    'STYLE_PROFILE_UPDATED','STYLE_PROFILE_APPLIED','ASSET_DNA_CREATED','VISUAL_LANGUAGE_UPDATED',
    'RIGHTS_UPDATED','DO_NOT_USE_UPDATED','LAYER_CREATED','LAYER_UPDATED','LAYER_DELETED',
    'MASK_CREATED','MANUAL_EDIT_CREATED','MANUAL_EDIT_APPLIED','MANUAL_EDIT_REJECTED'
  )
);