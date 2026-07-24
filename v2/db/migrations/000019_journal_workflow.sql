ALTER TABLE accounting.drafts
    DROP CONSTRAINT IF EXISTS drafts_description_check;

ALTER TABLE accounting.drafts
    ADD COLUMN reference text,
    ADD COLUMN currency_code char(3),
    ADD COLUMN exchange_rate numeric(24, 10),
    ADD COLUMN exchange_rate_date date,
    ADD COLUMN exchange_rate_source text,
    ADD COLUMN updated_by text,
    ADD COLUMN discarded_by text,
    ADD COLUMN discard_reason text,
    ADD COLUMN discarded_at timestamptz;

-- Draft currency metadata used to live only on each line. Never choose an
-- arbitrary first line when historical lines disagree: abort the migration
-- with the exact draft identifier so the data can be repaired explicitly.
DO $preflight$
DECLARE
    invalid_org_id uuid;
    invalid_draft_id uuid;
BEGIN
    SELECT draft.org_id, draft.id
      INTO invalid_org_id, invalid_draft_id
      FROM accounting.drafts AS draft
      JOIN accounting.draft_lines AS line
        ON line.org_id = draft.org_id
       AND line.draft_id = draft.id
      LEFT JOIN accounting.organization_settings AS settings
        ON settings.org_id = draft.org_id
     WHERE
        line.exchange_rate_source
            IS DISTINCT FROM btrim(line.exchange_rate_source)
        OR length(line.exchange_rate_source) > 160
        OR line.exchange_rate_date > draft.entry_date
        OR (
            line.currency_code = coalesce(
                settings.functional_currency,
                'ARS'
            )
            AND (
                line.exchange_rate <> 1
                OR line.exchange_rate_date IS NOT NULL
                OR line.exchange_rate_source IS NOT NULL
            )
        )
        OR (
            line.currency_code <> coalesce(
                settings.functional_currency,
                'ARS'
            )
            AND (
                line.exchange_rate_date IS NULL
                OR line.exchange_rate_source IS NULL
                OR round(
                    line.currency_amount * line.exchange_rate,
                    accounting.currency_minor_units(
                        coalesce(settings.functional_currency, 'ARS')
                    )
                ) <> line.debit_amount + line.credit_amount
            )
        )
        OR EXISTS (
            SELECT 1
              FROM accounting.draft_lines AS other
             WHERE other.org_id = line.org_id
               AND other.draft_id = line.draft_id
               AND (
                    other.currency_code,
                    other.exchange_rate,
                    other.exchange_rate_date,
                    other.exchange_rate_source
               ) IS DISTINCT FROM (
                    line.currency_code,
                    line.exchange_rate,
                    line.exchange_rate_date,
                    line.exchange_rate_source
               )
        )
     ORDER BY draft.org_id, draft.id
     LIMIT 1;

    IF invalid_draft_id IS NOT NULL THEN
        RAISE EXCEPTION
            'historical draft % in organization % has inconsistent currency metadata',
            invalid_draft_id,
            invalid_org_id
            USING ERRCODE = '23514',
                  CONSTRAINT =
                      'accounting_draft_lines_historical_currency';
    END IF;
END
$preflight$;

UPDATE accounting.drafts AS draft
   SET currency_code = coalesce(
           (
               SELECT line.currency_code
                 FROM accounting.draft_lines AS line
                WHERE line.org_id = draft.org_id
                  AND line.draft_id = draft.id
                ORDER BY line.line_no
                LIMIT 1
           ),
           (
               SELECT settings.functional_currency
                 FROM accounting.organization_settings AS settings
                WHERE settings.org_id = draft.org_id
           ),
           'ARS'
       ),
       exchange_rate = coalesce(
           (
               SELECT line.exchange_rate
                 FROM accounting.draft_lines AS line
                WHERE line.org_id = draft.org_id
                  AND line.draft_id = draft.id
                ORDER BY line.line_no
                LIMIT 1
           ),
           1
       ),
       exchange_rate_date = (
           SELECT line.exchange_rate_date
             FROM accounting.draft_lines AS line
            WHERE line.org_id = draft.org_id
              AND line.draft_id = draft.id
            ORDER BY line.line_no
            LIMIT 1
       ),
       exchange_rate_source = (
           SELECT btrim(line.exchange_rate_source)
             FROM accounting.draft_lines AS line
            WHERE line.org_id = draft.org_id
              AND line.draft_id = draft.id
            ORDER BY line.line_no
            LIMIT 1
       );

UPDATE accounting.drafts
   SET updated_by = created_by
 WHERE updated_by IS NULL;

UPDATE accounting.drafts
   SET discarded_by = created_by,
       discarded_at = updated_at
 WHERE status = 'discarded'
   AND (discarded_by IS NULL OR discarded_at IS NULL);

ALTER TABLE accounting.drafts
    ALTER COLUMN currency_code SET NOT NULL,
    ALTER COLUMN exchange_rate SET NOT NULL,
    ALTER COLUMN updated_by SET NOT NULL,
    ADD CONSTRAINT accounting_drafts_reference_check CHECK (
        reference IS NULL
        OR (
            btrim(reference) <> ''
            AND reference = btrim(reference)
            AND length(reference) <= 160
        )
    ),
    ADD CONSTRAINT accounting_drafts_currency_code_check
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT accounting_drafts_exchange_rate_check
        CHECK (exchange_rate > 0),
    ADD CONSTRAINT accounting_drafts_exchange_rate_metadata_check CHECK (
        (
            exchange_rate_date IS NULL
            AND exchange_rate_source IS NULL
        )
        OR
        (
            exchange_rate_date IS NOT NULL
            AND exchange_rate_date <= entry_date
            AND exchange_rate_source IS NOT NULL
            AND btrim(exchange_rate_source) <> ''
            AND exchange_rate_source = btrim(exchange_rate_source)
            AND length(exchange_rate_source) <= 160
        )
    ),
    ADD CONSTRAINT accounting_drafts_updated_by_check
        CHECK (btrim(updated_by) <> ''),
    ADD CONSTRAINT accounting_drafts_discard_audit_check CHECK (
        (
            status = 'discarded'
            AND discarded_by IS NOT NULL
            AND btrim(discarded_by) <> ''
            AND discarded_at IS NOT NULL
        )
        OR
        (
            status <> 'discarded'
            AND discarded_by IS NULL
            AND discard_reason IS NULL
            AND discarded_at IS NULL
        )
    ),
    ADD CONSTRAINT accounting_drafts_discard_reason_check CHECK (
        discard_reason IS NULL
        OR (
            discard_reason = btrim(discard_reason)
            AND btrim(discard_reason) <> ''
            AND length(discard_reason) <= 500
        )
    );

ALTER TABLE accounting.journal_entries
    ADD COLUMN reference text,
    ADD COLUMN creation_transaction_id xid8 NOT NULL
        DEFAULT pg_current_xact_id(),
    ADD CONSTRAINT accounting_journal_entries_reference_check CHECK (
        reference IS NULL
        OR (
            btrim(reference) <> ''
            AND reference = btrim(reference)
            AND length(reference) <= 160
        )
    );

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
    -- PostgreSQL reports the top-level xid8 consistently inside savepoints.
    -- Callers cannot forge or reuse this internal construction marker.
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
        RAISE EXCEPTION
            'posted journal entry lines are immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT =
                      'accounting_journal_lines_posted_entry_immutable';
    END IF;

    -- SHARE permits concurrent postings while serializing lifecycle changes
    -- such as archive or disabling posting on the referenced account.
    PERFORM account.id
      FROM accounting.accounts AS account
     WHERE account.org_id = NEW.org_id
       AND account.id = NEW.account_id
     FOR SHARE;

    -- Reconciliation creation and transitions take UPDATE on the same
    -- financial-account rows. Ordering prevents multi-resource deadlocks.
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
       AND NEW.financial_account_id IS DISTINCT FROM OLD.financial_account_id
    THEN
        RAISE EXCEPTION
            'reconciliation financial account is immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT =
                      'accounting_reconciliations_financial_account_immutable';
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
        RAISE EXCEPTION
            'financial account ledger account is immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT =
                      'accounting_financial_accounts_ledger_account_immutable';
    END IF;
    RETURN NEW;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.lock_journal_entry_period()
FROM PUBLIC;

REVOKE ALL
ON FUNCTION accounting.lock_journal_line_dependencies()
FROM PUBLIC;

REVOKE ALL
ON FUNCTION accounting.lock_reconciliation_financial_account()
FROM PUBLIC;

REVOKE ALL
ON FUNCTION accounting.reject_financial_account_ledger_change()
FROM PUBLIC;

CREATE TRIGGER accounting_journal_entries_dependency_lock
BEFORE INSERT
ON accounting.journal_entries
FOR EACH ROW
EXECUTE FUNCTION accounting.lock_journal_entry_period();

CREATE TRIGGER accounting_journal_lines_dependency_lock
BEFORE INSERT
ON accounting.journal_lines
FOR EACH ROW
EXECUTE FUNCTION accounting.lock_journal_line_dependencies();

CREATE TRIGGER accounting_reconciliations_dependency_lock
BEFORE INSERT OR UPDATE
ON accounting.reconciliations
FOR EACH ROW
EXECUTE FUNCTION accounting.lock_reconciliation_financial_account();

CREATE TRIGGER accounting_financial_accounts_ledger_account_immutable
BEFORE UPDATE OF ledger_account_id
ON accounting.financial_accounts
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_financial_account_ledger_change();

CREATE OR REPLACE FUNCTION accounting.validate_draft_currency_consistency()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    target_org_id uuid;
    target_draft_id uuid;
    draft_record accounting.drafts%ROWTYPE;
    functional_currency char(3);
BEGIN
    target_org_id := NEW.org_id;
    IF TG_TABLE_NAME = 'drafts' THEN
        target_draft_id := NEW.id;
    ELSE
        target_draft_id := NEW.draft_id;
    END IF;
    PERFORM app.assert_org_context(target_org_id);

    SELECT *
      INTO draft_record
      FROM accounting.drafts
     WHERE org_id = target_org_id
       AND id = target_draft_id;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT coalesce(settings.functional_currency, 'ARS')
      INTO functional_currency
      FROM (SELECT 1) AS singleton
      LEFT JOIN accounting.organization_settings AS settings
        ON settings.org_id = target_org_id;

    IF (
        draft_record.currency_code = functional_currency
        AND (
            draft_record.exchange_rate <> 1
            OR draft_record.exchange_rate_date IS NOT NULL
            OR draft_record.exchange_rate_source IS NOT NULL
        )
    ) OR (
        draft_record.currency_code <> functional_currency
        AND (
            draft_record.exchange_rate_date IS NULL
            OR draft_record.exchange_rate_source IS NULL
        )
    ) THEN
        RAISE EXCEPTION 'draft currency header is inconsistent with functional currency'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_drafts_functional_currency';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM accounting.draft_lines AS line
         WHERE line.org_id = target_org_id
           AND line.draft_id = target_draft_id
           AND (
               line.currency_code <> draft_record.currency_code
               OR line.exchange_rate <> draft_record.exchange_rate
               OR line.exchange_rate_date
                    IS DISTINCT FROM draft_record.exchange_rate_date
               OR line.exchange_rate_source
                    IS DISTINCT FROM draft_record.exchange_rate_source
               OR round(
                    line.currency_amount * draft_record.exchange_rate,
                    accounting.currency_minor_units(functional_currency)
               ) <> line.debit_amount + line.credit_amount
           )
    ) THEN
        RAISE EXCEPTION 'draft lines must use the currency metadata from the draft header'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_draft_lines_header_currency';
    END IF;
    RETURN NULL;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.validate_draft_currency_consistency()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER accounting_drafts_currency_consistency
AFTER INSERT OR UPDATE
ON accounting.drafts
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_draft_currency_consistency();

CREATE CONSTRAINT TRIGGER accounting_draft_lines_currency_consistency
AFTER INSERT OR UPDATE
ON accounting.draft_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_draft_currency_consistency();

CREATE OR REPLACE FUNCTION accounting.validate_journal_workflow_invariants()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    target_org_id uuid;
    target_entry_id uuid;
    entry_record accounting.journal_entries%ROWTYPE;
    functional_currency char(3);
    original_date date;
BEGIN
    target_org_id := NEW.org_id;
    IF TG_TABLE_NAME = 'journal_entries' THEN
        target_entry_id := NEW.id;
    ELSE
        target_entry_id := NEW.journal_entry_id;
    END IF;
    PERFORM app.assert_org_context(target_org_id);

    SELECT *
      INTO entry_record
      FROM accounting.journal_entries
     WHERE org_id = target_org_id
       AND id = target_entry_id;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT coalesce(settings.functional_currency, 'ARS')
      INTO functional_currency
      FROM (SELECT 1) AS singleton
      LEFT JOIN accounting.organization_settings AS settings
        ON settings.org_id = target_org_id;

    IF entry_record.functional_currency <> functional_currency THEN
        RAISE EXCEPTION
            'journal entry functional currency must match organization settings'
            USING ERRCODE = '23514',
                  CONSTRAINT =
                      'accounting_journal_entries_functional_currency';
    END IF;

    IF entry_record.reverses_entry_id IS NOT NULL THEN
        SELECT original.entry_date
          INTO original_date
          FROM accounting.journal_entries AS original
         WHERE original.org_id = target_org_id
           AND original.id = entry_record.reverses_entry_id;

        IF original_date IS NOT NULL
           AND entry_record.entry_date < original_date THEN
            RAISE EXCEPTION
                'reversal date cannot precede the original entry date'
                USING ERRCODE = '23514',
                      CONSTRAINT =
                          'accounting_journal_entries_reversal_date';
        END IF;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM accounting.journal_lines AS line
         WHERE line.org_id = target_org_id
           AND line.journal_entry_id = target_entry_id
           AND line.exchange_rate_date > entry_record.entry_date
    ) THEN
        RAISE EXCEPTION
            'journal line exchange-rate date cannot follow the entry date'
            USING ERRCODE = '23514',
                  CONSTRAINT =
                      'accounting_journal_lines_exchange_rate_date';
    END IF;

    RETURN NULL;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.validate_journal_workflow_invariants()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER accounting_journal_entries_workflow_invariants
AFTER INSERT OR UPDATE
ON accounting.journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_journal_workflow_invariants();

CREATE CONSTRAINT TRIGGER accounting_journal_lines_workflow_invariants
AFTER INSERT OR UPDATE
ON accounting.journal_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_journal_workflow_invariants();

CREATE OR REPLACE FUNCTION accounting.reject_closed_reconciliation_posting()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    entry_date date;
    closed_reconciliation_id uuid;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);

    SELECT entry.entry_date
      INTO entry_date
      FROM accounting.journal_entries AS entry
     WHERE entry.org_id = NEW.org_id
       AND entry.id = NEW.journal_entry_id;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT reconciliation.id
      INTO closed_reconciliation_id
      FROM accounting.reconciliations AS reconciliation
      JOIN accounting.financial_accounts AS financial_account
        ON financial_account.org_id = reconciliation.org_id
       AND financial_account.id = reconciliation.financial_account_id
     WHERE reconciliation.org_id = NEW.org_id
       AND reconciliation.status = 'closed'
       AND entry_date BETWEEN reconciliation.start_date
                          AND reconciliation.end_date
       AND financial_account.ledger_account_id = NEW.account_id
     ORDER BY reconciliation.end_date DESC, reconciliation.id
     LIMIT 1;

    IF closed_reconciliation_id IS NOT NULL THEN
        RAISE EXCEPTION 'posting would change a closed reconciliation'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_lines_closed_reconciliation';
    END IF;
    RETURN NULL;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.reject_closed_reconciliation_posting()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER accounting_journal_lines_closed_reconciliation
AFTER INSERT OR UPDATE
ON accounting.journal_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_closed_reconciliation_posting();

CREATE INDEX accounting_drafts_list_idx
    ON accounting.drafts (org_id, status, updated_at DESC, id DESC);

CREATE INDEX accounting_drafts_date_idx
    ON accounting.drafts (org_id, status, entry_date DESC, id DESC);

CREATE INDEX accounting_drafts_reference_idx
    ON accounting.drafts (
        org_id,
        lower(reference) text_pattern_ops
    )
    WHERE reference IS NOT NULL;

CREATE INDEX accounting_journal_entries_source_idx
    ON accounting.journal_entries (
        org_id,
        source_type,
        entry_number DESC
    );

CREATE INDEX accounting_journal_entries_reference_idx
    ON accounting.journal_entries (
        org_id,
        lower(reference) text_pattern_ops
    )
    WHERE reference IS NOT NULL;

CREATE OR REPLACE FUNCTION accounting.journal_snapshot_hash(
    p_org_id uuid,
    p_subject_type text,
    p_draft_id uuid,
    p_journal_entry_id uuid,
    p_action text,
    p_actor text,
    p_version bigint,
    p_before_snapshot jsonb,
    p_after_snapshot jsonb
)
RETURNS text
LANGUAGE sql
IMMUTABLE
SET search_path = pg_catalog, accounting
RETURN encode(
    public.digest(
        convert_to(
            jsonb_build_object(
                'org_id', p_org_id,
                'subject_type', p_subject_type,
                'draft_id', p_draft_id,
                'journal_entry_id', p_journal_entry_id,
                'action', p_action,
                'actor', p_actor,
                'version', p_version,
                'before_snapshot', p_before_snapshot,
                'after_snapshot', p_after_snapshot
            )::text,
            'UTF8'
        ),
        'sha256'
    ),
    'hex'
);

REVOKE ALL
ON FUNCTION accounting.journal_snapshot_hash(
    uuid,
    text,
    uuid,
    uuid,
    text,
    text,
    bigint,
    jsonb,
    jsonb
)
FROM PUBLIC;

CREATE TABLE accounting.journal_events (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    subject_type text NOT NULL CHECK (
        subject_type IN ('draft', 'journal_entry')
    ),
    draft_id uuid,
    journal_entry_id uuid,
    action text NOT NULL CHECK (
        action IN ('create', 'update', 'discard', 'post', 'reverse')
    ),
    actor text NOT NULL CHECK (btrim(actor) <> ''),
    version bigint NOT NULL CHECK (version > 0),
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (
        jsonb_typeof(details) = 'object'
    ),
    before_snapshot jsonb CHECK (
        before_snapshot IS NULL
        OR jsonb_typeof(before_snapshot) = 'object'
    ),
    after_snapshot jsonb NOT NULL CHECK (
        jsonb_typeof(after_snapshot) = 'object'
    ),
    snapshot_hash text NOT NULL CHECK (
        snapshot_hash ~ '^[0-9a-f]{64}$'
    ),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_journal_events_draft_fk
        FOREIGN KEY (org_id, draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_journal_events_entry_fk
        FOREIGN KEY (org_id, journal_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        (
            subject_type = 'draft'
            AND draft_id IS NOT NULL
            AND journal_entry_id IS NULL
            AND action IN ('create', 'update', 'discard')
        )
        OR
        (
            subject_type = 'journal_entry'
            AND draft_id IS NULL
            AND journal_entry_id IS NOT NULL
            AND action IN ('post', 'reverse')
        )
    ),
    CHECK (
        (
            action IN ('create', 'post', 'reverse')
            AND before_snapshot IS NULL
        )
        OR
        (
            action IN ('update', 'discard')
            AND before_snapshot IS NOT NULL
        )
    ),
    CHECK (
        snapshot_hash = accounting.journal_snapshot_hash(
            org_id,
            subject_type,
            draft_id,
            journal_entry_id,
            action,
            actor,
            version,
            before_snapshot,
            after_snapshot
        )
    )
);

CREATE INDEX accounting_journal_events_subject_idx
    ON accounting.journal_events (
        org_id,
        subject_type,
        draft_id,
        journal_entry_id,
        occurred_at DESC
    );

CREATE OR REPLACE FUNCTION accounting.audit_draft_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    event_action text;
    event_actor text;
    before_state jsonb;
    after_state jsonb;
    line_state jsonb;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);

    IF TG_OP = 'INSERT' THEN
        event_action := 'create';
    ELSIF OLD.status = 'active' AND NEW.status = 'discarded' THEN
        event_action := 'discard';
    ELSIF OLD.status = 'active' AND NEW.status = 'posted' THEN
        -- The immutable journal entry records the authoritative post event.
        RETURN NULL;
    ELSE
        event_action := 'update';
    END IF;

    event_actor := CASE
        WHEN event_action = 'discard' THEN NEW.discarded_by
        ELSE NEW.updated_by
    END;
    before_state := NULL;
    IF TG_OP <> 'INSERT' THEN
        SELECT previous.after_snapshot
          INTO before_state
          FROM accounting.journal_events AS previous
         WHERE previous.org_id = NEW.org_id
           AND previous.draft_id = NEW.id
           AND previous.version < NEW.version
         ORDER BY
            previous.version DESC,
            previous.occurred_at DESC,
            previous.id DESC
         LIMIT 1;

        -- Drafts that predate this migration have no creation event. Their
        -- first edit still receives a useful header-level before snapshot.
        IF before_state IS NULL THEN
            before_state := jsonb_build_object(
                'header',
                to_jsonb(OLD) - ARRAY['org_id', 'idempotency_key'],
                'lines',
                '[]'::jsonb
            );
        END IF;
    END IF;

    SELECT coalesce(
        jsonb_agg(
            jsonb_build_object(
                'id', line.id,
                'line_no', line.line_no,
                'account_id', line.account_id,
                'account_code', account.code,
                'account_name', account.name,
                'description', line.description,
                'debit_amount', line.debit_amount::text,
                'credit_amount', line.credit_amount::text,
                'currency_code', line.currency_code,
                'currency_amount', line.currency_amount::text,
                'exchange_rate', line.exchange_rate::text,
                'exchange_rate_date', line.exchange_rate_date,
                'exchange_rate_source', line.exchange_rate_source,
                'party_type', line.party_type,
                'party_id', line.party_id,
                'tax_code', line.tax_code,
                'origin_date', line.origin_date
            )
            ORDER BY line.line_no, line.id
        ),
        '[]'::jsonb
    )
      INTO line_state
      FROM accounting.draft_lines AS line
      JOIN accounting.accounts AS account
        ON account.org_id = line.org_id
       AND account.id = line.account_id
     WHERE line.org_id = NEW.org_id
       AND line.draft_id = NEW.id;

    after_state := jsonb_build_object(
        'header',
        to_jsonb(NEW) - ARRAY['org_id', 'idempotency_key'],
        'lines',
        line_state
    );

    INSERT INTO accounting.journal_events (
        org_id,
        subject_type,
        draft_id,
        action,
        actor,
        version,
        details,
        before_snapshot,
        after_snapshot,
        snapshot_hash,
        occurred_at
    )
    VALUES (
        NEW.org_id,
        'draft',
        NEW.id,
        event_action,
        event_actor,
        NEW.version,
        jsonb_build_object(
            'status', NEW.status,
            'reference', NEW.reference,
            'currency_code', NEW.currency_code,
            'exchange_rate', NEW.exchange_rate,
            'discard_reason', NEW.discard_reason,
            'posted_entry_id', NEW.posted_entry_id
        ),
        before_state,
        after_state,
        accounting.journal_snapshot_hash(
            NEW.org_id,
            'draft',
            NEW.id,
            NULL,
            event_action,
            event_actor,
            NEW.version,
            before_state,
            after_state
        ),
        coalesce(NEW.discarded_at, now())
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.audit_journal_entry_insert()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    event_action text;
    after_state jsonb;
    line_state jsonb;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);

    event_action := CASE
        WHEN NEW.reverses_entry_id IS NULL THEN 'post'
        ELSE 'reverse'
    END;

    SELECT coalesce(
        jsonb_agg(
            jsonb_build_object(
                'id', line.id,
                'line_no', line.line_no,
                'account_id', line.account_id,
                'account_code', account.code,
                'account_name', account.name,
                'description', line.description,
                'debit_amount', line.debit_amount::text,
                'credit_amount', line.credit_amount::text,
                'currency_code', line.currency_code,
                'currency_amount', line.currency_amount::text,
                'exchange_rate', line.exchange_rate::text,
                'exchange_rate_date', line.exchange_rate_date,
                'exchange_rate_source', line.exchange_rate_source,
                'party_type', line.party_type,
                'party_id', line.party_id,
                'tax_code', line.tax_code,
                'origin_date', line.origin_date
            )
            ORDER BY line.line_no, line.id
        ),
        '[]'::jsonb
    )
      INTO line_state
      FROM accounting.journal_lines AS line
      JOIN accounting.accounts AS account
        ON account.org_id = line.org_id
       AND account.id = line.account_id
     WHERE line.org_id = NEW.org_id
       AND line.journal_entry_id = NEW.id;

    after_state := jsonb_build_object(
        'header',
        to_jsonb(NEW) - ARRAY[
            'org_id',
            'idempotency_key',
            'creation_transaction_id'
        ],
        'lines',
        line_state
    );

    INSERT INTO accounting.journal_events (
        org_id,
        subject_type,
        journal_entry_id,
        action,
        actor,
        version,
        details,
        before_snapshot,
        after_snapshot,
        snapshot_hash
    )
    VALUES (
        NEW.org_id,
        'journal_entry',
        NEW.id,
        event_action,
        NEW.created_by,
        1,
        jsonb_build_object(
            'entry_number', NEW.entry_number,
            'reference', NEW.reference,
            'draft_id', NEW.draft_id,
            'reverses_entry_id', NEW.reverses_entry_id,
            'reversal_reason', NEW.reversal_reason
        ),
        NULL,
        after_state,
        accounting.journal_snapshot_hash(
            NEW.org_id,
            'journal_entry',
            NULL,
            NEW.id,
            event_action,
            NEW.created_by,
            1,
            NULL,
            after_state
        )
    );
    RETURN NULL;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.audit_draft_change()
FROM PUBLIC;

REVOKE ALL
ON FUNCTION accounting.audit_journal_entry_insert()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER accounting_drafts_audit
AFTER INSERT OR UPDATE
ON accounting.drafts
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.audit_draft_change();

CREATE CONSTRAINT TRIGGER accounting_journal_entries_audit
AFTER INSERT
ON accounting.journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.audit_journal_entry_insert();

CREATE TRIGGER accounting_journal_events_immutable
BEFORE UPDATE OR DELETE
ON accounting.journal_events
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

ALTER TABLE accounting.journal_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting.journal_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation
ON accounting.journal_events
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
);

REVOKE ALL ON TABLE accounting.journal_events FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT ON accounting.journal_events TO pymes_backend;
        GRANT EXECUTE
        ON FUNCTION accounting.journal_snapshot_hash(
            uuid,
            text,
            uuid,
            uuid,
            text,
            text,
            bigint,
            jsonb,
            jsonb
        )
        TO pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_accounting_worker'
    ) THEN
        GRANT SELECT ON
            accounting.journal_events,
            accounting.financial_accounts,
            accounting.reconciliations
        TO pymes_fiscal_accounting_worker;
        -- PostgreSQL requires UPDATE privilege for SELECT ... FOR SHARE.
        -- Posting only locks these dependencies; limiting the grant to the
        -- non-business timestamp keeps account identity and lifecycle read-only.
        GRANT UPDATE (updated_at) ON
            accounting.accounts,
            accounting.financial_accounts
        TO pymes_fiscal_accounting_worker;
        GRANT EXECUTE
        ON FUNCTION accounting.journal_snapshot_hash(
            uuid,
            text,
            uuid,
            uuid,
            text,
            text,
            bigint,
            jsonb,
            jsonb
        )
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
