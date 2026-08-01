BEGIN;

CREATE TABLE IF NOT EXISTS app.outbox_dead_letters (
  id uuid PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  topic text NOT NULL,
  payload jsonb NOT NULL,
  payload_hash char(64) NOT NULL,
  idempotency_key text NOT NULL,
  correlation_id text NOT NULL,
  attempts integer NOT NULL CHECK (attempts > 0),
  failure_code text NOT NULL,
  failed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS outbox_dead_letters_failed_at_idx ON app.outbox_dead_letters (failed_at DESC);

ALTER TABLE app.outbox_dead_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.outbox_dead_letters FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS outbox_dead_letters_org_isolation ON app.outbox_dead_letters;
CREATE POLICY outbox_dead_letters_org_isolation ON app.outbox_dead_letters
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

COMMIT;
