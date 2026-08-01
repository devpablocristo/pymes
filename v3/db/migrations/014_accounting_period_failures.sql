BEGIN;

-- PERIOD_LOCKED is a recoverable accounting outcome, not a transport failure.
-- Keep the exact rejected command and its origin so an authorized user can
-- explicitly choose an effective date in an open period without changing the
-- source document or any journal entry.
CREATE TABLE IF NOT EXISTS app.accounting_failures (
  id uuid NOT NULL,
  org_id text NOT NULL REFERENCES app.organizations(id),
  original_event_id uuid NOT NULL,
  source_kind text NOT NULL CHECK (
    source_kind IN ('sale', 'purchase', 'payment', 'accounting_application', 'accounting_reversal')
  ),
  source_id text NOT NULL,
  command_kind text NOT NULL CHECK (
    command_kind IN ('posting', 'application', 'reversal', 'application_reversal')
  ),
  command_payload jsonb NOT NULL,
  command_digest char(64) NOT NULL CHECK (command_digest ~ '^[0-9a-f]{64}$'),
  failed_effective_at timestamptz NOT NULL,
  status text NOT NULL CHECK (
    status IN ('awaiting_adjustment', 'adjustment_pending', 'resolved')
  ),
  failure_code text NOT NULL CHECK (failure_code = 'PERIOD_LOCKED'),
  request_id text NOT NULL,
  actor_ref text NOT NULL,
  source_version integer NOT NULL CHECK (source_version > 0),
  snapshot_digest char(64) NOT NULL CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, original_event_id)
);

CREATE TABLE IF NOT EXISTS app.accounting_adjustments (
  id text NOT NULL,
  org_id text NOT NULL REFERENCES app.organizations(id),
  failure_id uuid NOT NULL,
  effective_at timestamptz NOT NULL,
  reason text NOT NULL CHECK (length(btrim(reason)) > 0 AND length(reason) <= 500),
  status text NOT NULL CHECK (status IN ('pending', 'period_locked', 'posted')),
  request_id text NOT NULL,
  actor_ref text NOT NULL,
  source_version integer NOT NULL CHECK (source_version > 0),
  snapshot_digest char(64) NOT NULL CHECK (snapshot_digest ~ '^[0-9a-f]{64}$'),
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, failure_id)
    REFERENCES app.accounting_failures(org_id, id)
);

ALTER TABLE app.sales
  ADD COLUMN IF NOT EXISTS accounting_failure_id uuid,
  ADD COLUMN IF NOT EXISTS accounting_failure_code text;
ALTER TABLE app.purchases
  ADD COLUMN IF NOT EXISTS accounting_failure_id uuid,
  ADD COLUMN IF NOT EXISTS accounting_failure_code text;
ALTER TABLE app.payments
  ADD COLUMN IF NOT EXISTS accounting_failure_id uuid,
  ADD COLUMN IF NOT EXISTS accounting_failure_code text;
ALTER TABLE app.accounting_application_commands
  ADD COLUMN IF NOT EXISTS accounting_failure_id uuid,
  ADD COLUMN IF NOT EXISTS accounting_failure_code text;
ALTER TABLE app.accounting_reversals
  ADD COLUMN IF NOT EXISTS accounting_failure_id uuid,
  ADD COLUMN IF NOT EXISTS accounting_failure_code text;

ALTER TABLE app.purchases DROP CONSTRAINT IF EXISTS purchases_status_check;
ALTER TABLE app.purchases
  ADD CONSTRAINT purchases_status_check CHECK (
    status IN (
      'confirmed', 'posted', 'partially_paid', 'paid', 'reversed',
      'accounting_adjustment_required', 'accounting_adjustment_pending'
    )
  );
ALTER TABLE app.payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE app.payments
  ADD CONSTRAINT payments_status_check CHECK (
    status IN (
      'confirmed', 'posted', 'reversed',
      'accounting_adjustment_required', 'accounting_adjustment_pending'
    )
  );
ALTER TABLE app.accounting_application_commands
  DROP CONSTRAINT IF EXISTS accounting_application_commands_status_check;
ALTER TABLE app.accounting_application_commands
  ADD CONSTRAINT accounting_application_commands_status_check CHECK (
    status IN (
      'pending', 'applied', 'reversed',
      'accounting_adjustment_required', 'accounting_adjustment_pending'
    )
  );
ALTER TABLE app.accounting_reversals
  DROP CONSTRAINT IF EXISTS accounting_reversals_status_check;
ALTER TABLE app.accounting_reversals
  ADD CONSTRAINT accounting_reversals_status_check CHECK (
    status IN (
      'requested', 'reversed',
      'accounting_adjustment_required', 'accounting_adjustment_pending'
    )
  );

ALTER TABLE app.sales DROP CONSTRAINT IF EXISTS sales_accounting_failure_code_check;
ALTER TABLE app.sales
  ADD CONSTRAINT sales_accounting_failure_code_check
  CHECK (accounting_failure_code IS NULL OR accounting_failure_code = 'PERIOD_LOCKED');
ALTER TABLE app.purchases DROP CONSTRAINT IF EXISTS purchases_accounting_failure_code_check;
ALTER TABLE app.purchases
  ADD CONSTRAINT purchases_accounting_failure_code_check
  CHECK (accounting_failure_code IS NULL OR accounting_failure_code = 'PERIOD_LOCKED');
ALTER TABLE app.payments DROP CONSTRAINT IF EXISTS payments_accounting_failure_code_check;
ALTER TABLE app.payments
  ADD CONSTRAINT payments_accounting_failure_code_check
  CHECK (accounting_failure_code IS NULL OR accounting_failure_code = 'PERIOD_LOCKED');
ALTER TABLE app.accounting_application_commands
  DROP CONSTRAINT IF EXISTS accounting_application_failure_code_check;
ALTER TABLE app.accounting_application_commands
  ADD CONSTRAINT accounting_application_failure_code_check
  CHECK (accounting_failure_code IS NULL OR accounting_failure_code = 'PERIOD_LOCKED');
ALTER TABLE app.accounting_reversals
  DROP CONSTRAINT IF EXISTS accounting_reversal_failure_code_check;
ALTER TABLE app.accounting_reversals
  ADD CONSTRAINT accounting_reversal_failure_code_check
  CHECK (accounting_failure_code IS NULL OR accounting_failure_code = 'PERIOD_LOCKED');

CREATE INDEX IF NOT EXISTS accounting_failures_pending_idx
  ON app.accounting_failures (org_id, updated_at)
  WHERE status <> 'resolved';
CREATE INDEX IF NOT EXISTS accounting_adjustments_failure_idx
  ON app.accounting_adjustments (org_id, failure_id, created_at);

ALTER TABLE app.accounting_failures ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_adjustments ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_failures FORCE ROW LEVEL SECURITY;
ALTER TABLE app.accounting_adjustments FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS accounting_failures_org_isolation ON app.accounting_failures;
CREATE POLICY accounting_failures_org_isolation
  ON app.accounting_failures
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));
DROP POLICY IF EXISTS accounting_adjustments_org_isolation ON app.accounting_adjustments;
CREATE POLICY accounting_adjustments_org_isolation
  ON app.accounting_adjustments
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

COMMIT;
