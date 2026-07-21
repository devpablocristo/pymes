-- 0022_ledger.down.sql — reverso completo de 0022_ledger.up.sql

ALTER TABLE returns   DROP COLUMN IF EXISTS posting_status;
ALTER TABLE payments  DROP COLUMN IF EXISTS posting_status;
ALTER TABLE purchases DROP COLUMN IF EXISTS posting_status;
ALTER TABLE sales     DROP COLUMN IF EXISTS posting_status;

ALTER TABLE org_settings DROP COLUMN IF EXISTS journal_prefix;
ALTER TABLE org_settings DROP COLUMN IF EXISTS ledger_enabled;

DROP TABLE IF EXISTS ledger_outbox;
DROP TABLE IF EXISTS journal_lines;
DROP TABLE IF EXISTS journal_entries;
DROP TABLE IF EXISTS ledger_sequences;
DROP TABLE IF EXISTS ledger_account_links;
DROP TABLE IF EXISTS ledger_accounts;
