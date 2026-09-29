ALTER TABLE generations
  ADD COLUMN IF NOT EXISTS provider_deterministic BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS determinism_note TEXT NOT NULL DEFAULT 'Provider determinism is not guaranteed unless explicitly stated.',
  ADD COLUMN IF NOT EXISTS reference_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS reference_usages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  reference_id UUID NOT NULL REFERENCES "references"(id) ON DELETE CASCADE,
  iteration_id UUID REFERENCES iterations(id) ON DELETE CASCADE,
  generation_id UUID REFERENCES generations(id) ON DELETE CASCADE,
  usage_type TEXT NOT NULL DEFAULT 'influence',
  influence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT reference_usages_target_check CHECK (iteration_id IS NOT NULL OR generation_id IS NOT NULL),
  CONSTRAINT reference_usages_type_check CHECK (btrim(usage_type) <> '')
);

CREATE INDEX IF NOT EXISTS reference_usages_project_idx
  ON reference_usages(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS reference_usages_reference_idx
  ON reference_usages(reference_id, created_at DESC);
CREATE INDEX IF NOT EXISTS reference_usages_iteration_idx
  ON reference_usages(iteration_id, created_at DESC);
CREATE INDEX IF NOT EXISTS reference_usages_generation_idx
  ON reference_usages(generation_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS reference_usages_generation_reference_uq
  ON reference_usages(reference_id, generation_id, usage_type)
  WHERE generation_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS reference_usages_iteration_reference_uq
  ON reference_usages(reference_id, iteration_id, usage_type)
  WHERE iteration_id IS NOT NULL;
