BEGIN;

ALTER TABLE app.sales
  ADD COLUMN IF NOT EXISTS fiscal_consult_attempts integer NOT NULL DEFAULT 0;

ALTER TABLE app.sales
  DROP CONSTRAINT IF EXISTS sales_fiscal_consult_attempts_non_negative;
ALTER TABLE app.sales
  ADD CONSTRAINT sales_fiscal_consult_attempts_non_negative
    CHECK (fiscal_consult_attempts >= 0);

ALTER TABLE app.accounting_application_commands
  ADD COLUMN IF NOT EXISTS snapshot_digest char(64);

UPDATE app.accounting_application_commands
SET snapshot_digest =
  md5(concat_ws(
    chr(31), id, debit_open_item_id, credit_open_item_id,
    amount::text, currency
  ))
  ||
  md5(concat_ws(
    chr(31), 'pymes-v3', id, debit_open_item_id, credit_open_item_id,
    amount::text, currency
  ))
WHERE snapshot_digest IS NULL;

ALTER TABLE app.accounting_application_commands
  ALTER COLUMN snapshot_digest SET NOT NULL;
ALTER TABLE app.accounting_application_commands
  DROP CONSTRAINT IF EXISTS accounting_application_commands_snapshot_digest_valid;
ALTER TABLE app.accounting_application_commands
  ADD CONSTRAINT accounting_application_commands_snapshot_digest_valid
    CHECK (snapshot_digest ~ '^[0-9a-f]{64}$');

ALTER TABLE app.accounting_reversals
  ADD COLUMN IF NOT EXISTS snapshot_digest char(64);

UPDATE app.accounting_reversals
SET snapshot_digest =
  md5(concat_ws(
    chr(31), id, document_kind, document_id, effective_at::text, reason
  ))
  ||
  md5(concat_ws(
    chr(31), 'pymes-v3', id, document_kind, document_id,
    effective_at::text, reason
  ))
WHERE snapshot_digest IS NULL;

ALTER TABLE app.accounting_reversals
  ALTER COLUMN snapshot_digest SET NOT NULL;
ALTER TABLE app.accounting_reversals
  DROP CONSTRAINT IF EXISTS accounting_reversals_snapshot_digest_valid;
ALTER TABLE app.accounting_reversals
  ADD CONSTRAINT accounting_reversals_snapshot_digest_valid
    CHECK (snapshot_digest ~ '^[0-9a-f]{64}$');

COMMIT;
