CREATE TABLE IF NOT EXISTS agent_confirmations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text NOT NULL DEFAULT '',
    capability_id text NOT NULL,
    payload_hash text NOT NULL,
    human_summary text NOT NULL DEFAULT '',
    risk_level text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'used', 'expired', 'revoked')),
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_confirmations_org_actor
    ON agent_confirmations(org_id, actor, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_confirmations_capability
    ON agent_confirmations(org_id, capability_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_idempotency_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text NOT NULL DEFAULT '',
    capability_id text NOT NULL,
    idempotency_key text NOT NULL,
    payload_hash text NOT NULL,
    response jsonb NOT NULL,
    status_code int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, actor, capability_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_agent_idempotency_org_created
    ON agent_idempotency_records(org_id, created_at DESC);

ALTER TABLE IF EXISTS ai_agent_events
    ADD COLUMN IF NOT EXISTS request_id text,
    ADD COLUMN IF NOT EXISTS capability_id text,
    ADD COLUMN IF NOT EXISTS confirmation_id text,
    ADD COLUMN IF NOT EXISTS review_request_id text,
    ADD COLUMN IF NOT EXISTS idempotency_key text,
    ADD COLUMN IF NOT EXISTS payload_hash text;

CREATE INDEX IF NOT EXISTS idx_ai_agent_events_capability
    ON ai_agent_events(org_id, capability_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_agent_events_request
    ON ai_agent_events(org_id, request_id);
CREATE INDEX IF NOT EXISTS idx_ai_agent_events_review
    ON ai_agent_events(org_id, review_request_id);

ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS hash_version int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS payload_hash text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_audit_log_hash_version
    ON audit_log(org_id, hash_version, created_at DESC);

