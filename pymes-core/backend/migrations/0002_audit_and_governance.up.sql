-- 0002_audit_and_governance.up.sql
-- Audit log con hash chain, protected resources, restore evidence, tenant
-- invitations (Clerk), webhook events de Clerk.
--
-- audit_log usa schema saas (org_id + payload_json) pero conserva el hash
-- chain que pymes-core agregó en 0009 / 0071 (prev_hash + hash) — fraud
-- prevention y trazabilidad de dinero (ver pymes-core/docs/FRAUD_PREVENTION.md).

CREATE TABLE IF NOT EXISTS audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    prev_hash text,
    hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_log_org_created
    ON audit_log(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_org_action
    ON audit_log(org_id, action);

CREATE TABLE IF NOT EXISTS protected_resources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    resource_type text NOT NULL,
    match_value text NOT NULL,
    match_mode text NOT NULL DEFAULT 'exact'
        CONSTRAINT protected_resources_match_mode_check
        CHECK (match_mode IN ('exact','prefix','regex')),
    environment text NOT NULL DEFAULT '*',
    reason text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    created_by text,
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_protected_resources_org_created
    ON protected_resources(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_protected_resources_org_enabled
    ON protected_resources(org_id, enabled);

CREATE TABLE IF NOT EXISTS restore_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment text NOT NULL DEFAULT 'prod',
    system text NOT NULL,
    status text NOT NULL,
    snapshot_id text NOT NULL DEFAULT '',
    restore_target text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    source text NOT NULL DEFAULT '',
    artifact_sha256 text NOT NULL DEFAULT '',
    summary_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_restore_evidence_org_created
    ON restore_evidence(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_restore_evidence_org_system_env
    ON restore_evidence(org_id, system, environment, created_at DESC);

CREATE TABLE IF NOT EXISTS tenant_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email text NOT NULL,
    role text NOT NULL DEFAULT 'member'
        CONSTRAINT tenant_invitations_role_check CHECK (role IN ('admin','member','secops')),
    token text NOT NULL UNIQUE,
    invited_by text,
    accepted_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tenant_invitations_org ON tenant_invitations(org_id);
CREATE INDEX IF NOT EXISTS idx_tenant_invitations_email ON tenant_invitations(lower(email));

CREATE TABLE IF NOT EXISTS webhook_events_clerk (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    svix_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payload_json jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    error_message text
);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_received
    ON webhook_events_clerk(received_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_unprocessed
    ON webhook_events_clerk(received_at) WHERE processed_at IS NULL;

CREATE TRIGGER trg_protected_resources_updated_at
    BEFORE UPDATE ON protected_resources
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
