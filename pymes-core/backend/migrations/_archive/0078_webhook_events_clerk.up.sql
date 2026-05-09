-- Tabla de idempotencia para webhooks de Clerk. Cada evento entregado por
-- SVIX (firma `svix-id`) se almacena una sola vez. Si Clerk reintenta el
-- mismo evento (mismo `svix_id`), el INSERT falla por la UNIQUE y el
-- handler retorna 200 idempotente sin reprocesar.
CREATE TABLE IF NOT EXISTS webhook_events_clerk (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    svix_id text NOT NULL UNIQUE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processed', 'failed', 'ignored')),
    error_message text,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_status ON webhook_events_clerk (status);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_event_type ON webhook_events_clerk (event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_events_clerk_received_at ON webhook_events_clerk (received_at DESC);
