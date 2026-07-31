BEGIN;

ALTER TABLE app.sales ADD COLUMN IF NOT EXISTS source_document_id text;
ALTER TABLE app.sales ADD COLUMN IF NOT EXISTS open_item_id text;
ALTER TABLE app.purchases ADD COLUMN IF NOT EXISTS open_item_id text;
ALTER TABLE app.payments ADD COLUMN IF NOT EXISTS open_item_id text;
ALTER TABLE app.open_item_applications ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'pending';
ALTER TABLE app.open_item_applications ADD COLUMN IF NOT EXISTS accounting_application_id text;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'open_item_applications_status_check'
      AND conrelid = 'app.open_item_applications'::regclass
  ) THEN
    ALTER TABLE app.open_item_applications
      ADD CONSTRAINT open_item_applications_status_check
      CHECK (status IN ('pending', 'applied', 'reversed'));
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS app.accounting_application_commands (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  source_kind text NOT NULL CHECK (source_kind IN ('payment_application', 'credit_note')),
  source_id text NOT NULL,
  debit_open_item_id text NOT NULL,
  credit_open_item_id text NOT NULL,
  amount numeric(20,6) NOT NULL CHECK (amount > 0),
  currency char(3) NOT NULL,
  status text NOT NULL CHECK (status IN ('pending', 'applied', 'reversed')),
  accounting_application_id text,
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, source_kind, source_id)
);

CREATE TABLE IF NOT EXISTS app.accounting_reversals (
  id text PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id),
  document_kind text NOT NULL CHECK (document_kind IN ('sale', 'purchase', 'payment')),
  document_id text NOT NULL,
  original_journal_entry_id text NOT NULL,
  effective_at timestamptz NOT NULL,
  reason text NOT NULL,
  status text NOT NULL CHECK (status IN ('requested', 'reversed')),
  reversal_journal_entry_id text,
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, document_kind, document_id)
);

ALTER TABLE app.accounting_application_commands ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_reversals ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_application_commands FORCE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_reversals FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS accounting_application_commands_org_isolation ON app.accounting_application_commands;
DROP POLICY IF EXISTS accounting_reversals_org_isolation ON app.accounting_reversals;
CREATE POLICY accounting_application_commands_org_isolation
  ON app.accounting_application_commands
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));
CREATE POLICY accounting_reversals_org_isolation
  ON app.accounting_reversals
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

COMMIT;
