
DROP TABLE IF EXISTS project_events;
DROP TABLE IF EXISTS provenance_events;
DROP TABLE IF EXISTS human_actions;
DROP TABLE IF EXISTS "references";
DROP TABLE IF EXISTS prompts;
DROP INDEX IF EXISTS assets_user_checksum_uq;
ALTER TABLE assets DROP COLUMN IF EXISTS exif, DROP COLUMN IF EXISTS checksum, DROP COLUMN IF EXISTS thumbnail_key, DROP COLUMN IF EXISTS preview_key, DROP COLUMN IF EXISTS user_id;
ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_cost_check_v2;
DROP INDEX IF EXISTS generations_user_id_idempotency_uq;
ALTER TABLE generations DROP COLUMN IF EXISTS started_at, DROP COLUMN IF EXISTS cost, DROP COLUMN IF EXISTS error_message, DROP COLUMN IF EXISTS error_code, DROP COLUMN IF EXISTS idempotency_key;
UPDATE generations SET status = 'completed' WHERE status = 'succeeded';
ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_status_check;
ALTER TABLE generations ADD CONSTRAINT generations_status_check CHECK (status IN ('queued','running','completed','failed','cancelled'));
ALTER TABLE generations ALTER COLUMN iteration_id SET NOT NULL;
ALTER TABLE generations DROP COLUMN IF EXISTS user_id;
