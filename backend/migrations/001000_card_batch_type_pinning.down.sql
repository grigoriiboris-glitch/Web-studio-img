DROP INDEX IF EXISTS card_batches_card_type_idx;
ALTER TABLE card_batches DROP COLUMN IF EXISTS card_type_version;
ALTER TABLE card_batches DROP COLUMN IF EXISTS card_type_id;
