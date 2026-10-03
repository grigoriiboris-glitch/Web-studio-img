ALTER TABLE card_batches
  ADD COLUMN IF NOT EXISTS template_id UUID REFERENCES card_templates(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS template_version INTEGER;

CREATE INDEX IF NOT EXISTS card_batches_template_idx
  ON card_batches(template_id, template_version);
