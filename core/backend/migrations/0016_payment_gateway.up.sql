-- 0016_payment_gateway.up.sql
-- MercadoPago integration: connections (tokens encrypted), preferences,
-- webhooks (raw inbound), events (parsed events).
-- Consolida: 0016_payment_gateway, 0019_payment_gateway_events.

CREATE TABLE IF NOT EXISTS payment_gateway_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago'
        CONSTRAINT payment_gateway_connections_provider_check
        CHECK (provider IN ('mercadopago')),
    external_user_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    refresh_token_encrypted text NOT NULL,
    token_expires_at timestamptz NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    connected_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_payment_gateway_connections_updated_at
    BEFORE UPDATE ON payment_gateway_connections FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS payment_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago',
    external_id text NOT NULL DEFAULT '',
    reference_type text NOT NULL
        CONSTRAINT payment_preferences_reference_type_check
        CHECK (reference_type IN ('sale','quote')),
    reference_id uuid NOT NULL,
    amount numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    payment_url text NOT NULL DEFAULT '',
    qr_data text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT payment_preferences_status_check
        CHECK (status IN ('pending','approved','rejected','expired','refunded')),
    external_payer_id text NOT NULL DEFAULT '',
    paid_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_org
    ON payment_preferences(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_reference
    ON payment_preferences(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_external
    ON payment_preferences(provider, external_id) WHERE external_id != '';

CREATE TABLE IF NOT EXISTS payment_gateway_webhooks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL,
    external_webhook_id text NOT NULL,
    resource text NOT NULL,
    action text NOT NULL,
    raw_payload jsonb NOT NULL,
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_payment_gateway_webhooks_org
    ON payment_gateway_webhooks(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payment_gateway_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    external_event_id text NOT NULL,
    event_type text NOT NULL,
    raw_payload jsonb NOT NULL,
    signature text NOT NULL DEFAULT '',
    processed_at timestamptz,
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payment_gateway_events_provider_event_uniq
        UNIQUE (provider, external_event_id)
);
CREATE INDEX IF NOT EXISTS idx_payment_gateway_events_pending
    ON payment_gateway_events(created_at) WHERE processed_at IS NULL;
