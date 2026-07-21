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
    UNIQUE(provider, external_event_id)
);

CREATE INDEX IF NOT EXISTS idx_payment_gateway_events_pending
    ON payment_gateway_events(created_at)
    WHERE processed_at IS NULL;
