CREATE SCHEMA IF NOT EXISTS accounting;

REVOKE CREATE ON SCHEMA accounting FROM PUBLIC;

CREATE OR REPLACE FUNCTION app.current_org_id()
RETURNS uuid
LANGUAGE sql
STABLE
SET search_path = pg_catalog
AS $function$
    SELECT nullif(current_setting('app.org_id', true), '')::uuid
$function$;

CREATE OR REPLACE FUNCTION app.assert_org_context(requested_org_id uuid)
RETURNS void
LANGUAGE plpgsql
STABLE
SET search_path = pg_catalog, app
AS $function$
BEGIN
    IF requested_org_id IS NULL
       OR requested_org_id IS DISTINCT FROM app.current_org_id() THEN
        RAISE EXCEPTION 'organization context does not match requested organization'
            USING ERRCODE = '42501';
    END IF;
END
$function$;

REVOKE ALL ON FUNCTION app.current_org_id() FROM PUBLIC;
REVOKE ALL ON FUNCTION app.assert_org_context(uuid) FROM PUBLIC;

CREATE TABLE accounting.currencies (
    code char(3) PRIMARY KEY CHECK (code ~ '^[A-Z]{3}$'),
    minor_units smallint NOT NULL CHECK (minor_units BETWEEN 0 AND 6),
    name text NOT NULL CHECK (btrim(name) <> '')
);

INSERT INTO accounting.currencies (code, minor_units, name)
VALUES
    ('ARS', 2, 'Peso argentino'),
    ('USD', 2, 'Dólar estadounidense'),
    ('EUR', 2, 'Euro');

CREATE OR REPLACE FUNCTION accounting.currency_minor_units(
    requested_currency char(3)
)
RETURNS smallint
LANGUAGE sql
STABLE
SET search_path = pg_catalog, accounting
AS $function$
    SELECT coalesce((
        SELECT currency.minor_units
          FROM accounting.currencies AS currency
         WHERE currency.code = requested_currency
    ), 2)::smallint
$function$;

REVOKE ALL
ON FUNCTION accounting.currency_minor_units(char)
FROM PUBLIC;

ALTER TABLE iam.memberships
    ADD CONSTRAINT iam_memberships_org_identity_unique
    UNIQUE (org_id, id);

CREATE TABLE iam.membership_permissions (
    org_id uuid NOT NULL,
    membership_id uuid NOT NULL,
    permission text NOT NULL CHECK (
        permission IN (
            'accounting:view',
            'accounting:manage',
            'fiscal:view',
            'fiscal:manage'
        )
    ),
    granted_by text NOT NULL CHECK (btrim(granted_by) <> ''),
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, membership_id, permission),
    CONSTRAINT iam_membership_permissions_membership_fk
        FOREIGN KEY (org_id, membership_id)
        REFERENCES iam.memberships(org_id, id)
        ON DELETE CASCADE
);

CREATE INDEX iam_membership_permissions_permission_idx
    ON iam.membership_permissions (org_id, permission, membership_id);

ALTER TABLE iam.membership_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.membership_permissions FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_membership_permissions_tenant_policy
ON iam.membership_permissions
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
);

REVOKE ALL ON TABLE iam.membership_permissions FROM PUBLIC;

CREATE TABLE accounting.chart_templates (
    code text NOT NULL CHECK (code = lower(btrim(code)) AND code <> ''),
    version integer NOT NULL CHECK (version > 0),
    country_code char(2) NOT NULL CHECK (country_code ~ '^[A-Z]{2}$'),
    functional_currency char(3) NOT NULL
        CHECK (functional_currency ~ '^[A-Z]{3}$'),
    name text NOT NULL CHECK (btrim(name) <> ''),
    source text NOT NULL CHECK (btrim(source) <> ''),
    source_checksum char(64) NOT NULL
        CHECK (source_checksum ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (code, version)
);

CREATE TABLE accounting.chart_template_accounts (
    template_code text NOT NULL,
    template_version integer NOT NULL,
    code text NOT NULL CHECK (btrim(code) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    account_class text NOT NULL CHECK (
        account_class IN (
            'asset',
            'liability',
            'equity',
            'revenue',
            'cost',
            'expense'
        )
    ),
    parent_code text,
    normal_balance text NOT NULL CHECK (normal_balance IN ('debit', 'credit')),
    monetary_class text NOT NULL CHECK (
        monetary_class IN ('monetary', 'non_monetary', 'not_applicable')
    ),
    posting_allowed boolean NOT NULL,
    display_order integer NOT NULL CHECK (display_order >= 0),
    PRIMARY KEY (template_code, template_version, code),
    CONSTRAINT accounting_chart_template_accounts_template_fk
        FOREIGN KEY (template_code, template_version)
        REFERENCES accounting.chart_templates(code, version)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_chart_template_accounts_parent_fk
        FOREIGN KEY (template_code, template_version, parent_code)
        REFERENCES accounting.chart_template_accounts(
            template_code,
            template_version,
            code
        )
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (parent_code IS NULL OR parent_code <> code),
    CHECK (NOT posting_allowed OR monetary_class <> 'not_applicable')
);

CREATE TABLE accounting.chart_template_mappings (
    template_code text NOT NULL,
    template_version integer NOT NULL,
    mapping_key text NOT NULL
        CHECK (mapping_key = lower(btrim(mapping_key)) AND mapping_key <> ''),
    account_code text NOT NULL,
    description text NOT NULL DEFAULT '',
    PRIMARY KEY (template_code, template_version, mapping_key),
    CONSTRAINT accounting_chart_template_mappings_account_fk
        FOREIGN KEY (template_code, template_version, account_code)
        REFERENCES accounting.chart_template_accounts(
            template_code,
            template_version,
            code
        )
        ON DELETE RESTRICT
);

CREATE TABLE accounting.organization_settings (
    org_id uuid PRIMARY KEY
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    country_code char(2) NOT NULL DEFAULT 'AR'
        CHECK (country_code ~ '^[A-Z]{2}$'),
    functional_currency char(3) NOT NULL DEFAULT 'ARS'
        CHECK (functional_currency ~ '^[A-Z]{3}$'),
    fiscal_year_start_month smallint NOT NULL DEFAULT 1
        CHECK (fiscal_year_start_month BETWEEN 1 AND 12),
    timezone text NOT NULL DEFAULT 'America/Argentina/Buenos_Aires'
        CHECK (btrim(timezone) <> ''),
    chart_template_code text,
    chart_template_version integer,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT accounting_organization_settings_template_pair_check CHECK (
        (chart_template_code IS NULL AND chart_template_version IS NULL)
        OR
        (chart_template_code IS NOT NULL AND chart_template_version IS NOT NULL)
    ),
    CONSTRAINT accounting_organization_settings_template_fk
        FOREIGN KEY (chart_template_code, chart_template_version)
        REFERENCES accounting.chart_templates(code, version)
        ON DELETE RESTRICT
);

CREATE TABLE accounting.accounts (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    code text NOT NULL CHECK (btrim(code) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    account_class text NOT NULL CHECK (
        account_class IN (
            'asset',
            'liability',
            'equity',
            'revenue',
            'cost',
            'expense'
        )
    ),
    parent_id uuid,
    normal_balance text NOT NULL CHECK (normal_balance IN ('debit', 'credit')),
    monetary_class text NOT NULL CHECK (
        monetary_class IN ('monetary', 'non_monetary', 'not_applicable')
    ),
    posting_allowed boolean NOT NULL DEFAULT true,
    system_key text,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    archived_at timestamptz,
    trashed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_accounts_code_unique UNIQUE (org_id, code),
    CONSTRAINT accounting_accounts_parent_fk
        FOREIGN KEY (org_id, parent_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (parent_id IS NULL OR parent_id <> id),
    CHECK (system_key IS NULL OR btrim(system_key) <> ''),
    CHECK (NOT posting_allowed OR monetary_class <> 'not_applicable'),
    CHECK (NOT (archived_at IS NOT NULL AND trashed_at IS NOT NULL))
);

CREATE UNIQUE INDEX accounting_accounts_system_key_uidx
    ON accounting.accounts (org_id, system_key)
    WHERE system_key IS NOT NULL;

CREATE INDEX accounting_accounts_parent_idx
    ON accounting.accounts (org_id, parent_id, code);

CREATE TABLE accounting.account_mappings (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    mapping_key text NOT NULL
        CHECK (mapping_key = lower(btrim(mapping_key)) AND mapping_key <> ''),
    account_id uuid NOT NULL,
    description text NOT NULL DEFAULT '',
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_account_mappings_key_unique
        UNIQUE (org_id, mapping_key),
    CONSTRAINT accounting_account_mappings_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE accounting.periods (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    code text NOT NULL CHECK (btrim(code) <> ''),
    start_date date NOT NULL,
    end_date date NOT NULL,
    status text NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'soft_closed', 'locked')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    status_changed_by text,
    transition_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_periods_code_unique UNIQUE (org_id, code),
    CONSTRAINT accounting_periods_dates_unique
        UNIQUE (org_id, start_date, end_date),
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

CREATE INDEX accounting_periods_lookup_idx
    ON accounting.periods (org_id, start_date, end_date, status);

CREATE TABLE accounting.period_events (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    period_id uuid NOT NULL,
    from_status text NOT NULL
        CHECK (from_status IN ('open', 'soft_closed', 'locked')),
    to_status text NOT NULL
        CHECK (to_status IN ('open', 'soft_closed', 'locked')),
    actor text NOT NULL CHECK (btrim(actor) <> ''),
    reason text,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_period_events_period_fk
        FOREIGN KEY (org_id, period_id)
        REFERENCES accounting.periods(org_id, id)
        ON DELETE RESTRICT
);

CREATE INDEX accounting_period_events_period_idx
    ON accounting.period_events (org_id, period_id, occurred_at DESC);

CREATE TABLE accounting.drafts (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    entry_date date NOT NULL,
    entry_kind text NOT NULL DEFAULT 'manual' CHECK (
        entry_kind IN (
            'manual',
            'sale',
            'purchase',
            'collection',
            'payment',
            'refund',
            'inventory',
            'cogs',
            'tax',
            'adjustment',
            'closing',
            'inflation',
            'revaluation',
            'reversal'
        )
    ),
    description text NOT NULL CHECK (btrim(description) <> ''),
    source_type text,
    source_id text,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'posted', 'discarded')),
    posted_entry_id uuid,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_drafts_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CHECK (
        (source_type IS NULL AND source_id IS NULL)
        OR
        (
            source_type IS NOT NULL
            AND source_id IS NOT NULL
            AND btrim(source_type) <> ''
            AND btrim(source_id) <> ''
        )
    ),
    CHECK (
        (status = 'posted' AND posted_entry_id IS NOT NULL)
        OR
        (status <> 'posted' AND posted_entry_id IS NULL)
    )
);

CREATE TABLE accounting.draft_lines (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    draft_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    account_id uuid NOT NULL,
    description text NOT NULL DEFAULT '',
    debit_amount numeric(24, 6) NOT NULL DEFAULT 0,
    credit_amount numeric(24, 6) NOT NULL DEFAULT 0,
    currency_code char(3) NOT NULL DEFAULT 'ARS'
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    currency_amount numeric(24, 6) NOT NULL,
    exchange_rate numeric(24, 10) NOT NULL DEFAULT 1,
    exchange_rate_date date,
    exchange_rate_source text,
    party_type text,
    party_id text,
    tax_code text,
    origin_date date,
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_draft_lines_number_unique
        UNIQUE (org_id, draft_id, line_no),
    CONSTRAINT accounting_draft_lines_draft_fk
        FOREIGN KEY (org_id, draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE CASCADE,
    CONSTRAINT accounting_draft_lines_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        (debit_amount > 0 AND credit_amount = 0)
        OR
        (credit_amount > 0 AND debit_amount = 0)
    ),
    CHECK (currency_amount > 0),
    CHECK (exchange_rate > 0),
    CHECK (
        (exchange_rate_date IS NULL AND exchange_rate_source IS NULL)
        OR
        (
            exchange_rate_date IS NOT NULL
            AND exchange_rate_source IS NOT NULL
            AND btrim(exchange_rate_source) <> ''
        )
    ),
    CHECK (
        (party_type IS NULL AND party_id IS NULL)
        OR
        (
            party_type IS NOT NULL
            AND party_id IS NOT NULL
            AND btrim(party_type) <> ''
            AND btrim(party_id) <> ''
        )
    )
);

CREATE TABLE accounting.entry_sequences (
    org_id uuid PRIMARY KEY
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    next_number bigint NOT NULL DEFAULT 1 CHECK (next_number > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE accounting.journal_entries (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    entry_number bigint NOT NULL CHECK (entry_number > 0),
    entry_date date NOT NULL,
    period_id uuid NOT NULL,
    entry_kind text NOT NULL CHECK (
        entry_kind IN (
            'manual',
            'sale',
            'purchase',
            'collection',
            'payment',
            'refund',
            'inventory',
            'cogs',
            'tax',
            'adjustment',
            'closing',
            'inflation',
            'revaluation',
            'reversal'
        )
    ),
    description text NOT NULL CHECK (btrim(description) <> ''),
    functional_currency char(3) NOT NULL DEFAULT 'ARS'
        CHECK (functional_currency ~ '^[A-Z]{3}$'),
    source_type text,
    source_id text,
    source_event text NOT NULL DEFAULT 'primary'
        CHECK (btrim(source_event) <> ''),
    posting_kind text NOT NULL DEFAULT 'primary'
        CHECK (btrim(posting_kind) <> ''),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    draft_id uuid,
    reverses_entry_id uuid,
    reversal_reason text,
    reversed_by text,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    posted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_journal_entries_number_unique
        UNIQUE (org_id, entry_number),
    CONSTRAINT accounting_journal_entries_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CONSTRAINT accounting_journal_entries_period_fk
        FOREIGN KEY (org_id, period_id)
        REFERENCES accounting.periods(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_journal_entries_draft_fk
        FOREIGN KEY (org_id, draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_journal_entries_reversal_fk
        FOREIGN KEY (org_id, reverses_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (
        (source_type IS NULL AND source_id IS NULL)
        OR
        (
            source_type IS NOT NULL
            AND source_id IS NOT NULL
            AND btrim(source_type) <> ''
            AND btrim(source_id) <> ''
        )
    ),
    CHECK (
        (
            reverses_entry_id IS NULL
            AND reversal_reason IS NULL
            AND reversed_by IS NULL
            AND entry_kind <> 'reversal'
        )
        OR
        (
            reverses_entry_id IS NOT NULL
            AND reverses_entry_id <> id
            AND reversal_reason IS NOT NULL
            AND btrim(reversal_reason) <> ''
            AND reversed_by IS NOT NULL
            AND btrim(reversed_by) <> ''
            AND entry_kind = 'reversal'
        )
    )
);

CREATE UNIQUE INDEX accounting_journal_entries_source_uidx
    ON accounting.journal_entries (
        org_id,
        source_type,
        source_id,
        posting_kind
    )
    WHERE source_type IS NOT NULL AND source_id IS NOT NULL;

CREATE UNIQUE INDEX accounting_journal_entries_draft_uidx
    ON accounting.journal_entries (org_id, draft_id)
    WHERE draft_id IS NOT NULL;

CREATE UNIQUE INDEX accounting_journal_entries_direct_reversal_uidx
    ON accounting.journal_entries (org_id, reverses_entry_id)
    WHERE reverses_entry_id IS NOT NULL;

CREATE INDEX accounting_journal_entries_date_idx
    ON accounting.journal_entries (org_id, entry_date DESC, entry_number DESC);

ALTER TABLE accounting.drafts
    ADD CONSTRAINT accounting_drafts_posted_entry_fk
    FOREIGN KEY (org_id, posted_entry_id)
    REFERENCES accounting.journal_entries(org_id, id)
    ON DELETE RESTRICT
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE accounting.journal_lines (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    journal_entry_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    account_id uuid NOT NULL,
    description text NOT NULL DEFAULT '',
    debit_amount numeric(24, 6) NOT NULL DEFAULT 0,
    credit_amount numeric(24, 6) NOT NULL DEFAULT 0,
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    currency_amount numeric(24, 6) NOT NULL,
    exchange_rate numeric(24, 10) NOT NULL,
    exchange_rate_date date,
    exchange_rate_source text,
    party_type text,
    party_id text,
    tax_code text,
    origin_date date,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_journal_lines_number_unique
        UNIQUE (org_id, journal_entry_id, line_no),
    CONSTRAINT accounting_journal_lines_entry_identity_unique
        UNIQUE (org_id, journal_entry_id, id),
    CONSTRAINT accounting_journal_lines_entry_fk
        FOREIGN KEY (org_id, journal_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_journal_lines_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        (debit_amount > 0 AND credit_amount = 0)
        OR
        (credit_amount > 0 AND debit_amount = 0)
    ),
    CHECK (currency_amount > 0),
    CHECK (exchange_rate > 0),
    CHECK (
        (exchange_rate_date IS NULL AND exchange_rate_source IS NULL)
        OR
        (
            exchange_rate_date IS NOT NULL
            AND exchange_rate_source IS NOT NULL
            AND btrim(exchange_rate_source) <> ''
        )
    ),
    CHECK (
        (party_type IS NULL AND party_id IS NULL)
        OR
        (
            party_type IS NOT NULL
            AND party_id IS NOT NULL
            AND btrim(party_type) <> ''
            AND btrim(party_id) <> ''
        )
    )
);

CREATE INDEX accounting_journal_lines_account_idx
    ON accounting.journal_lines (
        org_id,
        account_id,
        journal_entry_id,
        line_no
    );

CREATE OR REPLACE FUNCTION accounting.validate_account_hierarchy()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    parent_account accounting.accounts%ROWTYPE;
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT *
      INTO parent_account
      FROM accounting.accounts
     WHERE org_id = NEW.org_id
       AND id = NEW.parent_id;

    IF NOT FOUND THEN
        RETURN NEW;
    END IF;

    IF parent_account.posting_allowed THEN
        RAISE EXCEPTION 'a posting account cannot be an account parent'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_accounts_parent_not_postable';
    END IF;

    IF parent_account.account_class <> NEW.account_class THEN
        RAISE EXCEPTION 'parent and child accounts must share their class'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_accounts_parent_class';
    END IF;

    IF EXISTS (
        WITH RECURSIVE ancestors AS (
            SELECT account.parent_id
              FROM accounting.accounts AS account
             WHERE account.org_id = NEW.org_id
               AND account.id = NEW.parent_id
            UNION ALL
            SELECT account.parent_id
              FROM accounting.accounts AS account
              JOIN ancestors ON ancestors.parent_id = account.id
             WHERE account.org_id = NEW.org_id
               AND account.parent_id IS NOT NULL
        )
        SELECT 1
          FROM ancestors
         WHERE parent_id = NEW.id
    ) THEN
        RAISE EXCEPTION 'account hierarchy cannot contain cycles'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_accounts_hierarchy_acyclic';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_account_lifecycle()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    IF NEW.posting_allowed
       AND (
           OLD.posting_allowed IS DISTINCT FROM NEW.posting_allowed
           OR OLD.parent_id IS DISTINCT FROM NEW.parent_id
       )
       AND EXISTS (
           SELECT 1
             FROM accounting.accounts AS child
            WHERE child.org_id = NEW.org_id
              AND child.parent_id = NEW.id
       ) THEN
        RAISE EXCEPTION 'an account with children cannot accept postings'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_accounts_postable_leaf';
    END IF;

    IF OLD.trashed_at IS NULL AND NEW.trashed_at IS NOT NULL THEN
        IF EXISTS (
            SELECT 1
              FROM accounting.journal_lines AS line
             WHERE line.org_id = NEW.org_id
               AND line.account_id = NEW.id
        ) OR EXISTS (
            SELECT 1
              FROM accounting.draft_lines AS line
             WHERE line.org_id = NEW.org_id
               AND line.account_id = NEW.id
        ) OR EXISTS (
            SELECT 1
              FROM accounting.account_mappings AS mapping
             WHERE mapping.org_id = NEW.org_id
               AND mapping.account_id = NEW.id
        ) OR EXISTS (
            SELECT 1
              FROM accounting.accounts AS child
             WHERE child.org_id = NEW.org_id
               AND child.parent_id = NEW.id
        ) THEN
            RAISE EXCEPTION 'only unused and unlinked accounts can be trashed'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_trash_unused';
        END IF;
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_period()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended(NEW.org_id::text, 910010)
    );

    IF EXISTS (
        SELECT 1
          FROM accounting.periods AS period
         WHERE period.org_id = NEW.org_id
           AND period.id <> NEW.id
           AND daterange(period.start_date, period.end_date, '[]')
               && daterange(NEW.start_date, NEW.end_date, '[]')
    ) THEN
        RAISE EXCEPTION 'accounting periods cannot overlap'
            USING ERRCODE = '23P01',
                  CONSTRAINT = 'accounting_periods_no_overlap';
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'open' THEN
            RAISE EXCEPTION 'a new accounting period must be open'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_initially_open';
        END IF;
        RETURN NEW;
    END IF;

    IF (OLD.start_date, OLD.end_date)
           IS DISTINCT FROM
       (NEW.start_date, NEW.end_date)
       AND EXISTS (
            SELECT 1
              FROM accounting.journal_entries AS entry
             WHERE entry.org_id = OLD.org_id
               AND entry.period_id = OLD.id
       ) THEN
        RAISE EXCEPTION 'period dates are immutable after the first posting'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_periods_dates_after_posting';
    END IF;

    IF OLD.status IS DISTINCT FROM NEW.status THEN
        IF NOT (
            (OLD.status = 'open' AND NEW.status = 'soft_closed')
            OR
            (OLD.status = 'soft_closed' AND NEW.status IN ('open', 'locked'))
            OR
            (OLD.status = 'locked' AND NEW.status = 'open')
        ) THEN
            RAISE EXCEPTION 'invalid accounting period transition % -> %',
                OLD.status,
                NEW.status
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_transition';
        END IF;

        IF NEW.version <> OLD.version + 1 THEN
            RAISE EXCEPTION 'period status transition must increment version'
                USING ERRCODE = '40001',
                      CONSTRAINT = 'accounting_periods_transition_version';
        END IF;

        IF NEW.status_changed_by IS NULL
           OR btrim(NEW.status_changed_by) = '' THEN
            RAISE EXCEPTION 'period status transition requires an actor'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_transition_actor';
        END IF;

        IF NEW.status = 'open'
           AND OLD.status <> 'open'
           AND (
               NEW.transition_reason IS NULL
               OR btrim(NEW.transition_reason) = ''
           ) THEN
            RAISE EXCEPTION 'reopening a period requires a reason'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_reopen_reason';
        END IF;
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.audit_period_transition()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    INSERT INTO accounting.period_events (
        org_id,
        period_id,
        from_status,
        to_status,
        actor,
        reason,
        occurred_at
    )
    VALUES (
        NEW.org_id,
        NEW.id,
        OLD.status,
        NEW.status,
        NEW.status_changed_by,
        NEW.transition_reason,
        now()
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.assign_entry_number()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);

    INSERT INTO accounting.entry_sequences (org_id, next_number, updated_at)
    VALUES (NEW.org_id, 2, now())
    ON CONFLICT (org_id) DO UPDATE
       SET next_number = accounting.entry_sequences.next_number + 1,
           updated_at = now()
    RETURNING next_number - 1 INTO NEW.entry_number;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.assert_journal_entry_valid(
    target_org_id uuid,
    target_entry_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    entry_record accounting.journal_entries%ROWTYPE;
    period_status text;
    line_count integer;
    debit_total numeric(24, 6);
    credit_total numeric(24, 6);
BEGIN
    SELECT *
      INTO entry_record
      FROM accounting.journal_entries
     WHERE org_id = target_org_id
       AND id = target_entry_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT period.status
      INTO period_status
      FROM accounting.periods AS period
     WHERE period.org_id = entry_record.org_id
       AND period.id = entry_record.period_id
       AND entry_record.entry_date
           BETWEEN period.start_date AND period.end_date;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'journal entry date is outside its accounting period'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_entries_period_date';
    END IF;

    IF period_status = 'locked' THEN
        RAISE EXCEPTION 'journal entry cannot be posted to a locked period'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_entries_period_locked';
    END IF;

    IF period_status = 'soft_closed'
       AND entry_record.entry_kind NOT IN (
           'adjustment',
           'closing',
           'inflation',
           'revaluation',
           'reversal'
       ) THEN
        RAISE EXCEPTION 'only adjusting entries are allowed in a soft-closed period'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_entries_period_soft_closed';
    END IF;

    SELECT
        count(*),
        coalesce(sum(line.debit_amount), 0),
        coalesce(sum(line.credit_amount), 0)
      INTO line_count, debit_total, credit_total
      FROM accounting.journal_lines AS line
     WHERE line.org_id = target_org_id
       AND line.journal_entry_id = target_entry_id;

    IF line_count < 2 OR debit_total <= 0 OR debit_total <> credit_total THEN
        RAISE EXCEPTION 'journal entry must have at least two balanced lines'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_entries_balanced';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM accounting.journal_lines AS line
          JOIN accounting.accounts AS account
            ON account.org_id = line.org_id
           AND account.id = line.account_id
         WHERE line.org_id = target_org_id
           AND line.journal_entry_id = target_entry_id
           AND (
               NOT account.posting_allowed
               OR account.archived_at IS NOT NULL
               OR account.trashed_at IS NOT NULL
           )
    ) THEN
        RAISE EXCEPTION 'journal lines require active posting accounts'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_lines_active_posting_account';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM accounting.journal_lines AS line
         WHERE line.org_id = target_org_id
           AND line.journal_entry_id = target_entry_id
           AND (
               (
                   line.currency_code = entry_record.functional_currency
                   AND (
                       line.exchange_rate <> 1
                       OR line.currency_amount
                           <> line.debit_amount + line.credit_amount
                   )
               )
               OR
               (
                   line.currency_code <> entry_record.functional_currency
                   AND (
                       line.exchange_rate_date IS NULL
                       OR line.exchange_rate_source IS NULL
                       OR round(
                           line.currency_amount * line.exchange_rate,
                           accounting.currency_minor_units(
                               entry_record.functional_currency
                           )
                       ) <> line.debit_amount + line.credit_amount
                   )
               )
           )
    ) THEN
        RAISE EXCEPTION 'journal line currency conversion is inconsistent'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_lines_currency_conversion';
    END IF;

    IF entry_record.reverses_entry_id IS NOT NULL AND EXISTS (
        WITH original AS (
            SELECT
                line.account_id,
                line.currency_code,
                line.currency_amount,
                line.exchange_rate,
                sum(line.debit_amount) AS debit_amount,
                sum(line.credit_amount) AS credit_amount,
                count(*) AS line_count
            FROM accounting.journal_lines AS line
            WHERE line.org_id = target_org_id
              AND line.journal_entry_id = entry_record.reverses_entry_id
            GROUP BY
                line.account_id,
                line.currency_code,
                line.currency_amount,
                line.exchange_rate
        ),
        reversal AS (
            SELECT
                line.account_id,
                line.currency_code,
                line.currency_amount,
                line.exchange_rate,
                sum(line.debit_amount) AS debit_amount,
                sum(line.credit_amount) AS credit_amount,
                count(*) AS line_count
            FROM accounting.journal_lines AS line
            WHERE line.org_id = target_org_id
              AND line.journal_entry_id = target_entry_id
            GROUP BY
                line.account_id,
                line.currency_code,
                line.currency_amount,
                line.exchange_rate
        )
        SELECT 1
          FROM original
          FULL JOIN reversal USING (
              account_id,
              currency_code,
              currency_amount,
              exchange_rate
          )
         WHERE coalesce(original.debit_amount, 0)
                   <> coalesce(reversal.credit_amount, 0)
            OR coalesce(original.credit_amount, 0)
                   <> coalesce(reversal.debit_amount, 0)
            OR coalesce(original.line_count, 0)
                   <> coalesce(reversal.line_count, 0)
    ) THEN
        RAISE EXCEPTION 'reversal lines must exactly invert the original entry'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_journal_entries_exact_reversal';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.check_journal_entry_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM accounting.assert_journal_entry_valid(
        coalesce(NEW.org_id, OLD.org_id),
        coalesce(NEW.journal_entry_id, OLD.journal_entry_id, NEW.id, OLD.id)
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.check_journal_entry_row_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    PERFORM accounting.assert_journal_entry_valid(NEW.org_id, NEW.id);
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.reject_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
    RAISE EXCEPTION '% rows are immutable after insertion', TG_TABLE_NAME
        USING ERRCODE = '55000';
END
$function$;

REVOKE ALL
ON FUNCTION accounting.validate_account_hierarchy()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_account_lifecycle()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_period()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.audit_period_transition()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.assign_entry_number()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.assert_journal_entry_valid(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.check_journal_entry_constraint()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.check_journal_entry_row_constraint()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.reject_immutable_change()
FROM PUBLIC;

CREATE TRIGGER accounting_accounts_hierarchy_guard
BEFORE INSERT OR UPDATE OF parent_id, account_class, posting_allowed
ON accounting.accounts
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_account_hierarchy();

CREATE TRIGGER accounting_accounts_lifecycle_guard
BEFORE UPDATE OF posting_allowed, parent_id, trashed_at
ON accounting.accounts
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_account_lifecycle();

CREATE TRIGGER accounting_periods_guard
BEFORE INSERT OR UPDATE
ON accounting.periods
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_period();

CREATE TRIGGER accounting_periods_audit
AFTER UPDATE OF status
ON accounting.periods
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION accounting.audit_period_transition();

CREATE TRIGGER accounting_journal_entries_number
BEFORE INSERT
ON accounting.journal_entries
FOR EACH ROW
EXECUTE FUNCTION accounting.assign_entry_number();

CREATE CONSTRAINT TRIGGER accounting_journal_entries_valid
AFTER INSERT
ON accounting.journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.check_journal_entry_row_constraint();

CREATE CONSTRAINT TRIGGER accounting_journal_lines_entry_valid
AFTER INSERT OR UPDATE OR DELETE
ON accounting.journal_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.check_journal_entry_constraint();

CREATE TRIGGER accounting_journal_entries_immutable
BEFORE UPDATE OR DELETE
ON accounting.journal_entries
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_journal_lines_immutable
BEFORE UPDATE OR DELETE
ON accounting.journal_lines
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE TRIGGER accounting_period_events_immutable
BEFORE UPDATE OR DELETE
ON accounting.period_events
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

DO $rls$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'organization_settings',
        'accounts',
        'account_mappings',
        'periods',
        'period_events',
        'drafts',
        'draft_lines',
        'entry_sequences',
        'journal_entries',
        'journal_lines'
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
        GRANT USAGE ON SCHEMA app, accounting TO pymes_backend;
        GRANT EXECUTE ON FUNCTION app.current_org_id() TO pymes_backend;
        GRANT EXECUTE ON FUNCTION app.assert_org_context(uuid) TO pymes_backend;
        GRANT EXECUTE
        ON FUNCTION accounting.currency_minor_units(char)
        TO pymes_backend;

        GRANT SELECT ON
            accounting.currencies,
            accounting.chart_templates,
            accounting.chart_template_accounts,
            accounting.chart_template_mappings,
            accounting.entry_sequences,
            accounting.period_events,
            accounting.journal_entries,
            accounting.journal_lines
        TO pymes_backend;

        GRANT SELECT, INSERT, UPDATE, DELETE ON
            accounting.organization_settings,
            accounting.accounts,
            accounting.account_mappings,
            accounting.periods,
            accounting.drafts,
            accounting.draft_lines
        TO pymes_backend;

        GRANT INSERT ON
            accounting.journal_entries,
            accounting.journal_lines
        TO pymes_backend;

        GRANT SELECT, INSERT, DELETE
        ON TABLE iam.membership_permissions
        TO pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_accounting_worker'
    ) THEN
        GRANT USAGE ON SCHEMA app, accounting
        TO pymes_fiscal_accounting_worker;
        GRANT EXECUTE ON FUNCTION app.current_org_id()
        TO pymes_fiscal_accounting_worker;
        GRANT EXECUTE ON FUNCTION app.assert_org_context(uuid)
        TO pymes_fiscal_accounting_worker;
        GRANT EXECUTE
        ON FUNCTION accounting.currency_minor_units(char)
        TO pymes_fiscal_accounting_worker;

        GRANT SELECT ON
            accounting.organization_settings,
            accounting.accounts,
            accounting.account_mappings,
            accounting.periods,
            accounting.journal_entries,
            accounting.journal_lines
        TO pymes_fiscal_accounting_worker;

        GRANT INSERT ON
            accounting.journal_entries,
            accounting.journal_lines
        TO pymes_fiscal_accounting_worker;

        -- PostgreSQL requires an UPDATE privilege for SELECT ... FOR UPDATE.
        -- The posting worker locks periods to serialize against close/reopen,
        -- but it may only write the non-business timestamp column.
        GRANT UPDATE (updated_at) ON accounting.periods
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
