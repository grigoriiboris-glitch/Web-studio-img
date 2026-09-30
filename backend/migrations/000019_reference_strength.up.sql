ALTER TABLE generations
  ADD COLUMN IF NOT EXISTS resolved_reference_influence JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE reference_usages
  ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'inspiration';

ALTER TABLE reference_usages
  DROP CONSTRAINT IF EXISTS reference_usages_role_check;

ALTER TABLE reference_usages
  ADD CONSTRAINT reference_usages_role_check
  CHECK (role IN ('inspiration','composition','subject','color','material','mood'));
