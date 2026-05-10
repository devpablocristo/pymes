-- 0013_agent.up.sql
-- Agent confirmations (token-based explicit user confirmation) +
-- idempotency records (replay protection).
-- Consolida: 0071_agent_readiness.

CREATE TABLE IF NOT EXISTS agent_confirmations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text NOT NULL DEFAULT '',
    capability_id text NOT NULL,
    payload_hash text NOT NULL,
    human_summary text NOT NULL DEFAULT '',
    risk_level text NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT agent_confirmations_status_check
        CHECK (status IN ('pending','used','expired','revoked')),
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
    CONSTRAINT agent_idempotency_records_org_actor_capability_key_uniq
        UNIQUE (org_id, actor, capability_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_agent_idempotency_org_created
    ON agent_idempotency_records(org_id, created_at DESC);

CREATE TRIGGER trg_agent_idempotency_updated_at
    BEFORE UPDATE ON agent_idempotency_records FOR EACH ROW EXECUTE FUNCTION set_updated_at();
