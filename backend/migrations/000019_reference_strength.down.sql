ALTER TABLE reference_usages
  DROP CONSTRAINT IF EXISTS reference_usages_role_check;
ALTER TABLE reference_usages
  DROP COLUMN IF EXISTS role;
ALTER TABLE generations
  DROP COLUMN IF EXISTS resolved_reference_influence;
