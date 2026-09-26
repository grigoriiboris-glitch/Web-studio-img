CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    avatar TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_email_not_blank CHECK (btrim(email) <> ''),
    CONSTRAINT users_name_not_blank CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT projects_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT projects_status_check CHECK (status IN ('active', 'archived', 'deleted'))
);

CREATE INDEX projects_user_id_idx ON projects (user_id);
CREATE INDEX projects_user_updated_idx ON projects (user_id, updated_at DESC);

CREATE TABLE iterations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_iteration_id UUID REFERENCES iterations(id) ON DELETE RESTRICT,
    type TEXT NOT NULL,
    title TEXT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT iterations_type_check CHECK (
        type IN ('idea', 'sketch', 'generation', 'selection', 'composition', 'prompt', 'manual_edit', 'final')
    ),
    CONSTRAINT iterations_parent_not_self CHECK (parent_iteration_id IS NULL OR parent_iteration_id <> id)
);

CREATE INDEX iterations_project_created_idx ON iterations (project_id, created_at);
CREATE INDEX iterations_parent_idx ON iterations (parent_iteration_id);

CREATE TABLE generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    iteration_id UUID NOT NULL REFERENCES iterations(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    provider_job_id TEXT,
    model TEXT NOT NULL,
    model_version TEXT,
    prompt TEXT NOT NULL,
    negative_prompt TEXT,
    seed BIGINT,
    aspect_ratio TEXT,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'queued',
    error TEXT,
    estimated_cost NUMERIC(12, 4),
    actual_cost NUMERIC(12, 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT generations_status_check CHECK (
        status IN ('queued', 'running', 'completed', 'failed', 'cancelled')
    ),
    CONSTRAINT generations_cost_check CHECK (
        (estimated_cost IS NULL OR estimated_cost >= 0)
        AND (actual_cost IS NULL OR actual_cost >= 0)
    )
);

CREATE UNIQUE INDEX generations_provider_job_uq
    ON generations (provider, provider_job_id)
    WHERE provider_job_id IS NOT NULL;
CREATE INDEX generations_project_created_idx ON generations (project_id, created_at DESC);
CREATE INDEX generations_iteration_idx ON generations (iteration_id);
CREATE INDEX generations_status_idx ON generations (status);

CREATE TABLE assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    generation_id UUID REFERENCES generations(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size BIGINT NOT NULL,
    width INTEGER,
    height INTEGER,
    sha256 CHAR(64),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assets_type_not_blank CHECK (btrim(type) <> ''),
    CONSTRAINT assets_storage_key_not_blank CHECK (btrim(storage_key) <> ''),
    CONSTRAINT assets_mime_type_not_blank CHECK (btrim(mime_type) <> ''),
    CONSTRAINT assets_size_check CHECK (size >= 0),
    CONSTRAINT assets_dimensions_check CHECK (
        (width IS NULL OR width > 0) AND (height IS NULL OR height > 0)
    ),
    CONSTRAINT assets_sha256_check CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-fA-F]{64}$')
);

CREATE INDEX assets_project_created_idx ON assets (project_id, created_at DESC);
CREATE INDEX assets_generation_idx ON assets (generation_id);
CREATE INDEX assets_sha256_idx ON assets (sha256) WHERE sha256 IS NOT NULL;
