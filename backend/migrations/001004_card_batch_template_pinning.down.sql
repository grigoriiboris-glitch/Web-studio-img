DROP INDEX IF EXISTS card_batches_template_idx;
ALTER TABLE card_batches
  DROP COLUMN IF EXISTS template_id,
  DROP COLUMN IF EXISTS template_version;
