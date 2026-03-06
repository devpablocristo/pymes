CREATE TABLE IF NOT EXISTS ai_agent_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    conversation_id uuid REFERENCES ai_conversations(id) ON DELETE SET NULL,
    external_request_id text,
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

CREATE INDEX IF NOT EXISTS idx_ai_agent_events_org_created_at
    ON ai_agent_events(org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_agent_events_conversation
    ON ai_agent_events(conversation_id, created_at DESC)
    WHERE conversation_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_agent_events_request_id
    ON ai_agent_events(org_id, external_request_id)
    WHERE external_request_id IS NOT NULL AND external_request_id <> '';
