CREATE TABLE fiscal.purchase_vouchers (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    supplier_id text NOT NULL CHECK (btrim(supplier_id) <> ''),
    supplier_tax_id char(11) NOT NULL
        CHECK (fiscal_ar.is_valid_cuit(supplier_tax_id::text)),
    supplier_name text NOT NULL CHECK (btrim(supplier_name) <> ''),
    voucher_type integer NOT NULL CHECK (voucher_type > 0),
    point_of_sale integer NOT NULL CHECK (point_of_sale BETWEEN 1 AND 99999),
    voucher_number bigint NOT NULL CHECK (voucher_number > 0),
    issue_date date NOT NULL,
    due_date date,
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    exchange_rate numeric(24, 10) NOT NULL CHECK (exchange_rate > 0),
    exchange_rate_date date NOT NULL,
    exchange_rate_source text NOT NULL CHECK (btrim(exchange_rate_source) <> ''),
    net_amount numeric(24, 6) NOT NULL CHECK (net_amount >= 0),
    exempt_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (exempt_amount >= 0),
    non_taxed_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (non_taxed_amount >= 0),
    vat_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (vat_amount >= 0),
    other_taxes_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (other_taxes_amount >= 0),
    withholding_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (withholding_amount >= 0),
    perception_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (perception_amount >= 0),
    total_amount numeric(24, 6) NOT NULL CHECK (total_amount > 0),
    source_type text NOT NULL CHECK (btrim(source_type) <> ''),
    source_id text NOT NULL CHECK (btrim(source_id) <> ''),
    associated_purchase_voucher_id uuid,
    source_reference text,
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    canonical_json text NOT NULL CHECK (
        jsonb_typeof(canonical_json::jsonb) = 'object'
    ),
    snapshot_sha256 char(64) NOT NULL
        CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_purchase_vouchers_supplier_number_unique
        UNIQUE (
            org_id,
            environment,
            supplier_tax_id,
            voucher_type,
            point_of_sale,
            voucher_number
        ),
    CONSTRAINT fiscal_purchase_vouchers_source_unique
        UNIQUE (org_id, source_type, source_id),
    CONSTRAINT fiscal_purchase_vouchers_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CONSTRAINT fiscal_purchase_vouchers_associated_fk
        FOREIGN KEY (org_id, associated_purchase_voucher_id)
        REFERENCES fiscal.purchase_vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (due_date IS NULL OR due_date >= issue_date),
    CHECK (source_reference IS NULL OR btrim(source_reference) <> ''),
    CHECK (
        (voucher_type IN (1, 6, 11) AND associated_purchase_voucher_id IS NULL)
        OR
        (
            voucher_type IN (2, 3, 7, 8, 12, 13)
            AND associated_purchase_voucher_id IS NOT NULL
            AND associated_purchase_voucher_id <> id
        )
    ),
    CHECK (
        total_amount
        = net_amount
        + exempt_amount
        + non_taxed_amount
        + vat_amount
        + other_taxes_amount
        + perception_amount
    ),
    CHECK (
        snapshot_sha256
        = encode(
            digest(convert_to(canonical_json, 'UTF8'), 'sha256'),
            'hex'
        )
    )
);

CREATE INDEX fiscal_purchase_vouchers_period_idx
    ON fiscal.purchase_vouchers (
        org_id,
        environment,
        issue_date,
        supplier_tax_id
    );

CREATE TABLE fiscal.purchase_voucher_lines (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    purchase_voucher_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    product_reference text,
    description text NOT NULL CHECK (btrim(description) <> ''),
    quantity numeric(24, 6) NOT NULL CHECK (quantity > 0),
    unit_of_measure text NOT NULL CHECK (btrim(unit_of_measure) <> ''),
    unit_price numeric(24, 6) NOT NULL CHECK (unit_price >= 0),
    discount_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (discount_amount >= 0),
    tax_treatment text NOT NULL
        CHECK (tax_treatment IN ('taxable', 'exempt', 'non_taxed')),
    vat_rate numeric(9, 6) NOT NULL DEFAULT 0 CHECK (vat_rate >= 0),
    net_amount numeric(24, 6) NOT NULL CHECK (net_amount >= 0),
    vat_amount numeric(24, 6) NOT NULL CHECK (vat_amount >= 0),
    total_amount numeric(24, 6) NOT NULL CHECK (total_amount >= 0),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_purchase_voucher_lines_number_unique
        UNIQUE (org_id, purchase_voucher_id, line_no),
    CONSTRAINT fiscal_purchase_voucher_lines_voucher_fk
        FOREIGN KEY (org_id, purchase_voucher_id)
        REFERENCES fiscal.purchase_vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (discount_amount <= round(quantity * unit_price, 6)),
    CHECK (total_amount = net_amount + vat_amount),
    CHECK (
        tax_treatment = 'taxable'
        OR (vat_rate = 0 AND vat_amount = 0)
    )
);

CREATE TABLE fiscal.purchase_voucher_taxes (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    purchase_voucher_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    tax_type text NOT NULL CHECK (
        tax_type IN ('vat', 'tribute', 'withholding', 'perception')
    ),
    authority_code text NOT NULL CHECK (btrim(authority_code) <> ''),
    jurisdiction text,
    description text NOT NULL CHECK (btrim(description) <> ''),
    taxable_base numeric(24, 6) NOT NULL CHECK (taxable_base >= 0),
    rate numeric(9, 6) NOT NULL CHECK (rate >= 0),
    amount numeric(24, 6) NOT NULL CHECK (amount >= 0),
    creditable boolean NOT NULL DEFAULT false,
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_purchase_voucher_taxes_number_unique
        UNIQUE (org_id, purchase_voucher_id, line_no),
    CONSTRAINT fiscal_purchase_voucher_taxes_voucher_fk
        FOREIGN KEY (org_id, purchase_voucher_id)
        REFERENCES fiscal.purchase_vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (jurisdiction IS NULL OR btrim(jurisdiction) <> ''),
    CHECK (tax_type = 'vat' OR NOT creditable)
);

CREATE TABLE fiscal.purchase_voucher_accounting_links (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    purchase_voucher_id uuid NOT NULL,
    journal_entry_id uuid NOT NULL,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_purchase_voucher_accounting_links_voucher_unique
        UNIQUE (org_id, purchase_voucher_id),
    CONSTRAINT fiscal_purchase_voucher_accounting_links_entry_unique
        UNIQUE (org_id, journal_entry_id),
    CONSTRAINT fiscal_purchase_voucher_accounting_links_voucher_fk
        FOREIGN KEY (org_id, purchase_voucher_id)
        REFERENCES fiscal.purchase_vouchers(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_purchase_voucher_accounting_links_entry_fk
        FOREIGN KEY (org_id, journal_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal.iva_periods (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    period_month date NOT NULL CHECK (
        period_month = date_trunc('month', period_month)::date
    ),
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'closed', 'exported')),
    opening_balance numeric(24, 6) NOT NULL DEFAULT 0,
    closing_balance numeric(24, 6),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    status_changed_by text,
    transition_reason text,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz,
    exported_at timestamptz,
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_iva_periods_month_unique
        UNIQUE (org_id, environment, period_month),
    CHECK (
        status_changed_by IS NULL
        OR btrim(status_changed_by) <> ''
    ),
    CHECK (
        transition_reason IS NULL
        OR btrim(transition_reason) <> ''
    ),
    CHECK (
        (status = 'draft' AND closed_at IS NULL AND exported_at IS NULL)
        OR
        (status = 'closed' AND closed_at IS NOT NULL AND exported_at IS NULL)
        OR
        (status = 'exported' AND closed_at IS NOT NULL AND exported_at IS NOT NULL)
    )
);

CREATE TABLE fiscal.iva_period_items (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    iva_period_id uuid NOT NULL,
    book text NOT NULL CHECK (book IN ('sales', 'purchases')),
    voucher_id uuid,
    purchase_voucher_id uuid,
    document_sha256 char(64) NOT NULL
        CHECK (document_sha256 ~ '^[0-9a-f]{64}$'),
    net_amount numeric(24, 6) NOT NULL CHECK (net_amount >= 0),
    exempt_amount numeric(24, 6) NOT NULL CHECK (exempt_amount >= 0),
    non_taxed_amount numeric(24, 6) NOT NULL CHECK (non_taxed_amount >= 0),
    vat_debit_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (vat_debit_amount >= 0),
    vat_credit_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (vat_credit_amount >= 0),
    withholding_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (withholding_amount >= 0),
    perception_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (perception_amount >= 0),
    other_tributes_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (other_tributes_amount >= 0),
    added_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_iva_period_items_sales_unique
        UNIQUE (org_id, iva_period_id, voucher_id),
    CONSTRAINT fiscal_iva_period_items_purchases_unique
        UNIQUE (org_id, iva_period_id, purchase_voucher_id),
    CONSTRAINT fiscal_iva_period_items_period_fk
        FOREIGN KEY (org_id, iva_period_id)
        REFERENCES fiscal.iva_periods(org_id, id)
        ON DELETE CASCADE,
    CONSTRAINT fiscal_iva_period_items_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_iva_period_items_purchase_voucher_fk
        FOREIGN KEY (org_id, purchase_voucher_id)
        REFERENCES fiscal.purchase_vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        (
            book = 'sales'
            AND voucher_id IS NOT NULL
            AND purchase_voucher_id IS NULL
            AND vat_credit_amount = 0
        )
        OR
        (
            book = 'purchases'
            AND purchase_voucher_id IS NOT NULL
            AND voucher_id IS NULL
            AND vat_debit_amount = 0
        )
    )
);

CREATE TABLE fiscal.iva_period_events (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    iva_period_id uuid NOT NULL,
    from_status text NOT NULL CHECK (
        from_status IN ('draft', 'closed', 'exported')
    ),
    to_status text NOT NULL CHECK (
        to_status IN ('draft', 'closed', 'exported')
    ),
    actor text NOT NULL CHECK (btrim(actor) <> ''),
    reason text,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_iva_period_events_period_fk
        FOREIGN KEY (org_id, iva_period_id)
        REFERENCES fiscal.iva_periods(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal.iva_exports (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    iva_period_id uuid NOT NULL,
    export_type text NOT NULL CHECK (
        export_type IN (
            'iva_simple_sales',
            'iva_simple_purchases',
            'iva_simple_rates',
            'workpaper'
        )
    ),
    export_version integer NOT NULL DEFAULT 1 CHECK (export_version > 0),
    storage_ref text NOT NULL CHECK (btrim(storage_ref) <> ''),
    filename text NOT NULL CHECK (btrim(filename) <> ''),
    media_type text NOT NULL DEFAULT 'application/zip'
        CHECK (media_type = 'application/zip'),
    artifact bytea NOT NULL CHECK (octet_length(artifact) > 0),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    validation_result jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_iva_exports_version_unique
        UNIQUE (org_id, iva_period_id, export_type, export_version),
    CONSTRAINT fiscal_iva_exports_period_fk
        FOREIGN KEY (org_id, iva_period_id)
        REFERENCES fiscal.iva_periods(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        sha256 = encode(digest(artifact, 'sha256'), 'hex')
    )
);

CREATE OR REPLACE FUNCTION fiscal.assert_purchase_voucher_valid(
    target_org_id uuid,
    target_voucher_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    voucher_record fiscal.purchase_vouchers%ROWTYPE;
    line_count integer;
    taxable_total numeric(24, 6);
    exempt_total numeric(24, 6);
    non_taxed_total numeric(24, 6);
    line_vat_total numeric(24, 6);
    line_total numeric(24, 6);
    tax_vat_total numeric(24, 6);
    other_tax_total numeric(24, 6);
    withholding_total numeric(24, 6);
    perception_total numeric(24, 6);
BEGIN
    SELECT *
      INTO voucher_record
      FROM fiscal.purchase_vouchers
     WHERE org_id = target_org_id
       AND id = target_voucher_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT
        count(*),
        coalesce(sum(line.net_amount)
            FILTER (WHERE line.tax_treatment = 'taxable'), 0),
        coalesce(sum(line.net_amount)
            FILTER (WHERE line.tax_treatment = 'exempt'), 0),
        coalesce(sum(line.net_amount)
            FILTER (WHERE line.tax_treatment = 'non_taxed'), 0),
        coalesce(sum(line.vat_amount), 0),
        coalesce(sum(line.total_amount), 0)
      INTO
        line_count,
        taxable_total,
        exempt_total,
        non_taxed_total,
        line_vat_total,
        line_total
      FROM fiscal.purchase_voucher_lines AS line
     WHERE line.org_id = target_org_id
       AND line.purchase_voucher_id = target_voucher_id;

    SELECT
        coalesce(sum(tax.amount)
            FILTER (WHERE tax.tax_type = 'vat'), 0),
        coalesce(sum(tax.amount)
            FILTER (WHERE tax.tax_type = 'tribute'), 0),
        coalesce(sum(tax.amount)
            FILTER (WHERE tax.tax_type = 'withholding'), 0),
        coalesce(sum(tax.amount)
            FILTER (WHERE tax.tax_type = 'perception'), 0)
      INTO
        tax_vat_total,
        other_tax_total,
        withholding_total,
        perception_total
      FROM fiscal.purchase_voucher_taxes AS tax
     WHERE tax.org_id = target_org_id
       AND tax.purchase_voucher_id = target_voucher_id;

    IF line_count = 0
       OR taxable_total <> voucher_record.net_amount
       OR exempt_total <> voucher_record.exempt_amount
       OR non_taxed_total <> voucher_record.non_taxed_amount
       OR line_vat_total <> voucher_record.vat_amount
       OR line_total
            + voucher_record.other_taxes_amount
            + voucher_record.perception_amount
            <> voucher_record.total_amount
       OR tax_vat_total <> voucher_record.vat_amount
       OR other_tax_total <> voucher_record.other_taxes_amount
       OR withholding_total <> voucher_record.withholding_amount
       OR perception_total <> voucher_record.perception_amount THEN
        RAISE EXCEPTION 'purchase voucher detail does not reconcile to totals'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_purchase_vouchers_totals';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.check_purchase_voucher_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
BEGIN
    PERFORM fiscal.assert_purchase_voucher_valid(
        coalesce(
            (to_jsonb(NEW) ->> 'org_id')::uuid,
            (to_jsonb(OLD) ->> 'org_id')::uuid
        ),
        CASE
            WHEN TG_TABLE_NAME = 'purchase_vouchers'
                THEN coalesce(
                    (to_jsonb(NEW) ->> 'id')::uuid,
                    (to_jsonb(OLD) ->> 'id')::uuid
                )
            ELSE coalesce(
                (to_jsonb(NEW) ->> 'purchase_voucher_id')::uuid,
                (to_jsonb(OLD) ->> 'purchase_voucher_id')::uuid
            )
        END
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.guard_iva_period_item()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    period_status text;
    period_environment text;
    period_month date;
BEGIN
    SELECT period.environment, period.period_month
      INTO period_environment, period_month
      FROM fiscal.iva_periods AS period
     WHERE period.org_id = coalesce(NEW.org_id, OLD.org_id)
       AND period.id = coalesce(NEW.iva_period_id, OLD.iva_period_id);

    PERFORM pg_advisory_xact_lock(hashtextextended(
        coalesce(NEW.org_id, OLD.org_id)::text
        || ':' || period_environment
        || ':' || to_char(period_month, 'YYYY-MM'),
        0
    ));

    SELECT period.status
      INTO period_status
      FROM fiscal.iva_periods AS period
     WHERE period.org_id = coalesce(NEW.org_id, OLD.org_id)
       AND period.id = coalesce(NEW.iva_period_id, OLD.iva_period_id)
     FOR UPDATE;

    IF period_status <> 'draft' THEN
        RAISE EXCEPTION 'items of a closed IVA period are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.guard_iva_export()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    period_status text;
    period_environment text;
    period_month date;
BEGIN
    SELECT period.environment, period.period_month
      INTO period_environment, period_month
      FROM fiscal.iva_periods AS period
     WHERE period.org_id = NEW.org_id
       AND period.id = NEW.iva_period_id;

    PERFORM pg_advisory_xact_lock(hashtextextended(
        NEW.org_id::text
        || ':' || period_environment
        || ':' || to_char(period_month, 'YYYY-MM'),
        0
    ));

    SELECT period.status
      INTO period_status
      FROM fiscal.iva_periods AS period
     WHERE period.org_id = NEW.org_id
       AND period.id = NEW.iva_period_id
     FOR UPDATE;

    IF period_status IS DISTINCT FROM 'closed' THEN
        RAISE EXCEPTION 'IVA exports require a closed period'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.guard_iva_period_document_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    target_environment text;
    target_issue_date date;
    target_status text;
    target_period_status text;
BEGIN
    target_environment := to_jsonb(NEW) ->> 'environment';
    target_issue_date := (to_jsonb(NEW) ->> 'issue_date')::date;
    target_status := coalesce(to_jsonb(NEW) ->> 'status', 'recorded');

    PERFORM pg_advisory_xact_lock(hashtextextended(
        (to_jsonb(NEW) ->> 'org_id')
        || ':' || target_environment
        || ':' || to_char(target_issue_date, 'YYYY-MM'),
        0
    ));

    IF TG_TABLE_NAME = 'vouchers' AND TG_OP = 'UPDATE'
       AND (to_jsonb(OLD) ->> 'environment') IS NOT DISTINCT FROM target_environment
       AND (to_jsonb(OLD) ->> 'issue_date') IS NOT DISTINCT FROM
           (to_jsonb(NEW) ->> 'issue_date')
       AND (to_jsonb(OLD) ->> 'status') IS NOT DISTINCT FROM target_status THEN
        RETURN NEW;
    END IF;

    IF target_status = 'rejected' THEN
        RETURN NEW;
    END IF;

    SELECT period.status
      INTO target_period_status
      FROM fiscal.iva_periods AS period
     WHERE period.org_id = (to_jsonb(NEW) ->> 'org_id')::uuid
       AND period.environment = target_environment
       AND period.period_month
           = date_trunc('month', target_issue_date)::date
     FOR UPDATE;

    IF target_period_status IS NOT NULL
       AND target_period_status <> 'draft' THEN
        RAISE EXCEPTION 'fiscal documents require an open IVA period'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.iva_entry_account_effect(
    target_org_id uuid,
    target_entry_id uuid,
    mapping_prefix text,
    cutoff_date date,
    credit_positive boolean
)
RETURNS numeric
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, fiscal, accounting
AS $function$
    WITH RECURSIVE entry_chain AS (
        SELECT entry.id, entry.entry_date
          FROM accounting.journal_entries AS entry
         WHERE entry.org_id = target_org_id
           AND entry.id = target_entry_id
        UNION ALL
        SELECT reversal.id, reversal.entry_date
          FROM entry_chain AS prior
          JOIN accounting.journal_entries AS reversal
            ON reversal.org_id = target_org_id
           AND reversal.reverses_entry_id = prior.id
    )
    SELECT coalesce(sum(
        CASE
            WHEN credit_positive
                THEN line.credit_amount - line.debit_amount
            ELSE line.debit_amount - line.credit_amount
        END
    ), 0)
      FROM entry_chain AS entry
      JOIN accounting.journal_lines AS line
        ON line.org_id = target_org_id
       AND line.journal_entry_id = entry.id
     WHERE entry.entry_date < cutoff_date
       AND line.account_id IN (
           SELECT mapping.account_id
             FROM accounting.account_mappings AS mapping
            WHERE mapping.org_id = target_org_id
              AND mapping.mapping_key LIKE mapping_prefix || '%'
       )
$function$;

CREATE OR REPLACE FUNCTION fiscal.assert_iva_period_valid(
    target_org_id uuid,
    target_period_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    period_record fiscal.iva_periods%ROWTYPE;
    computed_closing_balance numeric(24, 6);
BEGIN
    SELECT *
      INTO period_record
      FROM fiscal.iva_periods
     WHERE org_id = target_org_id
       AND id = target_period_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM fiscal.vouchers AS voucher
         WHERE voucher.org_id = period_record.org_id
           AND voucher.environment = period_record.environment
           AND date_trunc('month', voucher.issue_date)::date
               = period_record.period_month
           AND voucher.status IN ('queued', 'processing', 'uncertain')
    ) THEN
        RAISE EXCEPTION 'IVA period has pending fiscal authorizations'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_pending_authorizations';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM fiscal.vouchers AS voucher
         WHERE voucher.org_id = period_record.org_id
           AND voucher.environment = period_record.environment
           AND voucher.status = 'authorized'
           AND date_trunc('month', voucher.issue_date)::date
               = period_record.period_month
           AND NOT EXISTS (
               SELECT 1
                 FROM fiscal.iva_period_items AS item
                WHERE item.org_id = voucher.org_id
                  AND item.iva_period_id = period_record.id
                  AND item.voucher_id = voucher.id
           )
    ) OR EXISTS (
        SELECT 1
          FROM fiscal.purchase_vouchers AS purchase
         WHERE purchase.org_id = period_record.org_id
           AND purchase.environment = period_record.environment
           AND date_trunc('month', purchase.issue_date)::date
               = period_record.period_month
           AND NOT EXISTS (
               SELECT 1
                 FROM fiscal.iva_period_items AS item
                WHERE item.org_id = purchase.org_id
                  AND item.iva_period_id = period_record.id
                  AND item.purchase_voucher_id = purchase.id
           )
    ) THEN
        RAISE EXCEPTION 'IVA period is missing fiscal documents'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_complete';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM fiscal.iva_period_items AS item
          LEFT JOIN fiscal.vouchers AS voucher
            ON voucher.org_id = item.org_id
           AND voucher.id = item.voucher_id
          LEFT JOIN fiscal.voucher_snapshots AS snapshot
            ON snapshot.org_id = voucher.org_id
           AND snapshot.voucher_id = voucher.id
          LEFT JOIN fiscal.purchase_vouchers AS purchase
            ON purchase.org_id = item.org_id
           AND purchase.id = item.purchase_voucher_id
         WHERE item.org_id = period_record.org_id
           AND item.iva_period_id = period_record.id
           AND (
               (
                   item.book = 'sales'
                   AND (
                       voucher.status <> 'authorized'
                       OR item.document_sha256 <> snapshot.snapshot_sha256
                       OR item.net_amount <> voucher.net_amount
                       OR item.exempt_amount <> voucher.exempt_amount
                       OR item.non_taxed_amount <> voucher.non_taxed_amount
                       OR item.vat_debit_amount <> voucher.vat_amount
                   )
               )
               OR
               (
                   item.book = 'purchases'
                   AND (
                       item.document_sha256 <> purchase.snapshot_sha256
                       OR item.net_amount <> purchase.net_amount
                       OR item.exempt_amount <> purchase.exempt_amount
                       OR item.non_taxed_amount <> purchase.non_taxed_amount
                       OR item.vat_credit_amount <> coalesce((
                           SELECT sum(tax.amount)
                             FROM fiscal.purchase_voucher_taxes AS tax
                            WHERE tax.org_id = purchase.org_id
                              AND tax.purchase_voucher_id = purchase.id
                              AND tax.tax_type = 'vat'
                              AND tax.creditable
                       ), 0)
                   )
               )
           )
    ) THEN
        RAISE EXCEPTION 'IVA period items do not match immutable documents'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_period_items_reconcile';
    END IF;

    IF EXISTS (
        SELECT 1
          FROM fiscal.iva_period_items AS item
          LEFT JOIN fiscal.vouchers AS voucher
            ON voucher.org_id = item.org_id
           AND voucher.id = item.voucher_id
          LEFT JOIN fiscal.purchase_vouchers AS purchase
            ON purchase.org_id = item.org_id
           AND purchase.id = item.purchase_voucher_id
         WHERE item.org_id = period_record.org_id
           AND item.iva_period_id = period_record.id
           AND (
               (
                   item.book = 'sales'
                   AND (
                       NOT EXISTS (
                           SELECT 1
                             FROM fiscal.voucher_accounting_links AS link
                             JOIN accounting.journal_entries AS entry
                               ON entry.org_id = link.org_id
                              AND entry.id = link.journal_entry_id
                            WHERE link.org_id = item.org_id
                              AND link.voucher_id = item.voucher_id
                              AND entry.entry_date = voucher.issue_date
                       )
                       OR fiscal.iva_entry_account_effect(
                           item.org_id,
                           (
                               SELECT link.journal_entry_id
                                 FROM fiscal.voucher_accounting_links AS link
                                WHERE link.org_id = item.org_id
                                  AND link.voucher_id = item.voucher_id
                           ),
                           'vat_payable_',
                           (
                               period_record.period_month
                               + interval '1 month'
                           )::date,
                           true
                       ) <> round(
                           item.vat_debit_amount
                           * voucher.exchange_rate
                           * CASE
                               WHEN voucher.voucher_type IN (3, 8, 13)
                                   THEN -1
                               ELSE 1
                             END,
                           accounting.currency_minor_units(
                               (
                                   SELECT entry.functional_currency
                                     FROM fiscal.voucher_accounting_links AS link
                                     JOIN accounting.journal_entries AS entry
                                       ON entry.org_id = link.org_id
                                      AND entry.id = link.journal_entry_id
                                    WHERE link.org_id = item.org_id
                                      AND link.voucher_id = item.voucher_id
                               )
                           )
                       )
                   )
               )
               OR
               (
                   item.book = 'purchases'
                   AND (
                       NOT EXISTS (
                           SELECT 1
                             FROM fiscal.purchase_voucher_accounting_links
                                  AS link
                             JOIN accounting.journal_entries AS entry
                               ON entry.org_id = link.org_id
                              AND entry.id = link.journal_entry_id
                            WHERE link.org_id = item.org_id
                              AND link.purchase_voucher_id
                                  = item.purchase_voucher_id
                              AND entry.entry_date = purchase.issue_date
                       )
                       OR fiscal.iva_entry_account_effect(
                           item.org_id,
                           (
                               SELECT link.journal_entry_id
                                 FROM fiscal.purchase_voucher_accounting_links
                                      AS link
                                WHERE link.org_id = item.org_id
                                  AND link.purchase_voucher_id
                                      = item.purchase_voucher_id
                           ),
                           'vat_credit_',
                           (
                               period_record.period_month
                               + interval '1 month'
                           )::date,
                           false
                       ) <> round(
                           item.vat_credit_amount
                           * purchase.exchange_rate
                           * CASE
                               WHEN purchase.voucher_type IN (3, 8, 13)
                                   THEN -1
                               ELSE 1
                             END,
                           accounting.currency_minor_units(
                               (
                                   SELECT entry.functional_currency
                                     FROM fiscal.purchase_voucher_accounting_links
                                          AS link
                                     JOIN accounting.journal_entries AS entry
                                       ON entry.org_id = link.org_id
                                      AND entry.id = link.journal_entry_id
                                    WHERE link.org_id = item.org_id
                                      AND link.purchase_voucher_id
                                          = item.purchase_voucher_id
                               )
                           )
                       )
                   )
               )
           )
    ) THEN
        RAISE EXCEPTION 'IVA period does not reconcile to accounting links'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_accounting_reconcile';
    END IF;

    SELECT (
        period_record.opening_balance
        + coalesce(sum(
            item.vat_credit_amount
            * CASE
                WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                    IN (3, 8, 13)
                THEN -1
                ELSE 1
              END
        ), 0)
        + coalesce(sum(
            item.withholding_amount
            * CASE
                WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                    IN (3, 8, 13)
                THEN -1
                ELSE 1
              END
        ), 0)
        + coalesce(sum(
            item.perception_amount
            * CASE
                WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                    IN (3, 8, 13)
                THEN -1
                ELSE 1
              END
        ), 0)
        - coalesce(sum(
            item.vat_debit_amount
            * CASE
                WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                    IN (3, 8, 13)
                THEN -1
                ELSE 1
              END
        ), 0)
    )::numeric(24, 6)
      INTO computed_closing_balance
      FROM fiscal.iva_period_items AS item
      LEFT JOIN fiscal.vouchers AS voucher
        ON voucher.org_id = item.org_id
       AND voucher.id = item.voucher_id
      LEFT JOIN fiscal.purchase_vouchers AS purchase
        ON purchase.org_id = item.org_id
       AND purchase.id = item.purchase_voucher_id
     WHERE item.org_id = period_record.org_id
       AND item.iva_period_id = period_record.id;

    IF period_record.closing_balance IS DISTINCT FROM computed_closing_balance THEN
        RAISE EXCEPTION 'IVA closing balance does not reconcile'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_closing_balance';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.validate_iva_period_transition()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(
        coalesce(
            (to_jsonb(NEW) ->> 'org_id'),
            (to_jsonb(OLD) ->> 'org_id')
        )
        || ':' || coalesce(
            (to_jsonb(NEW) ->> 'environment'),
            (to_jsonb(OLD) ->> 'environment')
        )
        || ':' || to_char(
            coalesce(
                (to_jsonb(NEW) ->> 'period_month')::date,
                (to_jsonb(OLD) ->> 'period_month')::date
            ),
            'YYYY-MM'
        ),
        0
    ));

    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'draft' THEN
            RAISE EXCEPTION 'a new IVA period must be a draft'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'fiscal_iva_periods_initially_draft';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        IF OLD.status <> 'draft' THEN
            RAISE EXCEPTION 'a closed IVA period cannot be deleted'
                USING ERRCODE = '55000';
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.status IS NOT DISTINCT FROM NEW.status THEN
        IF OLD.status <> 'draft' AND NEW IS DISTINCT FROM OLD THEN
            RAISE EXCEPTION 'a closed IVA period is immutable'
                USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;

    IF NOT (
        (OLD.status = 'draft' AND NEW.status = 'closed')
        OR
        (OLD.status = 'closed' AND NEW.status IN ('draft', 'exported'))
        OR
        (OLD.status = 'exported' AND NEW.status = 'draft')
    ) THEN
        RAISE EXCEPTION 'invalid IVA period transition % -> %',
            OLD.status,
            NEW.status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_transition';
    END IF;

    IF NEW.version <> OLD.version + 1
       OR NEW.status_changed_by IS NULL
       OR btrim(NEW.status_changed_by) = '' THEN
        RAISE EXCEPTION 'IVA period transition requires actor and next version'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_transition_metadata';
    END IF;

    IF (
           (OLD.status IN ('closed', 'exported') AND NEW.status = 'draft')
           OR NEW.status = 'closed'
       )
       AND (
           NEW.transition_reason IS NULL
           OR btrim(NEW.transition_reason) = ''
       ) THEN
        RAISE EXCEPTION 'closing or reopening an IVA period requires a reason'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_transition_reason';
    END IF;

    IF NEW.status IN ('closed', 'exported') THEN
        PERFORM fiscal.assert_iva_period_valid(NEW.org_id, NEW.id);
    END IF;

    IF NEW.status = 'exported' AND NOT EXISTS (
        SELECT 1
          FROM fiscal.iva_exports AS export
         WHERE export.org_id = NEW.org_id
           AND export.iva_period_id = NEW.id
    ) THEN
        RAISE EXCEPTION 'IVA period requires an export artifact'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_iva_periods_export_required';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.audit_iva_period_transition()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
BEGIN
    INSERT INTO fiscal.iva_period_events (
        org_id,
        iva_period_id,
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

REVOKE ALL
ON FUNCTION fiscal.assert_purchase_voucher_valid(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.check_purchase_voucher_constraint()
FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.guard_iva_period_item() FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.guard_iva_export() FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.guard_iva_period_document_change()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.iva_entry_account_effect(uuid, uuid, text, date, boolean)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.assert_iva_period_valid(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.validate_iva_period_transition()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.audit_iva_period_transition()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER fiscal_purchase_vouchers_valid
AFTER INSERT OR UPDATE ON fiscal.purchase_vouchers
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_purchase_voucher_constraint();

CREATE CONSTRAINT TRIGGER fiscal_purchase_voucher_lines_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.purchase_voucher_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_purchase_voucher_constraint();

CREATE CONSTRAINT TRIGGER fiscal_purchase_voucher_taxes_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.purchase_voucher_taxes
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_purchase_voucher_constraint();

CREATE TRIGGER fiscal_purchase_vouchers_immutable
BEFORE UPDATE OR DELETE ON fiscal.purchase_vouchers
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_purchase_voucher_lines_immutable
BEFORE UPDATE OR DELETE ON fiscal.purchase_voucher_lines
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_purchase_voucher_taxes_immutable
BEFORE UPDATE OR DELETE ON fiscal.purchase_voucher_taxes
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_purchase_voucher_accounting_links_immutable
BEFORE UPDATE OR DELETE ON fiscal.purchase_voucher_accounting_links
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_iva_period_items_guard
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.iva_period_items
FOR EACH ROW
EXECUTE FUNCTION fiscal.guard_iva_period_item();

CREATE TRIGGER fiscal_iva_exports_guard
BEFORE INSERT ON fiscal.iva_exports
FOR EACH ROW
EXECUTE FUNCTION fiscal.guard_iva_export();

CREATE TRIGGER fiscal_vouchers_iva_period_guard
BEFORE INSERT OR UPDATE ON fiscal.vouchers
FOR EACH ROW
EXECUTE FUNCTION fiscal.guard_iva_period_document_change();

CREATE TRIGGER fiscal_purchase_vouchers_iva_period_guard
BEFORE INSERT ON fiscal.purchase_vouchers
FOR EACH ROW
EXECUTE FUNCTION fiscal.guard_iva_period_document_change();

CREATE TRIGGER fiscal_iva_periods_transition
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.iva_periods
FOR EACH ROW
EXECUTE FUNCTION fiscal.validate_iva_period_transition();

CREATE TRIGGER fiscal_iva_periods_audit
AFTER UPDATE OF status ON fiscal.iva_periods
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION fiscal.audit_iva_period_transition();

CREATE TRIGGER fiscal_iva_period_events_immutable
BEFORE UPDATE OR DELETE ON fiscal.iva_period_events
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_iva_exports_immutable
BEFORE UPDATE OR DELETE ON fiscal.iva_exports
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE VIEW fiscal.iva_sales_book_view
WITH (security_invoker = true)
AS
SELECT
    voucher.org_id,
    voucher.environment,
    voucher.id AS voucher_id,
    voucher.issue_date,
    voucher.voucher_type,
    point_of_sale.code AS point_of_sale,
    voucher.voucher_number,
    snapshot.recipient_document_type,
    snapshot.recipient_document_number,
    snapshot.recipient_name,
    voucher.currency_code,
    voucher.exchange_rate,
    voucher.net_amount,
    voucher.exempt_amount,
    voucher.non_taxed_amount,
    voucher.vat_amount,
    voucher.other_tributes_amount,
    voucher.total_amount,
    snapshot.snapshot_sha256,
    accounting_link.journal_entry_id
FROM fiscal.vouchers AS voucher
JOIN fiscal.points_of_sale AS point_of_sale
  ON point_of_sale.org_id = voucher.org_id
 AND point_of_sale.id = voucher.point_of_sale_id
JOIN fiscal.voucher_snapshots AS snapshot
  ON snapshot.org_id = voucher.org_id
 AND snapshot.voucher_id = voucher.id
LEFT JOIN fiscal.voucher_accounting_links AS accounting_link
  ON accounting_link.org_id = voucher.org_id
 AND accounting_link.voucher_id = voucher.id
WHERE voucher.status = 'authorized';

CREATE VIEW fiscal.iva_purchase_book_view
WITH (security_invoker = true)
AS
SELECT
    purchase.org_id,
    purchase.environment,
    purchase.id AS purchase_voucher_id,
    purchase.issue_date,
    purchase.supplier_tax_id,
    purchase.supplier_name,
    purchase.voucher_type,
    purchase.point_of_sale,
    purchase.voucher_number,
    purchase.currency_code,
    purchase.exchange_rate,
    purchase.net_amount,
    purchase.exempt_amount,
    purchase.non_taxed_amount,
    purchase.vat_amount,
    coalesce((
        SELECT sum(tax.amount)
          FROM fiscal.purchase_voucher_taxes AS tax
         WHERE tax.org_id = purchase.org_id
           AND tax.purchase_voucher_id = purchase.id
           AND tax.tax_type = 'vat'
           AND tax.creditable
    ), 0)::numeric(24, 6) AS creditable_vat_amount,
    purchase.other_taxes_amount,
    purchase.withholding_amount,
    purchase.perception_amount,
    purchase.total_amount,
    purchase.snapshot_sha256,
    accounting_link.journal_entry_id
FROM fiscal.purchase_vouchers AS purchase
LEFT JOIN fiscal.purchase_voucher_accounting_links AS accounting_link
  ON accounting_link.org_id = purchase.org_id
 AND accounting_link.purchase_voucher_id = purchase.id;

CREATE VIEW fiscal.iva_position_view
WITH (security_invoker = true)
AS
SELECT
    period.org_id,
    period.id AS iva_period_id,
    period.environment,
    period.period_month,
    period.status,
    period.opening_balance,
    coalesce(sum(
        item.vat_debit_amount
        * CASE
            WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                IN (3, 8, 13)
            THEN -1
            ELSE 1
          END
    ), 0)::numeric(24, 6)
        AS vat_debit_amount,
    coalesce(sum(
        item.vat_credit_amount
        * CASE
            WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                IN (3, 8, 13)
            THEN -1
            ELSE 1
          END
    ), 0)::numeric(24, 6)
        AS vat_credit_amount,
    coalesce(sum(
        item.withholding_amount
        * CASE
            WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                IN (3, 8, 13)
            THEN -1
            ELSE 1
          END
    ), 0)::numeric(24, 6)
        AS withholding_amount,
    coalesce(sum(
        item.perception_amount
        * CASE
            WHEN coalesce(voucher.voucher_type, purchase.voucher_type)
                IN (3, 8, 13)
            THEN -1
            ELSE 1
          END
    ), 0)::numeric(24, 6)
        AS perception_amount,
    period.closing_balance
FROM fiscal.iva_periods AS period
LEFT JOIN fiscal.iva_period_items AS item
  ON item.org_id = period.org_id
 AND item.iva_period_id = period.id
LEFT JOIN fiscal.vouchers AS voucher
  ON voucher.org_id = item.org_id
 AND voucher.id = item.voucher_id
LEFT JOIN fiscal.purchase_vouchers AS purchase
  ON purchase.org_id = item.org_id
 AND purchase.id = item.purchase_voucher_id
GROUP BY
    period.org_id,
    period.id,
    period.environment,
    period.period_month,
    period.status,
    period.opening_balance,
    period.closing_balance;

REVOKE ALL ON
    fiscal.iva_sales_book_view,
    fiscal.iva_purchase_book_view,
    fiscal.iva_position_view
FROM PUBLIC;

DO $rls$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'purchase_vouchers',
        'purchase_voucher_lines',
        'purchase_voucher_taxes',
        'purchase_voucher_accounting_links',
        'iva_periods',
        'iva_period_items',
        'iva_period_events',
        'iva_exports'
    ]
    LOOP
        EXECUTE format(
            'ALTER TABLE fiscal.%I ENABLE ROW LEVEL SECURITY',
            table_name
        );
        EXECUTE format(
            'ALTER TABLE fiscal.%I FORCE ROW LEVEL SECURITY',
            table_name
        );
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON fiscal.%I
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

REVOKE ALL ON ALL TABLES IN SCHEMA fiscal FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT, INSERT ON
            fiscal.purchase_vouchers,
            fiscal.purchase_voucher_lines,
            fiscal.purchase_voucher_taxes,
            fiscal.purchase_voucher_accounting_links,
            fiscal.iva_period_events,
            fiscal.iva_exports
        TO pymes_backend;

        GRANT SELECT, INSERT, UPDATE, DELETE ON
            fiscal.iva_periods,
            fiscal.iva_period_items
        TO pymes_backend;

        GRANT SELECT ON
            fiscal.iva_sales_book_view,
            fiscal.iva_purchase_book_view,
            fiscal.iva_position_view
        TO pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_roles WHERE rolname = 'pymes_fiscal_worker'
    ) THEN
        GRANT SELECT ON
            fiscal.purchase_vouchers,
            fiscal.purchase_voucher_lines,
            fiscal.purchase_voucher_taxes,
            fiscal.purchase_voucher_accounting_links,
            fiscal.iva_periods,
            fiscal.iva_period_items,
            fiscal.iva_period_events,
            fiscal.iva_exports,
            fiscal.iva_sales_book_view,
            fiscal.iva_purchase_book_view,
            fiscal.iva_position_view
        TO pymes_fiscal_worker;
    END IF;
END
$grant$;
