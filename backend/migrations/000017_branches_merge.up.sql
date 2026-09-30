CREATE TABLE branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    parent_iteration_id UUID REFERENCES iterations(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'active',
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT branches_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT branches_name_length CHECK (char_length(name) <= 200),
    CONSTRAINT branches_status_check CHECK (status IN ('active', 'archived'))
);

CREATE UNIQUE INDEX branches_project_name_uq ON branches (project_id, lower(name));
CREATE INDEX branches_project_idx ON branches (project_id, created_at);
CREATE INDEX branches_parent_iteration_idx ON branches (parent_iteration_id);

ALTER TABLE iterations
    ADD COLUMN branch_id UUID REFERENCES branches(id) ON DELETE RESTRICT,
    ADD COLUMN decisions JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN merge_sources JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX iterations_branch_created_idx ON iterations (branch_id, created_at);

INSERT INTO branches (project_id, name, created_by)
SELECT p.id, 'main', p.user_id
FROM projects p;

UPDATE iterations i
SET branch_id = b.id
FROM branches b
WHERE b.project_id = i.project_id
  AND lower(b.name) = 'main';

ALTER TABLE iterations
    ALTER COLUMN branch_id SET NOT NULL;

ALTER TABLE iterations
    ADD CONSTRAINT iterations_decisions_object_check
    CHECK (jsonb_typeof(decisions) = 'object');