-- 0002_audit_and_governance.up.sql
-- Audit log con hash chain, protected resources, restore evidence, tenant
-- invitations (Clerk), webhook events de Clerk.
--
-- audit_log usa schema saas (org_id + payload) pero conserva el hash chain
-- que core agregó en 0009 / 0071 (prev_hash + hash) — fraud prevention
-- y trazabilidad de dinero (ver core/docs/FRAUD_PREVENTION.md).

CREATE TABLE IF NOT EXISTS audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    actor_type text NOT NULL DEFAULT 'user',
    actor_id uuid,
    actor_label text NOT NULL DEFAULT '',
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    payload_hash text NOT NULL DEFAULT '',
    prev_hash text,
    hash text NOT NULL,
    hash_version int NOT NULL DEFAULT 1,
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

-- tenant_invitations: schema completo legacy (legacy 0075).
CREATE TABLE IF NOT EXISTS tenant_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email_normalized text NOT NULL,
    role text NOT NULL
        CONSTRAINT tenant_invitations_role_check CHECK (role IN ('admin','member')),
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT tenant_invitations_status_check CHECK (status IN ('pending','accepted','revoked','expired')),
    token_hash text NOT NULL UNIQUE,
    clerk_invitation_id text,
    invited_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    accepted_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_invitations_pending_email
    ON tenant_invitations(org_id, email_normalized) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tenant_invitations_org_status
    ON tenant_invitations(org_id, status, created_at DESC);

CREATE TRIGGER trg_tenant_invitations_updated_at
    BEFORE UPDATE ON tenant_invitations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- webhook_events_clerk: schema completo con lifecycle (legacy 0078).
CREATE TABLE IF NOT EXISTS webhook_events_clerk (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    svix_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT webhook_events_clerk_status_check
        CHECK (status IN ('pending','processed','failed','ignored')),
    error_message text,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_status
    ON webhook_events_clerk(status);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_event_type
    ON webhook_events_clerk(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_received_at
    ON webhook_events_clerk(received_at DESC);

CREATE TRIGGER trg_webhook_events_clerk_updated_at
    BEFORE UPDATE ON webhook_events_clerk FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_protected_resources_updated_at
    BEFORE UPDATE ON protected_resources
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
