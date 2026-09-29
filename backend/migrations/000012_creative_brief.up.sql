CREATE TABLE IF NOT EXISTS creative_briefs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  version INTEGER NOT NULL CHECK (version > 0),
  title TEXT NOT NULL,
  goal TEXT NOT NULL,
  audience TEXT,
  deliverable TEXT,
  aspect_ratio TEXT,
  target_width INTEGER,
  target_height INTEGER,
  subject TEXT,
  must_have JSONB NOT NULL DEFAULT '[]'::jsonb,
  avoid JSONB NOT NULL DEFAULT '[]'::jsonb,
  mood TEXT,
  required_elements JSONB NOT NULL DEFAULT '[]'::jsonb,
  constraints JSONB NOT NULL DEFAULT '[]'::jsonb,
  success_criteria JSONB NOT NULL DEFAULT '[]'::jsonb,
  deadline TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT creative_briefs_status_check CHECK (status IN ('draft','approved')),
  CONSTRAINT creative_briefs_title_not_blank CHECK (btrim(title) <> ''),
  UNIQUE(project_id, version)
);

CREATE INDEX IF NOT EXISTS creative_briefs_project_version_idx
  ON creative_briefs(project_id, version DESC);

CREATE UNIQUE INDEX IF NOT EXISTS creative_briefs_project_approved_uq
  ON creative_briefs(project_id)
  WHERE status = 'approved';

ALTER TABLE human_actions DROP CONSTRAINT IF EXISTS human_actions_type_check;
ALTER TABLE human_actions ADD CONSTRAINT human_actions_type_check CHECK (
  action_type IN (
    'IDEA_CREATED','SKETCH_IMPORTED','PROMPT_EDITED','PROMPT_APPROVED','REFERENCE_ADDED','REFERENCE_SELECTED',
    'VARIANT_SELECTED','VARIANT_REJECTED','AI_RECOMMENDATION_REJECTED','COMPOSITION_CHANGED',
    'COMPOSITION_MUTATION_ACCEPTED','MATERIAL_SELECTED','TEXTURE_SELECTED','MANUAL_EDIT','APPROVED','EXPORT_CREATED',
    'MATERIAL_CREATED','MATERIAL_UPDATED','MATERIAL_DELETED','TEXTURE_CREATED','TEXTURE_UPDATED','TEXTURE_DELETED',
    'STYLE_PROFILE_CREATED','STYLE_PROFILE_UPDATED','STYLE_PROFILE_APPLIED','ASSET_DNA_CREATED','VISUAL_LANGUAGE_UPDATED',
    'RIGHTS_UPDATED','DO_NOT_USE_UPDATED','LAYER_CREATED','LAYER_UPDATED','LAYER_DELETED','MASK_CREATED',
    'MANUAL_EDIT_CREATED','MANUAL_EDIT_APPLIED','MANUAL_EDIT_REJECTED','CREATIVE_BRIEF_CREATED','CREATIVE_BRIEF_APPROVED'
  )
);