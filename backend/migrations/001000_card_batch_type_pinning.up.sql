ALTER TABLE card_batches
  ADD COLUMN IF NOT EXISTS card_type_id UUID REFERENCES card_type_definitions(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS card_type_version INTEGER;

CREATE INDEX IF NOT EXISTS card_batches_card_type_idx
  ON card_batches(card_type_id, card_type_version);
