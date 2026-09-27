CREATE TABLE IF NOT EXISTS assistant_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tool TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('tool_execution','recommendation')),
    input JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    explanation TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    uncertainty TEXT NOT NULL,
    decision TEXT CHECK (decision IN ('apply','edit','ignore')),
    idempotency_key TEXT,
    decision_idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS assistant_actions_project_created_idx ON assistant_actions(project_id, created_at ASC);
CREATE INDEX IF NOT EXISTS assistant_actions_user_idx ON assistant_actions(user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS assistant_actions_user_idempotency_uq ON assistant_actions(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS assistant_actions_user_decision_idempotency_uq ON assistant_actions(user_id, decision_idempotency_key) WHERE decision_idempotency_key IS NOT NULL;
