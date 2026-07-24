-- Upgrade databases where the first development revision of 000019 was
-- already applied. This is intentionally additive: it preserves every
-- existing draft and posted entry.

ALTER TABLE accounting.journal_entries
    ADD COLUMN IF NOT EXISTS creation_transaction_id xid8 NOT NULL
        DEFAULT pg_current_xact_id();

ALTER TABLE accounting.journal_entries
    ALTER COLUMN creation_transaction_id DROP DEFAULT;

CREATE OR REPLACE FUNCTION accounting.lock_journal_entry_period()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    NEW.creation_transaction_id := pg_current_xact_id();
    PERFORM period.id
      FROM accounting.periods AS period
     WHERE period.org_id = NEW.org_id
       AND period.id = NEW.period_id
     FOR SHARE;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.lock_journal_line_dependencies()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    entry_created_in_transaction boolean;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    SELECT entry.creation_transaction_id = pg_current_xact_id()
      INTO entry_created_in_transaction
      FROM accounting.journal_entries AS entry
     WHERE entry.org_id = NEW.org_id
       AND entry.id = NEW.journal_entry_id;
    IF FOUND AND NOT entry_created_in_transaction THEN
        RAISE EXCEPTION 'posted journal entry lines are immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_journal_lines_posted_entry_immutable';
    END IF;
    PERFORM account.id
      FROM accounting.accounts AS account
     WHERE account.org_id = NEW.org_id
       AND account.id = NEW.account_id
     FOR SHARE;
    PERFORM financial_account.id
      FROM accounting.financial_accounts AS financial_account
     WHERE financial_account.org_id = NEW.org_id
       AND financial_account.ledger_account_id = NEW.account_id
     ORDER BY financial_account.id
     FOR SHARE;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.lock_reconciliation_financial_account()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    IF TG_OP = 'UPDATE'
       AND NEW.financial_account_id IS DISTINCT FROM OLD.financial_account_id THEN
        RAISE EXCEPTION 'reconciliation financial account is immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_reconciliations_financial_account_immutable';
    END IF;
    PERFORM financial_account.id
      FROM accounting.financial_accounts AS financial_account
     WHERE financial_account.org_id = NEW.org_id
       AND financial_account.id = NEW.financial_account_id
     FOR UPDATE;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.reject_financial_account_ledger_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    IF NEW.ledger_account_id IS DISTINCT FROM OLD.ledger_account_id THEN
        RAISE EXCEPTION 'financial account ledger account is immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_financial_accounts_ledger_account_immutable';
    END IF;
    RETURN NEW;
END
$function$;

REVOKE ALL ON FUNCTION accounting.lock_journal_entry_period() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.lock_journal_line_dependencies() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.lock_reconciliation_financial_account() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.reject_financial_account_ledger_change() FROM PUBLIC;

DROP TRIGGER IF EXISTS accounting_journal_entries_dependency_lock ON accounting.journal_entries;
CREATE TRIGGER accounting_journal_entries_dependency_lock
BEFORE INSERT ON accounting.journal_entries
FOR EACH ROW EXECUTE FUNCTION accounting.lock_journal_entry_period();

DROP TRIGGER IF EXISTS accounting_journal_lines_dependency_lock ON accounting.journal_lines;
CREATE TRIGGER accounting_journal_lines_dependency_lock
BEFORE INSERT ON accounting.journal_lines
FOR EACH ROW EXECUTE FUNCTION accounting.lock_journal_line_dependencies();

DROP TRIGGER IF EXISTS accounting_reconciliations_dependency_lock ON accounting.reconciliations;
CREATE TRIGGER accounting_reconciliations_dependency_lock
BEFORE INSERT OR UPDATE ON accounting.reconciliations
FOR EACH ROW EXECUTE FUNCTION accounting.lock_reconciliation_financial_account();

DROP TRIGGER IF EXISTS accounting_financial_accounts_ledger_account_immutable ON accounting.financial_accounts;
CREATE TRIGGER accounting_financial_accounts_ledger_account_immutable
BEFORE UPDATE OF ledger_account_id ON accounting.financial_accounts
FOR EACH ROW EXECUTE FUNCTION accounting.reject_financial_account_ledger_change();

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_fiscal_accounting_worker') THEN
        -- Required by PostgreSQL for SELECT ... FOR SHARE; it does not grant
        -- writes to account identity, lifecycle or financial data.
        GRANT UPDATE (updated_at) ON accounting.accounts, accounting.financial_accounts
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
