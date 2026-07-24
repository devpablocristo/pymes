CREATE TABLE accounting.exchange_rates (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    rate_date date NOT NULL,
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    functional_currency char(3) NOT NULL
        CHECK (functional_currency ~ '^[A-Z]{3}$'),
    rate numeric(24, 10) NOT NULL CHECK (rate > 0),
    source text NOT NULL CHECK (btrim(source) <> ''),
    source_reference text,
    source_checksum char(64)
        CHECK (
            source_checksum IS NULL
            OR source_checksum ~ '^[0-9a-f]{64}$'
        ),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_exchange_rates_source_unique
        UNIQUE (
            org_id,
            rate_date,
            currency_code,
            functional_currency,
            source
        ),
    CHECK (currency_code <> functional_currency),
    CHECK (
        source_reference IS NULL
        OR btrim(source_reference) <> ''
    )
);

CREATE INDEX accounting_exchange_rates_lookup_idx
    ON accounting.exchange_rates (
        org_id,
        currency_code,
        functional_currency,
        rate_date DESC
    );

CREATE TABLE accounting.open_items (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    item_type text NOT NULL
        CHECK (item_type IN ('receivable', 'payable')),
    party_type text NOT NULL CHECK (btrim(party_type) <> ''),
    party_id text NOT NULL CHECK (btrim(party_id) <> ''),
    account_id uuid NOT NULL,
    origin_journal_entry_id uuid NOT NULL,
    origin_journal_line_id uuid NOT NULL,
    document_type text NOT NULL CHECK (btrim(document_type) <> ''),
    document_id text NOT NULL CHECK (btrim(document_id) <> ''),
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    original_currency_amount numeric(24, 6) NOT NULL
        CHECK (original_currency_amount > 0),
    original_functional_amount numeric(24, 6) NOT NULL
        CHECK (original_functional_amount > 0),
    issued_at date NOT NULL,
    due_date date,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_open_items_origin_unique
        UNIQUE (org_id, origin_journal_entry_id, origin_journal_line_id),
    CONSTRAINT accounting_open_items_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_open_items_origin_line_fk
        FOREIGN KEY (
            org_id,
            origin_journal_entry_id,
            origin_journal_line_id
        )
        REFERENCES accounting.journal_lines(
            org_id,
            journal_entry_id,
            id
        )
        ON DELETE RESTRICT,
    CHECK (due_date IS NULL OR due_date >= issued_at)
);

CREATE INDEX accounting_open_items_party_idx
    ON accounting.open_items (
        org_id,
        item_type,
        party_type,
        party_id,
        due_date
    );

CREATE TABLE accounting.open_item_applications (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    open_item_id uuid NOT NULL,
    settlement_journal_entry_id uuid NOT NULL,
    settlement_journal_line_id uuid NOT NULL,
    currency_amount numeric(24, 6) NOT NULL CHECK (currency_amount > 0),
    functional_amount numeric(24, 6) NOT NULL CHECK (functional_amount > 0),
    exchange_difference_amount numeric(24, 6) NOT NULL DEFAULT 0,
    reverses_application_id uuid,
    applied_by text NOT NULL CHECK (btrim(applied_by) <> ''),
    applied_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_open_item_applications_item_fk
        FOREIGN KEY (org_id, open_item_id)
        REFERENCES accounting.open_items(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_open_item_applications_settlement_line_fk
        FOREIGN KEY (
            org_id,
            settlement_journal_entry_id,
            settlement_journal_line_id
        )
        REFERENCES accounting.journal_lines(
            org_id,
            journal_entry_id,
            id
        )
        ON DELETE RESTRICT,
    CONSTRAINT accounting_open_item_applications_reversal_fk
        FOREIGN KEY (org_id, reverses_application_id)
        REFERENCES accounting.open_item_applications(org_id, id)
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (reverses_application_id IS NULL OR reverses_application_id <> id)
);

CREATE UNIQUE INDEX accounting_open_item_applications_direct_reversal_uidx
    ON accounting.open_item_applications (org_id, reverses_application_id)
    WHERE reverses_application_id IS NOT NULL;

CREATE INDEX accounting_open_item_applications_item_idx
    ON accounting.open_item_applications (org_id, open_item_id, applied_at);

CREATE TABLE accounting.financial_accounts (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    ledger_account_id uuid NOT NULL,
    account_type text NOT NULL
        CHECK (account_type IN ('cash', 'bank', 'card', 'wallet')),
    name text NOT NULL CHECK (btrim(name) <> ''),
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    institution_name text,
    external_reference text,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_financial_accounts_ledger_unique
        UNIQUE (org_id, ledger_account_id),
    CONSTRAINT accounting_financial_accounts_ledger_fk
        FOREIGN KEY (org_id, ledger_account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT,
    CHECK (institution_name IS NULL OR btrim(institution_name) <> ''),
    CHECK (external_reference IS NULL OR btrim(external_reference) <> '')
);

CREATE TABLE accounting.statement_imports (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    financial_account_id uuid NOT NULL,
    file_name text NOT NULL CHECK (btrim(file_name) <> ''),
    file_format text NOT NULL CHECK (file_format IN ('csv', 'xlsx', 'ofx')),
    file_sha256 char(64) NOT NULL
        CHECK (file_sha256 ~ '^[0-9a-f]{64}$'),
    imported_by text NOT NULL CHECK (btrim(imported_by) <> ''),
    imported_at timestamptz NOT NULL DEFAULT now(),
    row_count integer NOT NULL DEFAULT 0 CHECK (row_count >= 0),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_statement_imports_file_unique
        UNIQUE (org_id, financial_account_id, file_sha256),
    CONSTRAINT accounting_statement_imports_account_fk
        FOREIGN KEY (org_id, financial_account_id)
        REFERENCES accounting.financial_accounts(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE accounting.statement_transactions (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    financial_account_id uuid NOT NULL,
    statement_import_id uuid NOT NULL,
    external_id text,
    fingerprint_sha256 char(64) NOT NULL
        CHECK (fingerprint_sha256 ~ '^[0-9a-f]{64}$'),
    booked_at date NOT NULL,
    value_date date,
    amount numeric(24, 6) NOT NULL CHECK (amount <> 0),
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    reference text,
    description text NOT NULL DEFAULT '',
    raw_data jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_statement_transactions_fingerprint_unique
        UNIQUE (org_id, financial_account_id, fingerprint_sha256),
    CONSTRAINT accounting_statement_transactions_account_fk
        FOREIGN KEY (org_id, financial_account_id)
        REFERENCES accounting.financial_accounts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_statement_transactions_import_fk
        FOREIGN KEY (org_id, statement_import_id)
        REFERENCES accounting.statement_imports(org_id, id)
        ON DELETE RESTRICT,
    CHECK (external_id IS NULL OR btrim(external_id) <> ''),
    CHECK (reference IS NULL OR btrim(reference) <> '')
);

CREATE INDEX accounting_statement_transactions_date_idx
    ON accounting.statement_transactions (
        org_id,
        financial_account_id,
        booked_at,
        id
    );

CREATE TABLE accounting.reconciliations (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    financial_account_id uuid NOT NULL,
    start_date date NOT NULL,
    end_date date NOT NULL,
    opening_balance numeric(24, 6) NOT NULL,
    closing_balance numeric(24, 6) NOT NULL,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'closed')),
    adjustment_draft_id uuid,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    status_changed_by text,
    transition_reason text,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_reconciliations_period_unique
        UNIQUE (org_id, financial_account_id, start_date, end_date),
    CONSTRAINT accounting_reconciliations_account_fk
        FOREIGN KEY (org_id, financial_account_id)
        REFERENCES accounting.financial_accounts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_reconciliations_adjustment_draft_fk
        FOREIGN KEY (org_id, adjustment_draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE RESTRICT,
    CHECK (end_date >= start_date),
    CHECK (
        status_changed_by IS NULL
        OR btrim(status_changed_by) <> ''
    ),
    CHECK (
        transition_reason IS NULL
        OR btrim(transition_reason) <> ''
    )
);

CREATE TABLE accounting.reconciliation_matches (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    reconciliation_id uuid NOT NULL,
    statement_transaction_id uuid NOT NULL,
    journal_entry_id uuid NOT NULL,
    journal_line_id uuid NOT NULL,
    matched_amount numeric(24, 6) NOT NULL CHECK (matched_amount > 0),
    functional_amount numeric(24, 6) NOT NULL CHECK (functional_amount > 0),
    match_source text NOT NULL CHECK (match_source IN ('suggested', 'manual')),
    matched_by text NOT NULL CHECK (btrim(matched_by) <> ''),
    matched_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_reconciliation_matches_identity_unique
        UNIQUE (
            org_id,
            reconciliation_id,
            statement_transaction_id,
            journal_line_id
        ),
    CONSTRAINT accounting_reconciliation_matches_reconciliation_fk
        FOREIGN KEY (org_id, reconciliation_id)
        REFERENCES accounting.reconciliations(org_id, id)
        ON DELETE CASCADE,
    CONSTRAINT accounting_reconciliation_matches_transaction_fk
        FOREIGN KEY (org_id, statement_transaction_id)
        REFERENCES accounting.statement_transactions(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_reconciliation_matches_line_fk
        FOREIGN KEY (org_id, journal_entry_id, journal_line_id)
        REFERENCES accounting.journal_lines(
            org_id,
            journal_entry_id,
            id
        )
        ON DELETE RESTRICT
);

CREATE INDEX accounting_reconciliation_matches_transaction_idx
    ON accounting.reconciliation_matches (
        org_id,
        statement_transaction_id
    );

CREATE INDEX accounting_reconciliation_matches_line_idx
    ON accounting.reconciliation_matches (org_id, journal_line_id);

CREATE TABLE accounting.reconciliation_events (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    reconciliation_id uuid NOT NULL,
    from_status text NOT NULL CHECK (from_status IN ('draft', 'closed')),
    to_status text NOT NULL CHECK (to_status IN ('draft', 'closed')),
    actor text NOT NULL CHECK (btrim(actor) <> ''),
    reason text,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_reconciliation_events_reconciliation_fk
        FOREIGN KEY (org_id, reconciliation_id)
        REFERENCES accounting.reconciliations(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE accounting.period_close_checks (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    period_id uuid NOT NULL,
    check_key text NOT NULL CHECK (
        check_key IN (
            'unposted_documents',
            'fiscal_pending',
            'posting_errors',
            'account_mappings',
            'exchange_rates',
            'unreconciled_accounts'
        )
    ),
    status text NOT NULL CHECK (status IN ('passed', 'warning', 'blocked')),
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    checked_by text NOT NULL CHECK (btrim(checked_by) <> ''),
    checked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_period_close_checks_key_unique
        UNIQUE (org_id, period_id, check_key),
    CONSTRAINT accounting_period_close_checks_period_fk
        FOREIGN KEY (org_id, period_id)
        REFERENCES accounting.periods(org_id, id)
        ON DELETE CASCADE
);

CREATE TABLE accounting.inflation_indices (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    series_code text NOT NULL CHECK (btrim(series_code) <> ''),
    period_month date NOT NULL CHECK (
        period_month = date_trunc('month', period_month)::date
    ),
    index_value numeric(24, 10) NOT NULL CHECK (index_value > 0),
    source_url text NOT NULL CHECK (btrim(source_url) <> ''),
    source_checksum char(64) NOT NULL
        CHECK (source_checksum ~ '^[0-9a-f]{64}$'),
    imported_by text NOT NULL CHECK (btrim(imported_by) <> ''),
    imported_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_inflation_indices_source_unique
        UNIQUE (org_id, series_code, period_month, source_checksum)
);

CREATE INDEX accounting_inflation_indices_lookup_idx
    ON accounting.inflation_indices (
        org_id,
        series_code,
        period_month DESC
    );

CREATE TABLE accounting.inflation_runs (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    period_id uuid NOT NULL,
    series_code text NOT NULL CHECK (btrim(series_code) <> ''),
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'ready', 'posted')),
    generated_draft_id uuid,
    source_checksum char(64) NOT NULL
        CHECK (source_checksum ~ '^[0-9a-f]{64}$'),
    workpaper_sha256 char(64)
        CHECK (
            workpaper_sha256 IS NULL
            OR workpaper_sha256 ~ '^[0-9a-f]{64}$'
        ),
    recpam_amount numeric(24, 6) NOT NULL DEFAULT 0,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    reviewed_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_inflation_runs_period_unique
        UNIQUE (org_id, period_id, series_code, source_checksum),
    CONSTRAINT accounting_inflation_runs_period_fk
        FOREIGN KEY (org_id, period_id)
        REFERENCES accounting.periods(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_inflation_runs_draft_fk
        FOREIGN KEY (org_id, generated_draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE RESTRICT,
    CHECK (reviewed_by IS NULL OR btrim(reviewed_by) <> ''),
    CHECK (
        (status = 'draft' AND reviewed_by IS NULL)
        OR
        (
            status IN ('ready', 'posted')
            AND reviewed_by IS NOT NULL
            AND generated_draft_id IS NOT NULL
            AND workpaper_sha256 IS NOT NULL
        )
    )
);

CREATE TABLE accounting.inflation_run_lines (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    inflation_run_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    account_id uuid NOT NULL,
    origin_date date NOT NULL,
    monetary_class text NOT NULL
        CHECK (monetary_class IN ('monetary', 'non_monetary')),
    original_amount numeric(24, 6) NOT NULL,
    origin_index numeric(24, 10) NOT NULL CHECK (origin_index > 0),
    closing_index numeric(24, 10) NOT NULL CHECK (closing_index > 0),
    coefficient numeric(24, 10) NOT NULL CHECK (coefficient > 0),
    adjusted_amount numeric(24, 6) NOT NULL,
    adjustment_amount numeric(24, 6) NOT NULL,
    recpam_amount numeric(24, 6) NOT NULL DEFAULT 0,
    calculation_details jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_inflation_run_lines_number_unique
        UNIQUE (org_id, inflation_run_id, line_no),
    CONSTRAINT accounting_inflation_run_lines_run_fk
        FOREIGN KEY (org_id, inflation_run_id)
        REFERENCES accounting.inflation_runs(org_id, id)
        ON DELETE CASCADE,
    CONSTRAINT accounting_inflation_run_lines_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE accounting.currency_revaluation_runs (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    period_id uuid NOT NULL,
    revaluation_date date NOT NULL,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'ready', 'posted')),
    generated_draft_id uuid,
    source_checksum char(64) NOT NULL
        CHECK (source_checksum ~ '^[0-9a-f]{64}$'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    reviewed_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_currency_revaluation_runs_unique
        UNIQUE (org_id, period_id, revaluation_date, source_checksum),
    CONSTRAINT accounting_currency_revaluation_runs_period_fk
        FOREIGN KEY (org_id, period_id)
        REFERENCES accounting.periods(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_currency_revaluation_runs_draft_fk
        FOREIGN KEY (org_id, generated_draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE RESTRICT,
    CHECK (reviewed_by IS NULL OR btrim(reviewed_by) <> ''),
    CHECK (
        (status = 'draft' AND reviewed_by IS NULL)
        OR
        (
            status IN ('ready', 'posted')
            AND reviewed_by IS NOT NULL
            AND generated_draft_id IS NOT NULL
        )
    )
);

CREATE TABLE accounting.currency_revaluation_lines (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    revaluation_run_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    account_id uuid NOT NULL,
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    currency_amount numeric(24, 6) NOT NULL,
    carrying_amount numeric(24, 6) NOT NULL,
    closing_rate numeric(24, 10) NOT NULL CHECK (closing_rate > 0),
    revalued_amount numeric(24, 6) NOT NULL,
    exchange_difference_amount numeric(24, 6) NOT NULL,
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_currency_revaluation_lines_number_unique
        UNIQUE (org_id, revaluation_run_id, line_no),
    CONSTRAINT accounting_currency_revaluation_lines_run_fk
        FOREIGN KEY (org_id, revaluation_run_id)
        REFERENCES accounting.currency_revaluation_runs(org_id, id)
        ON DELETE CASCADE,
    CONSTRAINT accounting_currency_revaluation_lines_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION accounting.assert_open_item_valid(
    target_org_id uuid,
    target_open_item_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    item_record accounting.open_items%ROWTYPE;
    applied_currency numeric(24, 6);
    applied_functional numeric(24, 6);
BEGIN
    SELECT *
      INTO item_record
      FROM accounting.open_items
     WHERE org_id = target_org_id
       AND id = target_open_item_id
     FOR UPDATE;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM accounting.open_item_applications AS application
          JOIN accounting.open_item_applications AS original
            ON original.org_id = application.org_id
           AND original.id = application.reverses_application_id
         WHERE application.org_id = target_org_id
           AND application.open_item_id = target_open_item_id
           AND (
               original.open_item_id <> application.open_item_id
               OR original.currency_amount <> application.currency_amount
               OR original.functional_amount <> application.functional_amount
               OR original.exchange_difference_amount
                    <> -application.exchange_difference_amount
           )
    ) THEN
        RAISE EXCEPTION 'application reversal must exactly cancel its target'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_open_item_application_reversal';
    END IF;

    WITH RECURSIVE application_effect AS (
        SELECT application.*, 1::integer AS effect_sign
          FROM accounting.open_item_applications AS application
         WHERE application.org_id = target_org_id
           AND application.open_item_id = target_open_item_id
           AND application.reverses_application_id IS NULL
        UNION ALL
        SELECT application.*, -parent.effect_sign
          FROM accounting.open_item_applications AS application
          JOIN application_effect AS parent
            ON parent.org_id = application.org_id
           AND parent.id = application.reverses_application_id
         WHERE application.org_id = target_org_id
           AND application.open_item_id = target_open_item_id
    )
    SELECT
        coalesce(sum(currency_amount * effect_sign), 0),
        coalesce(sum(functional_amount * effect_sign), 0)
      INTO applied_currency, applied_functional
      FROM application_effect;

    IF applied_currency < 0
       OR applied_functional < 0
       OR applied_currency > item_record.original_currency_amount
       OR applied_functional > item_record.original_functional_amount THEN
        RAISE EXCEPTION 'open item applications exceed the item balance'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_open_item_applications_capacity';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.check_open_item_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM accounting.assert_open_item_valid(
        coalesce(NEW.org_id, OLD.org_id),
        coalesce(NEW.open_item_id, OLD.open_item_id)
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_reconciliation_match()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    reconciliation_record accounting.reconciliations%ROWTYPE;
    financial_account_record accounting.financial_accounts%ROWTYPE;
    statement_record accounting.statement_transactions%ROWTYPE;
    line_account_id uuid;
BEGIN
    SELECT *
      INTO reconciliation_record
      FROM accounting.reconciliations
     WHERE org_id = coalesce(NEW.org_id, OLD.org_id)
       AND id = coalesce(NEW.reconciliation_id, OLD.reconciliation_id);

    IF reconciliation_record.status = 'closed' THEN
        RAISE EXCEPTION 'matches of a closed reconciliation are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;

    SELECT *
      INTO financial_account_record
      FROM accounting.financial_accounts
     WHERE org_id = NEW.org_id
       AND id = reconciliation_record.financial_account_id;

    SELECT *
      INTO statement_record
      FROM accounting.statement_transactions
     WHERE org_id = NEW.org_id
       AND id = NEW.statement_transaction_id;

    SELECT line.account_id
      INTO line_account_id
      FROM accounting.journal_lines AS line
     WHERE line.org_id = NEW.org_id
       AND line.journal_entry_id = NEW.journal_entry_id
       AND line.id = NEW.journal_line_id;

    IF statement_record.financial_account_id
           <> reconciliation_record.financial_account_id
       OR statement_record.booked_at NOT BETWEEN
           reconciliation_record.start_date AND reconciliation_record.end_date
       OR line_account_id <> financial_account_record.ledger_account_id THEN
        RAISE EXCEPTION 'reconciliation match crosses its account or date boundary'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliation_matches_boundary';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.assert_reconciliation_capacity(
    target_org_id uuid,
    target_reconciliation_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended(target_org_id::text, 910011)
    );

    IF EXISTS (
        SELECT 1
          FROM accounting.reconciliation_matches AS match
          JOIN accounting.statement_transactions AS statement
            ON statement.org_id = match.org_id
           AND statement.id = match.statement_transaction_id
         WHERE match.org_id = target_org_id
         GROUP BY statement.id, statement.amount
        HAVING sum(match.matched_amount) > abs(statement.amount)
    ) OR EXISTS (
        SELECT 1
          FROM accounting.reconciliation_matches AS match
          JOIN accounting.journal_lines AS line
            ON line.org_id = match.org_id
           AND line.id = match.journal_line_id
         WHERE match.org_id = target_org_id
         GROUP BY line.id, line.debit_amount, line.credit_amount
        HAVING sum(match.functional_amount)
               > line.debit_amount + line.credit_amount
    ) THEN
        RAISE EXCEPTION 'reconciliation match exceeds a transaction or journal line'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliation_matches_capacity';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.check_reconciliation_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM accounting.assert_reconciliation_capacity(
        coalesce(NEW.org_id, OLD.org_id),
        coalesce(NEW.reconciliation_id, OLD.reconciliation_id)
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_reconciliation_transition()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended(coalesce(NEW.org_id, OLD.org_id)::text, 910012)
    );

    IF TG_OP <> 'DELETE' AND EXISTS (
        SELECT 1
          FROM accounting.reconciliations AS reconciliation
         WHERE reconciliation.org_id = NEW.org_id
           AND reconciliation.financial_account_id = NEW.financial_account_id
           AND reconciliation.id <> NEW.id
           AND daterange(reconciliation.start_date, reconciliation.end_date, '[]')
               && daterange(NEW.start_date, NEW.end_date, '[]')
    ) THEN
        RAISE EXCEPTION 'reconciliation ranges cannot overlap for one account'
            USING ERRCODE = '23P01',
                  CONSTRAINT = 'accounting_reconciliations_no_overlap';
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'draft' THEN
            RAISE EXCEPTION 'a new reconciliation must be a draft'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_reconciliations_initially_draft';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        IF OLD.status <> 'draft' THEN
            RAISE EXCEPTION 'a closed reconciliation cannot be deleted'
                USING ERRCODE = '55000';
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.status IS NOT DISTINCT FROM NEW.status THEN
        IF (OLD.financial_account_id, OLD.start_date, OLD.end_date)
               IS DISTINCT FROM
           (NEW.financial_account_id, NEW.start_date, NEW.end_date)
           AND EXISTS (
                SELECT 1
                  FROM accounting.reconciliation_matches AS match
                 WHERE match.org_id = OLD.org_id
                   AND match.reconciliation_id = OLD.id
           ) THEN
            RAISE EXCEPTION 'reconciliation bounds are immutable after matching'
                USING ERRCODE = '55000',
                      CONSTRAINT = 'accounting_reconciliations_bounds_after_match';
        END IF;
        IF OLD.status = 'closed' AND NEW IS DISTINCT FROM OLD THEN
            RAISE EXCEPTION 'a closed reconciliation is immutable'
                USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;

    IF NOT (
        (OLD.status = 'draft' AND NEW.status = 'closed')
        OR
        (OLD.status = 'closed' AND NEW.status = 'draft')
    ) THEN
        RAISE EXCEPTION 'invalid reconciliation transition % -> %',
            OLD.status,
            NEW.status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliations_transition';
    END IF;

    IF NEW.version <> OLD.version + 1
       OR NEW.status_changed_by IS NULL
       OR btrim(NEW.status_changed_by) = '' THEN
        RAISE EXCEPTION 'reconciliation transition requires actor and next version'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliations_transition_metadata';
    END IF;

    IF OLD.status = 'closed'
       AND (
           NEW.transition_reason IS NULL
           OR btrim(NEW.transition_reason) = ''
       ) THEN
        RAISE EXCEPTION 'reopening a reconciliation requires a reason'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliations_reopen_reason';
    END IF;

    IF NEW.status = 'closed' AND EXISTS (
        SELECT 1
          FROM accounting.statement_transactions AS statement
         WHERE statement.org_id = NEW.org_id
           AND statement.financial_account_id = NEW.financial_account_id
           AND statement.booked_at BETWEEN NEW.start_date AND NEW.end_date
           AND coalesce((
               SELECT sum(match.matched_amount)
                 FROM accounting.reconciliation_matches AS match
                WHERE match.org_id = statement.org_id
                  AND match.reconciliation_id = NEW.id
                  AND match.statement_transaction_id = statement.id
           ), 0) <> abs(statement.amount)
    ) THEN
        RAISE EXCEPTION 'all statement transactions must be fully matched before closing'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliations_fully_matched';
    END IF;

    IF NEW.status = 'closed' AND EXISTS (
        SELECT 1
          FROM accounting.journal_lines AS line
          JOIN accounting.journal_entries AS entry
            ON entry.org_id = line.org_id
           AND entry.id = line.journal_entry_id
          JOIN accounting.financial_accounts AS financial_account
            ON financial_account.org_id = line.org_id
           AND financial_account.ledger_account_id = line.account_id
         WHERE line.org_id = NEW.org_id
           AND financial_account.id = NEW.financial_account_id
           AND entry.entry_date BETWEEN NEW.start_date AND NEW.end_date
           AND coalesce((
               SELECT sum(match.functional_amount)
                 FROM accounting.reconciliation_matches AS match
                WHERE match.org_id = line.org_id
                  AND match.reconciliation_id = NEW.id
                  AND match.journal_line_id = line.id
           ), 0) <> line.debit_amount + line.credit_amount
    ) THEN
        RAISE EXCEPTION 'all ledger movements must be fully matched before closing'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliations_ledger_fully_matched';
    END IF;

    IF NEW.status = 'closed'
       AND NEW.closing_balance IS DISTINCT FROM (
           NEW.opening_balance + coalesce((
               SELECT sum(statement.amount)
                 FROM accounting.statement_transactions AS statement
                WHERE statement.org_id = NEW.org_id
                  AND statement.financial_account_id = NEW.financial_account_id
                  AND statement.booked_at BETWEEN NEW.start_date AND NEW.end_date
           ), 0)
       ) THEN
        RAISE EXCEPTION 'statement opening, movements and closing balance do not reconcile'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_reconciliations_statement_equation';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.audit_reconciliation_transition()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    INSERT INTO accounting.reconciliation_events (
        org_id,
        reconciliation_id,
        from_status,
        to_status,
        actor,
        reason
    )
    VALUES (
        NEW.org_id,
        NEW.id,
        OLD.status,
        NEW.status,
        NEW.status_changed_by,
        NEW.transition_reason
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_period_lock_checks()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    completed_checks integer;
BEGIN
    IF NEW.status <> 'locked' OR OLD.status = 'locked' THEN
        RETURN NEW;
    END IF;

    SELECT count(*)
      INTO completed_checks
      FROM accounting.period_close_checks AS check_result
     WHERE check_result.org_id = NEW.org_id
       AND check_result.period_id = NEW.id
       AND check_result.status IN ('passed', 'warning');

    IF completed_checks <> 6 OR EXISTS (
        SELECT 1
          FROM accounting.period_close_checks AS check_result
         WHERE check_result.org_id = NEW.org_id
           AND check_result.period_id = NEW.id
           AND check_result.status = 'blocked'
    ) THEN
        RAISE EXCEPTION 'period close checklist is incomplete or blocked'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_periods_close_checklist';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.guard_calculation_lines()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    parent_status text;
BEGIN
    IF TG_TABLE_NAME = 'inflation_run_lines' THEN
        SELECT run.status
          INTO parent_status
          FROM accounting.inflation_runs AS run
         WHERE run.org_id = coalesce(NEW.org_id, OLD.org_id)
           AND run.id = coalesce(
               NEW.inflation_run_id,
               OLD.inflation_run_id
           );
    ELSE
        SELECT run.status
          INTO parent_status
          FROM accounting.currency_revaluation_runs AS run
         WHERE run.org_id = coalesce(NEW.org_id, OLD.org_id)
           AND run.id = coalesce(
               NEW.revaluation_run_id,
               OLD.revaluation_run_id
           );
    END IF;

    IF parent_status <> 'draft' THEN
        RAISE EXCEPTION 'reviewed calculation lines are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_inflation_line_amounts()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    rounding_scale smallint;
    expected_adjusted numeric;
BEGIN
    SELECT accounting.currency_minor_units(settings.functional_currency)
      INTO rounding_scale
      FROM accounting.organization_settings AS settings
     WHERE settings.org_id = NEW.org_id;

    IF rounding_scale IS NULL THEN
        RAISE EXCEPTION 'accounting settings are required for inflation'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_inflation_line_currency';
    END IF;

    expected_adjusted := round(
        NEW.original_amount * NEW.coefficient,
        rounding_scale
    );

    IF NEW.adjusted_amount IS DISTINCT FROM expected_adjusted
       OR NEW.adjustment_amount IS DISTINCT FROM (
           expected_adjusted - NEW.original_amount
       ) THEN
        RAISE EXCEPTION
            'inflation amounts do not match functional-currency rounding'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_inflation_line_amounts';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_revaluation_line_amounts()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    rounding_scale smallint;
    expected_revalued numeric;
BEGIN
    SELECT accounting.currency_minor_units(settings.functional_currency)
      INTO rounding_scale
      FROM accounting.organization_settings AS settings
     WHERE settings.org_id = NEW.org_id;

    IF rounding_scale IS NULL THEN
        RAISE EXCEPTION 'accounting settings are required for revaluation'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_revaluation_line_currency';
    END IF;

    expected_revalued := round(
        NEW.currency_amount * NEW.closing_rate,
        rounding_scale
    );

    IF NEW.revalued_amount IS DISTINCT FROM expected_revalued
       OR NEW.exchange_difference_amount IS DISTINCT FROM (
           expected_revalued - NEW.carrying_amount
       ) THEN
        RAISE EXCEPTION
            'revaluation amounts do not match functional-currency rounding'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_revaluation_line_amounts';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_calculation_run_transition()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'draft' THEN
            RAISE EXCEPTION 'a new calculation run must be a draft'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        IF OLD.status <> 'draft' THEN
            RAISE EXCEPTION 'a reviewed calculation run cannot be deleted'
                USING ERRCODE = '55000';
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.status IS DISTINCT FROM NEW.status THEN
        IF NOT (
            (OLD.status = 'draft' AND NEW.status = 'ready')
            OR
            (OLD.status = 'ready' AND NEW.status = 'posted')
        ) THEN
            RAISE EXCEPTION 'invalid calculation run transition % -> %',
                OLD.status,
                NEW.status
                USING ERRCODE = '23514';
        END IF;
        IF NEW.version <> OLD.version + 1 THEN
            RAISE EXCEPTION 'calculation transition must increment version'
                USING ERRCODE = '40001';
        END IF;
    ELSIF OLD.status <> 'draft' AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'reviewed calculation run is immutable'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.assert_open_item_valid(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.check_open_item_constraint()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_reconciliation_match()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.assert_reconciliation_capacity(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.check_reconciliation_constraint()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_reconciliation_transition()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.audit_reconciliation_transition()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_period_lock_checks()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.guard_calculation_lines()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_inflation_line_amounts()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_revaluation_line_amounts()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_calculation_run_transition()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER accounting_open_item_applications_valid
AFTER INSERT OR UPDATE OR DELETE
ON accounting.open_item_applications
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.check_open_item_constraint();

CREATE TRIGGER accounting_open_items_immutable
BEFORE UPDATE OR DELETE ON accounting.open_items
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_open_item_applications_immutable
BEFORE UPDATE OR DELETE ON accounting.open_item_applications
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_exchange_rates_immutable
BEFORE UPDATE OR DELETE ON accounting.exchange_rates
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_statement_imports_immutable
BEFORE UPDATE OR DELETE ON accounting.statement_imports
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_statement_transactions_immutable
BEFORE UPDATE OR DELETE ON accounting.statement_transactions
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_reconciliation_matches_guard
BEFORE INSERT OR UPDATE OR DELETE
ON accounting.reconciliation_matches
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_reconciliation_match();

CREATE CONSTRAINT TRIGGER accounting_reconciliation_matches_capacity
AFTER INSERT OR UPDATE OR DELETE
ON accounting.reconciliation_matches
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.check_reconciliation_constraint();

CREATE TRIGGER accounting_reconciliations_transition
BEFORE INSERT OR UPDATE OR DELETE
ON accounting.reconciliations
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_reconciliation_transition();

CREATE TRIGGER accounting_reconciliations_audit
AFTER UPDATE OF status
ON accounting.reconciliations
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION accounting.audit_reconciliation_transition();

CREATE TRIGGER accounting_reconciliation_events_immutable
BEFORE UPDATE OR DELETE ON accounting.reconciliation_events
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_periods_close_checklist
BEFORE UPDATE OF status
ON accounting.periods
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_period_lock_checks();

CREATE TRIGGER accounting_inflation_indices_immutable
BEFORE UPDATE OR DELETE ON accounting.inflation_indices
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_inflation_run_lines_guard
BEFORE UPDATE OR DELETE ON accounting.inflation_run_lines
FOR EACH ROW
EXECUTE FUNCTION accounting.guard_calculation_lines();

CREATE TRIGGER accounting_inflation_run_lines_amounts
BEFORE INSERT OR UPDATE ON accounting.inflation_run_lines
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_inflation_line_amounts();

CREATE TRIGGER accounting_currency_revaluation_lines_guard
BEFORE UPDATE OR DELETE ON accounting.currency_revaluation_lines
FOR EACH ROW
EXECUTE FUNCTION accounting.guard_calculation_lines();

CREATE TRIGGER accounting_currency_revaluation_lines_amounts
BEFORE INSERT OR UPDATE ON accounting.currency_revaluation_lines
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_revaluation_line_amounts();

CREATE TRIGGER accounting_inflation_runs_transition
BEFORE INSERT OR UPDATE OR DELETE ON accounting.inflation_runs
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_calculation_run_transition();

CREATE TRIGGER accounting_currency_revaluation_runs_transition
BEFORE INSERT OR UPDATE OR DELETE ON accounting.currency_revaluation_runs
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_calculation_run_transition();

CREATE VIEW accounting.journal_view
WITH (security_invoker = true)
AS
SELECT
    entry.org_id,
    entry.id AS journal_entry_id,
    entry.entry_number,
    entry.entry_date,
    entry.period_id,
    entry.entry_kind,
    entry.description AS entry_description,
    entry.source_type,
    entry.source_id,
    entry.posting_kind,
    entry.reverses_entry_id,
    entry.posted_at,
    line.id AS journal_line_id,
    line.line_no,
    line.account_id,
    account.code AS account_code,
    account.name AS account_name,
    account.account_class,
    line.description AS line_description,
    line.debit_amount,
    line.credit_amount,
    line.currency_code,
    line.currency_amount,
    line.exchange_rate,
    line.party_type,
    line.party_id,
    line.tax_code,
    line.origin_date
FROM accounting.journal_entries AS entry
JOIN accounting.journal_lines AS line
  ON line.org_id = entry.org_id
 AND line.journal_entry_id = entry.id
JOIN accounting.accounts AS account
  ON account.org_id = line.org_id
 AND account.id = line.account_id;

CREATE VIEW accounting.general_ledger_view
WITH (security_invoker = true)
AS
SELECT
    journal.*,
    journal.debit_amount - journal.credit_amount AS signed_amount,
    sum(journal.debit_amount - journal.credit_amount) OVER (
        PARTITION BY journal.org_id, journal.account_id
        ORDER BY
            journal.entry_date,
            journal.entry_number,
            journal.line_no,
            journal.journal_line_id
        ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
    ) AS running_balance
FROM accounting.journal_view AS journal;

CREATE VIEW accounting.trial_balance_view
WITH (security_invoker = true)
AS
SELECT
    account.org_id,
    account.id AS account_id,
    account.code,
    account.name,
    account.account_class,
    account.normal_balance,
    coalesce(sum(line.debit_amount), 0)::numeric(24, 6) AS debit_total,
    coalesce(sum(line.credit_amount), 0)::numeric(24, 6) AS credit_total,
    coalesce(
        sum(line.debit_amount - line.credit_amount),
        0
    )::numeric(24, 6) AS balance
FROM accounting.accounts AS account
LEFT JOIN accounting.journal_lines AS line
  ON line.org_id = account.org_id
 AND line.account_id = account.id
GROUP BY
    account.org_id,
    account.id,
    account.code,
    account.name,
    account.account_class,
    account.normal_balance;

CREATE VIEW accounting.open_item_balances_view
WITH (security_invoker = true)
AS
WITH RECURSIVE application_effect AS (
    SELECT
        application.org_id,
        application.id,
        application.open_item_id,
        application.currency_amount,
        application.functional_amount,
        1::integer AS effect_sign
      FROM accounting.open_item_applications AS application
     WHERE application.reverses_application_id IS NULL
    UNION ALL
    SELECT
        application.org_id,
        application.id,
        application.open_item_id,
        application.currency_amount,
        application.functional_amount,
        -parent.effect_sign
      FROM accounting.open_item_applications AS application
      JOIN application_effect AS parent
        ON parent.org_id = application.org_id
       AND parent.id = application.reverses_application_id
)
SELECT
    item.org_id,
    item.id AS open_item_id,
    item.item_type,
    item.party_type,
    item.party_id,
    item.account_id,
    item.document_type,
    item.document_id,
    item.currency_code,
    item.original_currency_amount,
    item.original_functional_amount,
    item.issued_at,
    item.due_date,
    (
        item.original_currency_amount
        - coalesce(sum(
            application.currency_amount * application.effect_sign
        ), 0)
    )::numeric(24, 6) AS remaining_currency_amount,
    (
        item.original_functional_amount
        - coalesce(sum(
            application.functional_amount * application.effect_sign
        ), 0)
    )::numeric(24, 6) AS remaining_functional_amount
FROM accounting.open_items AS item
LEFT JOIN application_effect AS application
  ON application.org_id = item.org_id
 AND application.open_item_id = item.id
GROUP BY
    item.org_id,
    item.id,
    item.item_type,
    item.party_type,
    item.party_id,
    item.account_id,
    item.document_type,
    item.document_id,
    item.currency_code,
    item.original_currency_amount,
    item.original_functional_amount,
    item.issued_at,
    item.due_date;

CREATE FUNCTION accounting.open_item_balances_as_of(target_as_of date)
RETURNS TABLE (
    org_id uuid,
    open_item_id uuid,
    item_type text,
    party_type text,
    party_id text,
    account_id uuid,
    document_type text,
    document_id text,
    currency_code character(3),
    original_currency_amount numeric(24, 6),
    original_functional_amount numeric(24, 6),
    issued_at date,
    due_date date,
    remaining_currency_amount numeric(24, 6),
    remaining_functional_amount numeric(24, 6)
)
LANGUAGE sql
STABLE
STRICT
SECURITY INVOKER
SET search_path = pg_catalog, accounting
AS $function$
    WITH RECURSIVE dated_application AS (
        SELECT
            application.*,
            settlement.entry_date AS application_date
          FROM accounting.open_item_applications AS application
          JOIN accounting.journal_entries AS settlement
            ON settlement.org_id = application.org_id
           AND settlement.id = application.settlement_journal_entry_id
    ),
    application_effect AS (
        SELECT
            application.org_id,
            application.id,
            application.open_item_id,
            application.currency_amount,
            application.functional_amount,
            1::integer AS effect_sign
          FROM dated_application AS application
         WHERE application.reverses_application_id IS NULL
           AND application.application_date <= target_as_of
        UNION ALL
        SELECT
            application.org_id,
            application.id,
            application.open_item_id,
            application.currency_amount,
            application.functional_amount,
            -parent.effect_sign
          FROM dated_application AS application
          JOIN application_effect AS parent
            ON parent.org_id = application.org_id
           AND parent.id = application.reverses_application_id
         WHERE application.application_date <= target_as_of
    )
    SELECT
        item.org_id,
        item.id AS open_item_id,
        item.item_type,
        item.party_type,
        item.party_id,
        item.account_id,
        item.document_type,
        item.document_id,
        item.currency_code,
        item.original_currency_amount,
        item.original_functional_amount,
        item.issued_at,
        item.due_date,
        (
            item.original_currency_amount
            - coalesce(sum(
                application.currency_amount * application.effect_sign
            ), 0)
        )::numeric(24, 6) AS remaining_currency_amount,
        (
            item.original_functional_amount
            - coalesce(sum(
                application.functional_amount * application.effect_sign
            ), 0)
        )::numeric(24, 6) AS remaining_functional_amount
    FROM accounting.open_items AS item
    LEFT JOIN application_effect AS application
      ON application.org_id = item.org_id
     AND application.open_item_id = item.id
    WHERE item.issued_at <= target_as_of
    GROUP BY
        item.org_id,
        item.id,
        item.item_type,
        item.party_type,
        item.party_id,
        item.account_id,
        item.document_type,
        item.document_id,
        item.currency_code,
        item.original_currency_amount,
        item.original_functional_amount,
        item.issued_at,
        item.due_date
$function$;

REVOKE ALL
ON FUNCTION accounting.open_item_balances_as_of(date)
FROM PUBLIC;

CREATE VIEW accounting.financial_account_movements_view
WITH (security_invoker = true)
AS
SELECT
    financial_account.org_id,
    financial_account.id AS financial_account_id,
    financial_account.account_type,
    financial_account.name,
    financial_account.currency_code,
    entry.id AS journal_entry_id,
    entry.entry_number,
    entry.entry_date,
    line.id AS journal_line_id,
    line.debit_amount,
    line.credit_amount,
    line.debit_amount - line.credit_amount AS signed_amount,
    line.description,
    entry.source_type,
    entry.source_id
FROM accounting.financial_accounts AS financial_account
JOIN accounting.journal_lines AS line
  ON line.org_id = financial_account.org_id
 AND line.account_id = financial_account.ledger_account_id
JOIN accounting.journal_entries AS entry
  ON entry.org_id = line.org_id
 AND entry.id = line.journal_entry_id;

REVOKE ALL ON
    accounting.journal_view,
    accounting.general_ledger_view,
    accounting.trial_balance_view,
    accounting.open_item_balances_view,
    accounting.financial_account_movements_view
FROM PUBLIC;

DO $rls$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'exchange_rates',
        'open_items',
        'open_item_applications',
        'financial_accounts',
        'statement_imports',
        'statement_transactions',
        'reconciliations',
        'reconciliation_matches',
        'reconciliation_events',
        'period_close_checks',
        'inflation_indices',
        'inflation_runs',
        'inflation_run_lines',
        'currency_revaluation_runs',
        'currency_revaluation_lines'
    ]
    LOOP
        EXECUTE format(
            'ALTER TABLE accounting.%I ENABLE ROW LEVEL SECURITY',
            table_name
        );
        EXECUTE format(
            'ALTER TABLE accounting.%I FORCE ROW LEVEL SECURITY',
            table_name
        );
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON accounting.%I
             USING (
                 org_id = nullif(
                     current_setting(''app.org_id'', true),
                     ''''
                 )::uuid
             )
             WITH CHECK (
                 org_id = nullif(
                     current_setting(''app.org_id'', true),
                     ''''
                 )::uuid
             )',
            table_name
        );
    END LOOP;
END
$rls$;

REVOKE ALL ON ALL TABLES IN SCHEMA accounting FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT ON
            accounting.journal_view,
            accounting.general_ledger_view,
            accounting.trial_balance_view,
            accounting.open_item_balances_view,
            accounting.financial_account_movements_view
        TO pymes_backend;

        GRANT SELECT, INSERT ON
            accounting.exchange_rates,
            accounting.open_items,
            accounting.open_item_applications,
            accounting.statement_imports,
            accounting.statement_transactions,
            accounting.inflation_indices
        TO pymes_backend;

        GRANT SELECT, INSERT, UPDATE, DELETE ON
            accounting.financial_accounts,
            accounting.reconciliations,
            accounting.reconciliation_matches,
            accounting.period_close_checks,
            accounting.inflation_runs,
            accounting.inflation_run_lines,
            accounting.currency_revaluation_runs,
            accounting.currency_revaluation_lines
        TO pymes_backend;

        GRANT SELECT ON accounting.reconciliation_events TO pymes_backend;
        GRANT EXECUTE
        ON FUNCTION accounting.open_item_balances_as_of(date)
        TO pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_accounting_worker'
    ) THEN
        GRANT SELECT ON
            accounting.open_item_balances_view
        TO pymes_fiscal_accounting_worker;

        GRANT SELECT, INSERT ON
            accounting.open_items,
            accounting.open_item_applications
        TO pymes_fiscal_accounting_worker;

        GRANT EXECUTE
        ON FUNCTION accounting.open_item_balances_as_of(date)
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
