DROP INDEX IF EXISTS card_batches_print_profile_idx;
ALTER TABLE card_batches
  DROP COLUMN IF EXISTS print_profile_version,
  DROP COLUMN IF EXISTS print_profile_id;
