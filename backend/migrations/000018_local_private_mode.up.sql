ALTER TABLE projects ADD COLUMN privacy_mode TEXT NOT NULL DEFAULT 'project_default';
ALTER TABLE projects ADD CONSTRAINT projects_privacy_mode_check CHECK (privacy_mode IN ('local_only','provider_allowed','project_default'));
