-- 0011_calendar.up.sql
-- Calendar export (read-only iCal tokens) + sync (Google/Microsoft OAuth).
-- Consolida: 0054_calendar_export_tokens + 0055_calendar_sync_connections.

CREATE TABLE IF NOT EXISTS calendar_export_tokens (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by text NOT NULL DEFAULT '',
    name text NOT NULL DEFAULT '',
    token_hash text NOT NULL,
    scopes text NOT NULL DEFAULT 'all',
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT calendar_export_tokens_token_hash_uniq UNIQUE (token_hash)
);
CREATE INDEX IF NOT EXISTS idx_calendar_export_tokens_org_creator
    ON calendar_export_tokens(org_id, created_by) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS calendar_sync_connections (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by text NOT NULL DEFAULT '',
    provider text NOT NULL
        CONSTRAINT calendar_sync_connections_provider_check
        CHECK (provider IN ('google','microsoft')),
    provider_account_email text NOT NULL DEFAULT '',
    provider_calendar_id text NOT NULL DEFAULT '',
    provider_calendar_name text NOT NULL DEFAULT '',
    scopes text NOT NULL DEFAULT '',
    refresh_token_encrypted text NOT NULL,
    access_token_encrypted text NOT NULL DEFAULT '',
    access_token_expires_at timestamptz,
    sync_token text NOT NULL DEFAULT '',
    last_sync_at timestamptz,
    last_sync_error text NOT NULL DEFAULT '',
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_calendar_sync_connections_active
    ON calendar_sync_connections(org_id, created_by, provider) WHERE revoked_at IS NULL;

CREATE TRIGGER trg_calendar_sync_connections_updated_at
    BEFORE UPDATE ON calendar_sync_connections FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS calendar_sync_oauth_states (
    state text PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by text NOT NULL DEFAULT '',
    provider text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_calendar_sync_oauth_states_expiry
    ON calendar_sync_oauth_states(expires_at);

CREATE TABLE IF NOT EXISTS calendar_sync_errors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    connection_id uuid REFERENCES calendar_sync_connections(id) ON DELETE SET NULL,
    error_message text NOT NULL,
    error_code text NOT NULL DEFAULT '',
    context jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_calendar_sync_errors_org
    ON calendar_sync_errors(org_id, created_at DESC);
