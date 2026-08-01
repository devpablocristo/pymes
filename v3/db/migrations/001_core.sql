BEGIN;

CREATE SCHEMA IF NOT EXISTS app;

CREATE TABLE IF NOT EXISTS app.organizations (
  id text PRIMARY KEY,
  name text NOT NULL,
  slug text NOT NULL UNIQUE,
  status text NOT NULL CHECK (status IN ('pending', 'ready', 'failed', 'suspended')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app.organization_provisioning (
  organization_id text PRIMARY KEY REFERENCES app.organizations(id),
  accounting_status text NOT NULL CHECK (accounting_status IN ('pending', 'ready', 'failed')),
  fiscal_status text NOT NULL CHECK (fiscal_status IN ('pending', 'ready', 'failed')),
  last_error text,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app.parties (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  kind text NOT NULL CHECK (kind IN ('customer', 'supplier', 'both')),
  display_name text NOT NULL,
  tax_identifier text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app.sales (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  recipient_ref text NOT NULL,
  point_of_sale integer NOT NULL CHECK (point_of_sale > 0),
  document_type text NOT NULL,
  voucher_number integer NOT NULL CHECK (voucher_number > 0),
  amount numeric(20,6) NOT NULL CHECK (amount >= 0),
  currency char(3) NOT NULL,
  status text NOT NULL,
  snapshot_digest char(64) NOT NULL,
  cae text,
  journal_entry_id text,
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, point_of_sale, document_type, voucher_number)
);

CREATE TABLE IF NOT EXISTS app.idempotency_records (
  org_id text NOT NULL REFERENCES app.organizations(id),
  operation text NOT NULL,
  source_id text NOT NULL,
  source_version integer NOT NULL,
  payload_hash char(64) NOT NULL,
  response jsonb,
  completed_at timestamptz,
  PRIMARY KEY (org_id, operation, source_id, source_version)
);

CREATE TABLE IF NOT EXISTS app.outbox (
  id uuid PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  topic text NOT NULL,
  payload jsonb NOT NULL,
  payload_hash char(64) NOT NULL,
  idempotency_key text NOT NULL,
  correlation_id text NOT NULL,
  available_at timestamptz NOT NULL DEFAULT now(),
  attempts integer NOT NULL DEFAULT 0,
  lease_token text,
  lease_expires_at timestamptz,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, topic, idempotency_key)
);

CREATE INDEX IF NOT EXISTS outbox_available_idx ON app.outbox (available_at) WHERE published_at IS NULL;

ALTER TABLE app.parties ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.sales ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.parties FORCE ROW LEVEL SECURITY;
ALTER TABLE app.sales FORCE ROW LEVEL SECURITY;
ALTER TABLE app.idempotency_records FORCE ROW LEVEL SECURITY;
ALTER TABLE app.outbox FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS parties_org_isolation ON app.parties;
DROP POLICY IF EXISTS sales_org_isolation ON app.sales;
DROP POLICY IF EXISTS idempotency_org_isolation ON app.idempotency_records;
DROP POLICY IF EXISTS outbox_org_isolation ON app.outbox;
CREATE POLICY parties_org_isolation ON app.parties USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY sales_org_isolation ON app.sales USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY idempotency_org_isolation ON app.idempotency_records USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY outbox_org_isolation ON app.outbox USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));

COMMIT;
