BEGIN;

-- Durable, tenant-scoped record of every Fiscal and Accounting response that
-- Pymes applies. The response and the local state transition are committed in
-- the same transaction, so a retry can prove whether the exact response was
-- already consumed without trusting an in-memory worker.
CREATE TABLE IF NOT EXISTS app.service_response_inbox (
  -- Deliberately no FK to organizations: the immutable consumption proof must
  -- outlive an operational tenant row and cannot be removed by TRUNCATE CASCADE.
  org_id text NOT NULL,
  service text NOT NULL CHECK (service IN ('fiscal', 'accounting')),
  operation text NOT NULL CHECK (btrim(operation) <> ''),
  request_id text NOT NULL CHECK (btrim(request_id) <> ''),
  idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
  source_version integer NOT NULL CHECK (source_version > 0),
  snapshot_digest char(64) NOT NULL
    CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
  correlation_id text NOT NULL CHECK (btrim(correlation_id) <> ''),
  payload_hash char(64) NOT NULL
    CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
  response jsonb NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  applied_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, service, request_id),
  UNIQUE (org_id, service, operation, idempotency_key)
);

ALTER TABLE app.service_response_inbox
  DROP CONSTRAINT IF EXISTS service_response_inbox_org_id_fkey;

CREATE INDEX IF NOT EXISTS service_response_inbox_received_at_idx
  ON app.service_response_inbox (received_at DESC);

ALTER TABLE app.service_response_inbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.service_response_inbox FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS service_response_inbox_org_isolation
  ON app.service_response_inbox;
CREATE POLICY service_response_inbox_org_isolation
  ON app.service_response_inbox
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

CREATE OR REPLACE FUNCTION app.reject_service_response_inbox_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'service response inbox is immutable'
    USING ERRCODE = '55000';
END
$$;

DROP TRIGGER IF EXISTS service_response_inbox_no_update_delete
  ON app.service_response_inbox;
CREATE TRIGGER service_response_inbox_no_update_delete
  BEFORE UPDATE OR DELETE ON app.service_response_inbox
  FOR EACH ROW
  EXECUTE FUNCTION app.reject_service_response_inbox_mutation();

DROP TRIGGER IF EXISTS service_response_inbox_no_truncate
  ON app.service_response_inbox;
CREATE TRIGGER service_response_inbox_no_truncate
  BEFORE TRUNCATE ON app.service_response_inbox
  FOR EACH STATEMENT
  EXECUTE FUNCTION app.reject_service_response_inbox_mutation();

COMMIT;
