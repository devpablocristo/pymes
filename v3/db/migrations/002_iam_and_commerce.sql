BEGIN;

-- Clerk remains the user identity authority.  These two projections are the
-- local authorization boundary used by every product query; they are never
-- inferred from an unverified HTTP header.
CREATE TABLE IF NOT EXISTS app.organization_identities (
  provider text NOT NULL CHECK (provider IN ('clerk')),
  provider_organization_id text NOT NULL,
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, provider_organization_id),
  UNIQUE (org_id, provider)
);

ALTER TABLE app.sales ADD COLUMN IF NOT EXISTS credential_ref text;
ALTER TABLE app.sales ADD COLUMN IF NOT EXISTS fiscal_snapshot jsonb;

CREATE TABLE IF NOT EXISTS app.memberships (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  provider text NOT NULL CHECK (provider IN ('clerk')),
  provider_user_id text NOT NULL,
  role text NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
  permissions jsonb NOT NULL DEFAULT '[]'::jsonb,
  status text NOT NULL CHECK (status IN ('active', 'inactive')),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, provider, provider_user_id)
);

CREATE TABLE IF NOT EXISTS app.purchases (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  supplier_ref text NOT NULL,
  external_document_ref text NOT NULL,
  amount numeric(20,6) NOT NULL CHECK (amount >= 0),
  currency char(3) NOT NULL,
  status text NOT NULL CHECK (status IN ('confirmed', 'posted', 'partially_paid', 'paid', 'reversed')),
  source_document_id text,
  journal_entry_id text,
  snapshot_digest char(64) NOT NULL,
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, external_document_ref)
);

CREATE TABLE IF NOT EXISTS app.payments (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  direction text NOT NULL CHECK (direction IN ('receipt', 'disbursement')),
  party_ref text NOT NULL,
  amount numeric(20,6) NOT NULL CHECK (amount > 0),
  currency char(3) NOT NULL,
  status text NOT NULL CHECK (status IN ('confirmed', 'posted', 'reversed')),
  journal_entry_id text,
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app.open_item_applications (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  payment_id text NOT NULL REFERENCES app.payments(id),
  document_kind text NOT NULL CHECK (document_kind IN ('sale', 'purchase')),
  document_id text NOT NULL,
  amount numeric(20,6) NOT NULL CHECK (amount > 0),
  currency char(3) NOT NULL,
  reversed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, payment_id, document_kind, document_id)
);

ALTER TABLE app.memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.purchases ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.open_item_applications ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.parties FORCE ROW LEVEL SECURITY;
ALTER TABLE app.sales FORCE ROW LEVEL SECURITY;
ALTER TABLE app.idempotency_records FORCE ROW LEVEL SECURITY;
ALTER TABLE app.outbox FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS memberships_org_isolation ON app.memberships;
DROP POLICY IF EXISTS purchases_org_isolation ON app.purchases;
DROP POLICY IF EXISTS payments_org_isolation ON app.payments;
DROP POLICY IF EXISTS open_item_applications_org_isolation ON app.open_item_applications;
CREATE POLICY memberships_org_isolation ON app.memberships USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY purchases_org_isolation ON app.purchases USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY payments_org_isolation ON app.payments USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY open_item_applications_org_isolation ON app.open_item_applications USING (org_id = current_setting('app.org_id', true)) WITH CHECK (org_id = current_setting('app.org_id', true));

COMMIT;
