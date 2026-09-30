ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_privacy_mode_check;
ALTER TABLE projects DROP COLUMN IF EXISTS privacy_mode;
