ALTER TABLE card_batches
  ADD COLUMN IF NOT EXISTS print_profile_id UUID REFERENCES print_profiles(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS print_profile_version INTEGER;

CREATE INDEX IF NOT EXISTS card_batches_print_profile_idx
  ON card_batches(print_profile_id, print_profile_version);
