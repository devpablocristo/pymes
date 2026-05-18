CREATE TABLE IF NOT EXISTS ai_dossiers (
    tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    dossier jsonb NOT NULL DEFAULT '{}'::jsonb,
    version int NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id),
    mode text NOT NULL DEFAULT 'internal'
        CHECK (mode IN ('internal', 'external')),
    external_contact text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    messages jsonb NOT NULL DEFAULT '[]'::jsonb,
    tool_calls_count int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversations_org
    ON ai_conversations(tenant_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_user
    ON ai_conversations(tenant_id, user_id, updated_at DESC)
    WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ai_conversations_external
    ON ai_conversations(tenant_id, external_contact, updated_at DESC)
    WHERE mode = 'external' AND external_contact != '';

CREATE TABLE IF NOT EXISTS ai_usage_daily (
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    usage_date date NOT NULL,
    queries int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, usage_date)
);
