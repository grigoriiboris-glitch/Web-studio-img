CREATE TABLE similarity_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  visual_score DOUBLE PRECISION NOT NULL CHECK (visual_score >= 0 AND visual_score <= 1),
  composition_score DOUBLE PRECISION NOT NULL CHECK (composition_score >= 0 AND composition_score <= 1),
  semantic_score DOUBLE PRECISION NOT NULL CHECK (semantic_score >= 0 AND semantic_score <= 1),
  style_score DOUBLE PRECISION NOT NULL CHECK (style_score >= 0 AND style_score <= 1),
  search_scope TEXT NOT NULL,
  sources JSONB NOT NULL DEFAULT '[]'::jsonb,
  unavailable_sources JSONB NOT NULL DEFAULT '[]'::jsonb,
  algorithm TEXT NOT NULL,
  algorithm_version TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  idempotency_key TEXT
);
CREATE UNIQUE INDEX similarity_checks_user_idempotency_uq ON similarity_checks(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX similarity_checks_project_created_idx ON similarity_checks(project_id, created_at DESC);
CREATE INDEX similarity_checks_target_idx ON similarity_checks(target_asset_id);

CREATE TABLE export_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  final_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
  status TEXT NOT NULL DEFAULT 'running',
  artifacts JSONB NOT NULL DEFAULT '{}'::jsonb,
  manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  idempotency_key TEXT,
  CONSTRAINT export_records_status_check CHECK (status IN ('running','completed','failed'))
);
CREATE UNIQUE INDEX export_records_user_idempotency_uq ON export_records(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX export_records_project_created_idx ON export_records(project_id, created_at DESC);

CREATE TABLE composition_specs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  iteration_id UUID NOT NULL REFERENCES iterations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  focal_points JSONB NOT NULL DEFAULT '[]'::jsonb,
  bounding_boxes JSONB NOT NULL DEFAULT '[]'::jsonb,
  relative_positions JSONB NOT NULL DEFAULT '{}'::jsonb,
  horizon DOUBLE PRECISION CHECK (horizon IS NULL OR horizon >= 0 AND horizon <= 1),
  camera_elevation DOUBLE PRECISION,
  perspective TEXT,
  hierarchy JSONB NOT NULL DEFAULT '[]'::jsonb,
  negative_space JSONB NOT NULL DEFAULT '{}'::jsonb,
  dominant_geometry JSONB NOT NULL DEFAULT '{}'::jsonb,
  object_scale JSONB NOT NULL DEFAULT '{}'::jsonb,
  light_direction JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(iteration_id)
);
CREATE INDEX composition_specs_project_idx ON composition_specs(project_id, updated_at DESC);

CREATE TABLE composition_mutations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  composition_spec_id UUID REFERENCES composition_specs(id) ON DELETE SET NULL,
  source_similarity_check_id UUID REFERENCES similarity_checks(id) ON DELETE SET NULL,
  suggestions JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'proposed',
  accepted_iteration_id UUID REFERENCES iterations(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT composition_mutations_status_check CHECK (status IN ('proposed','accepted','rejected'))
);
CREATE INDEX composition_mutations_project_idx ON composition_mutations(project_id, created_at DESC);

CREATE TABLE library_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  category TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  prompt_fragment TEXT NOT NULL,
  preview_key TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT library_items_kind_check CHECK (kind IN ('material','texture')),
  CONSTRAINT library_items_category_not_blank CHECK (btrim(category) <> ''),
  CONSTRAINT library_items_name_not_blank CHECK (btrim(name) <> ''),
  CONSTRAINT library_items_prompt_not_blank CHECK (btrim(prompt_fragment) <> '')
);
CREATE INDEX library_items_kind_category_idx ON library_items(kind, category, created_at DESC);
CREATE INDEX library_items_user_kind_idx ON library_items(user_id, kind, created_at DESC);
CREATE UNIQUE INDEX library_system_name_uq ON library_items(kind, lower(name)) WHERE user_id IS NULL;

ALTER TABLE human_actions
  DROP CONSTRAINT IF EXISTS human_actions_type_check;
ALTER TABLE human_actions
  ADD CONSTRAINT human_actions_type_check CHECK (
    action_type IN (
      'IDEA_CREATED','PROMPT_EDITED','PROMPT_APPROVED','REFERENCE_ADDED','REFERENCE_SELECTED',
      'VARIANT_SELECTED','VARIANT_REJECTED','AI_RECOMMENDATION_REJECTED','COMPOSITION_CHANGED',
      'COMPOSITION_MUTATION_ACCEPTED','MATERIAL_SELECTED','TEXTURE_SELECTED','MANUAL_EDIT',
      'APPROVED','EXPORT_CREATED'
    )
  );

CREATE TABLE idempotency_requests (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  method TEXT NOT NULL,
  request_path TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash CHAR(64) NOT NULL,
  status_code INTEGER NOT NULL,
  response_content_type TEXT,
  response_body BYTEA NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, method, request_path, idempotency_key)
);
CREATE INDEX idempotency_requests_created_idx ON idempotency_requests(created_at);

INSERT INTO library_items (user_id, kind, category, name, description, tags, prompt_fragment)
VALUES
(NULL,'material','stone','marble','White and veined stone', '["stone","luxury"]'::jsonb,'white veined marble'),
(NULL,'material','metal','brushed metal','Directional brushed metallic surface','["metal","industrial"]'::jsonb,'brushed metal'),
(NULL,'material','wood','raw wood','Unfinished natural wood','["wood","natural"]'::jsonb,'raw wood'),
(NULL,'material','glass','glass','Transparent reflective glass','["glass","transparent"]'::jsonb,'clear glass'),
(NULL,'material','ceramic','ceramic','Smooth kiln-fired ceramic','["ceramic"]'::jsonb,'matte ceramic'),
(NULL,'material','paper','paper','Natural paper stock','["paper"]'::jsonb,'textured paper'),
(NULL,'material','textile','textile','Woven textile surface','["textile","fabric"]'::jsonb,'woven textile'),
(NULL,'material','concrete','concrete','Mineral concrete surface','["concrete"]'::jsonb,'raw concrete'),
(NULL,'material','liquid','liquid','Fluid glossy liquid','["liquid"]'::jsonb,'glossy liquid'),
(NULL,'material','plastic','plastic','Smooth polymer surface','["plastic"]'::jsonb,'smooth plastic'),
(NULL,'texture','natural','natural grain','Organic natural grain','["natural"]'::jsonb,'natural grain texture'),
(NULL,'texture','geometric','geometric grid','Regular geometric grid','["geometric"]'::jsonb,'geometric grid texture'),
(NULL,'texture','organic','organic noise','Irregular organic variation','["organic"]'::jsonb,'organic texture'),
(NULL,'texture','fabric','fabric weave','Visible woven fibers','["fabric"]'::jsonb,'fabric weave texture'),
(NULL,'texture','surface','fine surface','Subtle fine surface detail','["surface"]'::jsonb,'fine surface texture'),
(NULL,'texture','abstract','abstract pattern','Non-representational abstract marks','["abstract"]'::jsonb,'abstract texture'),
(NULL,'texture','custom','custom','User-definable custom texture','["custom"]'::jsonb,'custom texture')
ON CONFLICT DO NOTHING;
