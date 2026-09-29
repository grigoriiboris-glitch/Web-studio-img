DROP TABLE IF EXISTS reference_usages;
ALTER TABLE generations
  DROP COLUMN IF EXISTS reference_ids,
  DROP COLUMN IF EXISTS determinism_note,
  DROP COLUMN IF EXISTS provider_deterministic;
