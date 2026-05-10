-- 0012_ai.up.sql
-- AI dossiers (memoria semántica per-org), conversations, usage_daily, agent_events.
-- Consolida: 0014_ai_tables, 0020_ai_agent_events, 0032_ai_conversations_review_columns.

CREATE TABLE IF NOT EXISTS ai_dossiers (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    dossier jsonb NOT NULL DEFAULT '{}'::jsonb,
    version int NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_ai_dossiers_updated_at
    BEFORE UPDATE ON ai_dossiers FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS ai_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    mode text NOT NULL DEFAULT 'internal'
        CONSTRAINT ai_conversations_mode_check
        CHECK (mode IN ('internal','external')),
    external_contact text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    messages jsonb NOT NULL DEFAULT '[]'::jsonb,
    tool_calls_count int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    channel varchar(32),
    contact_phone varchar(32),
    contact_name varchar(255),
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    pending_action jsonb,
    review_request_id uuid,
    review_status varchar(32),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_org
    ON ai_conversations(org_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_user
    ON ai_conversations(org_id, user_id, updated_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ai_conversations_external
    ON ai_conversations(org_id, external_contact, updated_at DESC)
    WHERE mode = 'external' AND external_contact != '';
CREATE INDEX IF NOT EXISTS idx_ai_conversations_review_request
    ON ai_conversations(review_request_id) WHERE review_request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ai_conversations_contact_phone
    ON ai_conversations(org_id, contact_phone) WHERE contact_phone IS NOT NULL;

CREATE TRIGGER trg_ai_conversations_updated_at
    BEFORE UPDATE ON ai_conversations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS ai_usage_daily (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    usage_date date NOT NULL,
    queries int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, usage_date)
);

CREATE TABLE IF NOT EXISTS ai_agent_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    conversation_id uuid REFERENCES ai_conversations(id) ON DELETE SET NULL,
    external_request_id text,
    request_id text,
    capability_id text,
    confirmation_id text,
    review_request_id text,
    idempotency_key text,
    payload_hash text,
    agent_mode text NOT NULL,
    channel text NOT NULL,
    actor_id text NOT NULL,
    actor_type text NOT NULL,
    action text NOT NULL,
    tool_name text NOT NULL DEFAULT '',
    entity_type text NOT NULL DEFAULT '',
    entity_id text NOT NULL DEFAULT '',
    result text NOT NULL,
    confirmed boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ai_agent_events_org_created
    ON ai_agent_events(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_agent_events_conversation
    ON ai_agent_events(conversation_id, created_at DESC) WHERE conversation_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_agent_events_request_id
    ON ai_agent_events(org_id, external_request_id)
    WHERE external_request_id IS NOT NULL AND external_request_id <> '';
CREATE INDEX IF NOT EXISTS idx_ai_agent_events_capability
    ON ai_agent_events(org_id, capability_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_agent_events_review
    ON ai_agent_events(org_id, review_request_id);
