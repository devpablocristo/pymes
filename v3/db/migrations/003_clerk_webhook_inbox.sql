BEGIN;

CREATE TABLE IF NOT EXISTS app.clerk_webhook_inbox (
  event_id text PRIMARY KEY,
  event_type text NOT NULL,
  occurred_at timestamptz NOT NULL,
  payload jsonb NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  last_error text
);

COMMIT;
