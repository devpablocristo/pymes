-- Fiscal years group the monthly accounting periods introduced by the
-- accounting MVP. Historical ranges remain readable as legacy periods; all
-- newly provisioned fiscal years are made of twelve contiguous calendar
-- months.

CREATE TABLE accounting.fiscal_years (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    code text NOT NULL CHECK (btrim(code) <> ''),
    start_date date NOT NULL,
    end_date date NOT NULL,
    is_legacy boolean NOT NULL DEFAULT false,
    annual_close_status text NOT NULL DEFAULT 'not_ready'
        CHECK (
            annual_close_status IN (
                'not_ready',
                'ready',
                'draft',
                'posted',
                'reversed',
                'not_required'
            )
        ),
    annual_close_draft_id uuid,
    annual_close_entry_id uuid,
    annual_close_reversal_entry_id uuid,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    annual_close_changed_by text,
    transition_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_fiscal_years_code_unique
        UNIQUE (org_id, code),
    CONSTRAINT accounting_fiscal_years_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CONSTRAINT accounting_fiscal_years_dates_unique
        UNIQUE (org_id, start_date, end_date),
    CONSTRAINT accounting_fiscal_years_twelve_months CHECK (
        start_date = date_trunc('month', start_date)::date
        AND end_date = (start_date + interval '1 year - 1 day')::date
    ),
    CONSTRAINT accounting_fiscal_years_draft_fk
        FOREIGN KEY (org_id, annual_close_draft_id)
        REFERENCES accounting.drafts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_fiscal_years_entry_fk
        FOREIGN KEY (org_id, annual_close_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_fiscal_years_reversal_fk
        FOREIGN KEY (org_id, annual_close_reversal_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_fiscal_years_annual_close_shape CHECK (
        (
            annual_close_status IN ('not_ready', 'ready', 'not_required')
            AND annual_close_draft_id IS NULL
            AND annual_close_entry_id IS NULL
            AND annual_close_reversal_entry_id IS NULL
        )
        OR
        (
            annual_close_status = 'draft'
            AND annual_close_draft_id IS NOT NULL
            AND annual_close_entry_id IS NULL
            AND annual_close_reversal_entry_id IS NULL
        )
        OR
        (
            annual_close_status = 'posted'
            AND annual_close_entry_id IS NOT NULL
            AND annual_close_reversal_entry_id IS NULL
        )
        OR
        (
            annual_close_status = 'reversed'
            AND annual_close_entry_id IS NOT NULL
            AND annual_close_reversal_entry_id IS NOT NULL
        )
    ),
    CHECK (
        annual_close_changed_by IS NULL
        OR btrim(annual_close_changed_by) <> ''
    ),
    CHECK (
        transition_reason IS NULL
        OR btrim(transition_reason) <> ''
    )
);

CREATE INDEX accounting_fiscal_years_lookup_idx
    ON accounting.fiscal_years (org_id, start_date DESC, id DESC);

CREATE TABLE accounting.fiscal_year_events (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    fiscal_year_id uuid NOT NULL,
    event_type text NOT NULL CHECK (
        event_type IN (
            'created',
            'annual_close_transition',
            'calendar_changed'
        )
    ),
    from_status text CHECK (
        from_status IS NULL
        OR from_status IN (
            'not_ready',
            'ready',
            'draft',
            'posted',
            'reversed',
            'not_required'
        )
    ),
    to_status text NOT NULL CHECK (
        to_status IN (
            'not_ready',
            'ready',
            'draft',
            'posted',
            'reversed',
            'not_required'
        )
    ),
    from_version bigint CHECK (from_version IS NULL OR from_version > 0),
    to_version bigint NOT NULL CHECK (to_version > 0),
    actor text NOT NULL CHECK (btrim(actor) <> ''),
    reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_fiscal_year_events_year_fk
        FOREIGN KEY (org_id, fiscal_year_id)
        REFERENCES accounting.fiscal_years(org_id, id)
        ON DELETE RESTRICT,
    CHECK (reason IS NULL OR btrim(reason) <> '')
);

CREATE INDEX accounting_fiscal_year_events_year_idx
    ON accounting.fiscal_year_events (
        org_id,
        fiscal_year_id,
        occurred_at DESC,
        id DESC
    );

ALTER TABLE accounting.organization_settings
    ADD COLUMN fiscal_calendar_idempotency_key text,
    ADD COLUMN fiscal_calendar_changed_by text,
    ADD CONSTRAINT accounting_settings_calendar_key_check CHECK (
        fiscal_calendar_idempotency_key IS NULL
        OR btrim(fiscal_calendar_idempotency_key) <> ''
    ),
    ADD CONSTRAINT accounting_settings_calendar_actor_check CHECK (
        fiscal_calendar_changed_by IS NULL
        OR btrim(fiscal_calendar_changed_by) <> ''
    );

ALTER TABLE accounting.periods
    ADD COLUMN fiscal_year_id uuid,
    ADD COLUMN period_no smallint,
    ADD COLUMN is_legacy boolean NOT NULL DEFAULT false;

ALTER TABLE accounting.period_events
    ADD COLUMN from_version bigint,
    ADD COLUMN to_version bigint,
    ADD COLUMN idempotency_key text,
    ADD CONSTRAINT accounting_period_events_from_version_check CHECK (
        from_version IS NULL OR from_version > 0
    ),
    ADD CONSTRAINT accounting_period_events_to_version_check CHECK (
        to_version IS NULL OR to_version > 0
    ),
    ADD CONSTRAINT accounting_period_events_idempotency_key_check CHECK (
        idempotency_key IS NULL OR btrim(idempotency_key) <> ''
    );

ALTER TABLE accounting.period_events
    DISABLE TRIGGER accounting_period_events_immutable;

WITH ordered_events AS (
    SELECT
        event.org_id,
        event.id,
        row_number() OVER (
            PARTITION BY event.org_id, event.period_id
            ORDER BY event.occurred_at, event.id
        )::bigint AS transition_number
      FROM accounting.period_events AS event
)
UPDATE accounting.period_events AS event
   SET from_version = ordered.transition_number,
       to_version = ordered.transition_number + 1
  FROM ordered_events AS ordered
 WHERE ordered.org_id = event.org_id
   AND ordered.id = event.id;

ALTER TABLE accounting.period_events
    ENABLE TRIGGER accounting_period_events_immutable;

ALTER TABLE accounting.period_events
    ALTER COLUMN to_version SET NOT NULL;

CREATE UNIQUE INDEX accounting_period_events_idempotency_unique
    ON accounting.period_events (org_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

ALTER TABLE accounting.period_close_checks
    DROP CONSTRAINT period_close_checks_check_key_check;

ALTER TABLE accounting.period_close_checks
    ADD CONSTRAINT period_close_checks_check_key_check CHECK (
        check_key IN (
            'unposted_documents',
            'fiscal_pending',
            'posting_errors',
            'account_mappings',
            'exchange_rates',
            'unreconciled_accounts',
            'pending_drafts'
        )
    );

-- Annual ranges created by the original bootstrap can be safely replaced by
-- monthly periods only when no durable object points at the old period.
DO $backfill$
DECLARE
    period_record record;
    fiscal_year_id uuid;
    month_offset integer;
BEGIN
    FOR period_record IN
        SELECT
            period.org_id,
            period.id,
            period.start_date,
            period.end_date,
            CASE
                WHEN extract(month FROM period.start_date)::integer = 1
                THEN extract(year FROM period.start_date)::integer::text
                ELSE
                    extract(year FROM period.start_date)::integer::text
                    || '/'
                    || extract(year FROM period.end_date)::integer::text
            END AS fiscal_year_code
          FROM accounting.periods AS period
         WHERE period.status = 'open'
           AND period.start_date =
               date_trunc('month', period.start_date)::date
           AND period.end_date =
               (period.start_date + interval '1 year - 1 day')::date
           AND NOT EXISTS (
                SELECT 1
                  FROM accounting.journal_entries AS entry
                 WHERE entry.org_id = period.org_id
                   AND entry.period_id = period.id
           )
           AND NOT EXISTS (
                SELECT 1
                  FROM accounting.period_events AS event
                 WHERE event.org_id = period.org_id
                   AND event.period_id = period.id
           )
           AND NOT EXISTS (
                SELECT 1
                  FROM accounting.period_close_checks AS check_result
                 WHERE check_result.org_id = period.org_id
                   AND check_result.period_id = period.id
           )
           AND NOT EXISTS (
                SELECT 1
                  FROM accounting.inflation_runs AS run
                 WHERE run.org_id = period.org_id
                   AND run.period_id = period.id
           )
           AND NOT EXISTS (
                SELECT 1
                  FROM accounting.currency_revaluation_runs AS run
                 WHERE run.org_id = period.org_id
                   AND run.period_id = period.id
           )
         ORDER BY period.org_id, period.start_date, period.id
    LOOP
        fiscal_year_id := gen_random_uuid();

        INSERT INTO accounting.fiscal_years (
            org_id,
            id,
            idempotency_key,
            code,
            start_date,
            end_date,
            created_by
        )
        VALUES (
            period_record.org_id,
            fiscal_year_id,
            'migration:split:' || period_record.id::text,
            period_record.fiscal_year_code,
            period_record.start_date,
            period_record.end_date,
            'system:migration'
        );

        DELETE FROM accounting.periods
         WHERE org_id = period_record.org_id
           AND id = period_record.id;

        FOR month_offset IN 0..11
        LOOP
            INSERT INTO accounting.periods (
                org_id,
                code,
                start_date,
                end_date,
                fiscal_year_id,
                period_no,
                is_legacy
            )
            VALUES (
                period_record.org_id,
                to_char(
                    period_record.start_date
                        + make_interval(months => month_offset),
                    'YYYY-MM'
                ),
                (
                    period_record.start_date
                        + make_interval(months => month_offset)
                )::date,
                (
                    period_record.start_date
                        + make_interval(months => month_offset + 1)
                        - interval '1 day'
                )::date,
                fiscal_year_id,
                month_offset + 1,
                false
            );
        END LOOP;
    END LOOP;
END
$backfill$;

-- Preserve used and non-standard historical periods byte-for-byte. Compatible
-- ranges are grouped under a legacy fiscal year; unusual historical ranges
-- remain explicitly ungrouped and can still be read and posted by their ID.
WITH period_bounds AS (
    SELECT
        period.org_id,
        period.id AS period_id,
        make_date(
            extract(year FROM period.start_date)::integer
                - CASE
                    WHEN extract(month FROM period.start_date)::integer
                        < setting.fiscal_year_start_month
                    THEN 1
                    ELSE 0
                  END,
            setting.fiscal_year_start_month,
            1
        ) AS fiscal_start
      FROM accounting.periods AS period
      JOIN accounting.organization_settings AS setting
        ON setting.org_id = period.org_id
     WHERE period.fiscal_year_id IS NULL
),
candidate_years AS (
    SELECT DISTINCT
        bound.org_id,
        bound.fiscal_start,
        (bound.fiscal_start + interval '1 year - 1 day')::date AS fiscal_end
      FROM period_bounds AS bound
      JOIN accounting.periods AS period
        ON period.org_id = bound.org_id
       AND period.id = bound.period_id
     WHERE period.start_date >= bound.fiscal_start
       AND period.end_date <=
           (bound.fiscal_start + interval '1 year - 1 day')::date
)
INSERT INTO accounting.fiscal_years (
    org_id,
    code,
    idempotency_key,
    start_date,
    end_date,
    is_legacy,
    created_by
)
SELECT
    candidate.org_id,
    CASE
        WHEN extract(month FROM candidate.fiscal_start)::integer = 1
        THEN extract(year FROM candidate.fiscal_start)::integer::text
        ELSE
            extract(year FROM candidate.fiscal_start)::integer::text
            || '/'
            || extract(year FROM candidate.fiscal_end)::integer::text
    END,
    'migration:legacy:'
        || to_char(candidate.fiscal_start, 'YYYY-MM-DD'),
    candidate.fiscal_start,
    candidate.fiscal_end,
    true,
    'system:migration'
  FROM candidate_years AS candidate
ON CONFLICT (org_id, start_date, end_date) DO NOTHING;

WITH matching_year AS (
    SELECT
        period.org_id,
        period.id AS period_id,
        fiscal_year.id AS fiscal_year_id,
        (
            (
                extract(year FROM period.start_date)::integer
                - extract(year FROM fiscal_year.start_date)::integer
            ) * 12
            + extract(month FROM period.start_date)::integer
            - extract(month FROM fiscal_year.start_date)::integer
            + 1
        )::smallint AS period_no
      FROM accounting.periods AS period
      JOIN accounting.fiscal_years AS fiscal_year
        ON fiscal_year.org_id = period.org_id
       AND period.start_date >= fiscal_year.start_date
       AND period.end_date <= fiscal_year.end_date
     WHERE period.fiscal_year_id IS NULL
)
UPDATE accounting.periods AS period
   SET fiscal_year_id = matching.fiscal_year_id,
       period_no = matching.period_no,
       is_legacy = true
  FROM matching_year AS matching
 WHERE period.org_id = matching.org_id
   AND period.id = matching.period_id;

UPDATE accounting.periods
   SET is_legacy = true
 WHERE fiscal_year_id IS NULL;

INSERT INTO accounting.fiscal_year_events (
    org_id,
    fiscal_year_id,
    event_type,
    from_status,
    to_status,
    from_version,
    to_version,
    actor,
    metadata
)
SELECT
    fiscal_year.org_id,
    fiscal_year.id,
    'created',
    NULL,
    fiscal_year.annual_close_status,
    NULL,
    fiscal_year.version,
    fiscal_year.created_by,
    jsonb_build_object('migration_backfill', true)
  FROM accounting.fiscal_years AS fiscal_year;

ALTER TABLE accounting.periods
    ADD CONSTRAINT accounting_periods_fiscal_year_fk
        FOREIGN KEY (org_id, fiscal_year_id)
        REFERENCES accounting.fiscal_years(org_id, id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT accounting_periods_sequence_check
        CHECK (period_no IS NULL OR period_no BETWEEN 1 AND 12),
    ADD CONSTRAINT accounting_periods_legacy_shape CHECK (
        is_legacy
        OR (fiscal_year_id IS NOT NULL AND period_no IS NOT NULL)
    );

CREATE UNIQUE INDEX accounting_periods_fiscal_year_number_uidx
    ON accounting.periods (org_id, fiscal_year_id, period_no)
    WHERE fiscal_year_id IS NOT NULL AND period_no IS NOT NULL;

CREATE INDEX accounting_periods_fiscal_year_idx
    ON accounting.periods (
        org_id,
        fiscal_year_id,
        period_no,
        start_date,
        id
    );

CREATE OR REPLACE FUNCTION accounting.validate_fiscal_year()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    invalid_prerequisites boolean;
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended(NEW.org_id::text, 910024)
    );

    IF EXISTS (
        SELECT 1
          FROM accounting.fiscal_years AS fiscal_year
         WHERE fiscal_year.org_id = NEW.org_id
           AND fiscal_year.id <> NEW.id
           AND daterange(fiscal_year.start_date, fiscal_year.end_date, '[]')
               && daterange(NEW.start_date, NEW.end_date, '[]')
    ) THEN
        RAISE EXCEPTION 'accounting fiscal years cannot overlap'
            USING ERRCODE = '23P01',
                  CONSTRAINT = 'accounting_fiscal_years_no_overlap';
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.annual_close_status <> 'not_ready' THEN
            RAISE EXCEPTION 'a new fiscal year must start not ready'
                USING ERRCODE = '23514',
                      CONSTRAINT =
                          'accounting_fiscal_years_initial_status';
        END IF;
        RETURN NEW;
    END IF;

    IF (OLD.code, OLD.start_date, OLD.end_date, OLD.is_legacy)
           IS DISTINCT FROM
       (NEW.code, NEW.start_date, NEW.end_date, NEW.is_legacy) THEN
        IF current_setting(
            'accounting.calendar_regeneration',
            true
        ) IS DISTINCT FROM 'on' THEN
            RAISE EXCEPTION 'fiscal year structure is immutable'
                USING ERRCODE = '55000',
                      CONSTRAINT =
                          'accounting_fiscal_years_structure_locked';
        END IF;
    END IF;

    IF OLD.annual_close_status IS DISTINCT FROM NEW.annual_close_status THEN
        IF NOT (
            (OLD.annual_close_status = 'not_ready'
                AND NEW.annual_close_status IN ('ready', 'not_required'))
            OR
            (OLD.annual_close_status = 'ready'
                AND NEW.annual_close_status IN (
                    'not_ready',
                    'draft',
                    'not_required'
                ))
            OR
            (OLD.annual_close_status = 'draft'
                AND NEW.annual_close_status IN ('ready', 'posted'))
            OR
            (OLD.annual_close_status = 'posted'
                AND NEW.annual_close_status = 'reversed')
            OR
            (OLD.annual_close_status = 'reversed'
                AND NEW.annual_close_status IN ('ready', 'not_required'))
            OR
            (OLD.annual_close_status = 'not_required'
                AND NEW.annual_close_status IN ('not_ready', 'ready'))
        ) THEN
            RAISE EXCEPTION 'invalid fiscal year annual close transition % -> %',
                OLD.annual_close_status,
                NEW.annual_close_status
                USING ERRCODE = '23514',
                      CONSTRAINT =
                          'accounting_fiscal_years_annual_transition';
        END IF;

        IF NEW.version <> OLD.version + 1 THEN
            RAISE EXCEPTION 'fiscal year transition must increment version'
                USING ERRCODE = '40001',
                      CONSTRAINT =
                          'accounting_fiscal_years_transition_version';
        END IF;

        IF NEW.annual_close_changed_by IS NULL
           OR btrim(NEW.annual_close_changed_by) = '' THEN
            RAISE EXCEPTION 'fiscal year transition requires an actor'
                USING ERRCODE = '23514',
                      CONSTRAINT =
                          'accounting_fiscal_years_transition_actor';
        END IF;

        IF NEW.annual_close_status IN (
            'ready',
            'draft',
            'not_required'
        ) AND NOT NEW.is_legacy THEN
            SELECT
                count(*) <> 12
                OR count(*) FILTER (
                    WHERE period.period_no BETWEEN 1 AND 11
                      AND period.status = 'locked'
                ) <> 11
                OR count(*) FILTER (
                    WHERE period.period_no = 12
                      AND period.status = 'soft_closed'
                ) <> 1
              INTO invalid_prerequisites
              FROM accounting.periods AS period
             WHERE period.org_id = NEW.org_id
               AND period.fiscal_year_id = NEW.id;

            IF invalid_prerequisites THEN
                RAISE EXCEPTION 'annual close prerequisites are not satisfied'
                    USING ERRCODE = '23514',
                          CONSTRAINT =
                              'accounting_fiscal_years_close_prerequisites';
            END IF;

            IF (
                SELECT count(*)
                  FROM accounting.period_close_checks AS check_result
                  JOIN accounting.periods AS period
                    ON period.org_id = check_result.org_id
                   AND period.id = check_result.period_id
                 WHERE period.org_id = NEW.org_id
                   AND period.fiscal_year_id = NEW.id
                   AND period.period_no = 12
                   AND check_result.status IN ('passed', 'warning')
            ) <> 7 OR EXISTS (
                SELECT 1
                  FROM accounting.period_close_checks AS check_result
                  JOIN accounting.periods AS period
                    ON period.org_id = check_result.org_id
                   AND period.id = check_result.period_id
                 WHERE period.org_id = NEW.org_id
                   AND period.fiscal_year_id = NEW.id
                   AND period.period_no = 12
                   AND check_result.status = 'blocked'
            ) THEN
                RAISE EXCEPTION 'annual close checklist is incomplete or blocked'
                    USING ERRCODE = '23514',
                          CONSTRAINT =
                              'accounting_fiscal_years_close_checklist';
            END IF;
        END IF;
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.audit_fiscal_year()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO accounting.fiscal_year_events (
            org_id,
            fiscal_year_id,
            event_type,
            from_status,
            to_status,
            from_version,
            to_version,
            actor,
            metadata
        )
        VALUES (
            NEW.org_id,
            NEW.id,
            'created',
            NULL,
            NEW.annual_close_status,
            NULL,
            NEW.version,
            NEW.created_by,
            '{}'::jsonb
        );
    ELSE
        INSERT INTO accounting.fiscal_year_events (
            org_id,
            fiscal_year_id,
            event_type,
            from_status,
            to_status,
            from_version,
            to_version,
            actor,
            reason,
            metadata
        )
        VALUES (
            NEW.org_id,
            NEW.id,
            'annual_close_transition',
            OLD.annual_close_status,
            NEW.annual_close_status,
            OLD.version,
            NEW.version,
            NEW.annual_close_changed_by,
            NEW.transition_reason,
            jsonb_build_object(
                'draft_id', NEW.annual_close_draft_id,
                'entry_id', NEW.annual_close_entry_id,
                'reversal_entry_id', NEW.annual_close_reversal_entry_id,
                'idempotency_key',
                nullif(
                    current_setting(
                        'app.accounting_idempotency_key',
                        true
                    ),
                    ''
                )
            )
        );
    END IF;
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_period()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    parent_year accounting.fiscal_years%ROWTYPE;
    local_today date;
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended(NEW.org_id::text, 910010)
    );

    IF NEW.fiscal_year_id IS NULL THEN
        NEW.is_legacy := true;
        NEW.period_no := coalesce(NEW.period_no, 1);
    ELSE
        SELECT *
          INTO parent_year
          FROM accounting.fiscal_years AS fiscal_year
         WHERE fiscal_year.org_id = NEW.org_id
           AND fiscal_year.id = NEW.fiscal_year_id
         FOR UPDATE;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'accounting period fiscal year does not exist'
                USING ERRCODE = '23503',
                      CONSTRAINT = 'accounting_periods_fiscal_year_fk';
        END IF;

        IF NEW.start_date < parent_year.start_date
           OR NEW.end_date > parent_year.end_date THEN
            RAISE EXCEPTION 'accounting period is outside its fiscal year'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_fiscal_year_range';
        END IF;

        IF NOT NEW.is_legacy AND (
            NEW.period_no IS NULL
            OR NEW.start_date <> (
                parent_year.start_date
                + make_interval(months => NEW.period_no - 1)
            )::date
            OR NEW.end_date <> (
                parent_year.start_date
                + make_interval(months => NEW.period_no)
                - interval '1 day'
            )::date
        ) THEN
            RAISE EXCEPTION 'accounting period must match its monthly sequence'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_monthly_sequence';
        END IF;
    END IF;

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

    IF (
        OLD.fiscal_year_id,
        OLD.period_no,
        OLD.is_legacy
    ) IS DISTINCT FROM (
        NEW.fiscal_year_id,
        NEW.period_no,
        NEW.is_legacy
    ) THEN
        RAISE EXCEPTION 'accounting period membership is immutable'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_periods_membership_locked';
    END IF;

    IF (OLD.start_date, OLD.end_date)
           IS DISTINCT FROM
       (NEW.start_date, NEW.end_date)
       AND (
            NOT OLD.is_legacy
            OR EXISTS (
                SELECT 1
                  FROM accounting.journal_entries AS entry
                 WHERE entry.org_id = OLD.org_id
                   AND entry.period_id = OLD.id
            )
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
            (OLD.status = 'locked' AND NEW.status = 'soft_closed')
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

        IF (
            (OLD.status = 'soft_closed' AND NEW.status = 'open')
            OR
            (OLD.status = 'locked' AND NEW.status = 'soft_closed')
        ) AND (
            NEW.transition_reason IS NULL
            OR btrim(NEW.transition_reason) = ''
        ) THEN
            RAISE EXCEPTION 'reopening a period requires a reason'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_reopen_reason';
        END IF;

        IF OLD.status = 'open' AND NEW.status = 'soft_closed' THEN
            SELECT (
                now() AT TIME ZONE coalesce(
                    (
                        SELECT setting.timezone
                          FROM accounting.organization_settings AS setting
                         WHERE setting.org_id = NEW.org_id
                    ),
                    'America/Argentina/Buenos_Aires'
                )
            )::date
              INTO local_today;

            IF NEW.end_date > local_today THEN
                RAISE EXCEPTION 'a future accounting period cannot be closed'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_periods_future_close';
            END IF;

            IF NEW.fiscal_year_id IS NOT NULL AND EXISTS (
                SELECT 1
                  FROM accounting.periods AS earlier
                 WHERE earlier.org_id = NEW.org_id
                   AND earlier.fiscal_year_id = NEW.fiscal_year_id
                   AND earlier.period_no < NEW.period_no
                   AND earlier.status = 'open'
            ) THEN
                RAISE EXCEPTION 'accounting periods must close chronologically'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_periods_close_order';
            END IF;
        END IF;

        IF OLD.status = 'soft_closed' AND NEW.status = 'locked' THEN
            IF NEW.fiscal_year_id IS NOT NULL AND EXISTS (
                SELECT 1
                  FROM accounting.periods AS earlier
                 WHERE earlier.org_id = NEW.org_id
                   AND earlier.fiscal_year_id = NEW.fiscal_year_id
                   AND earlier.period_no < NEW.period_no
                   AND earlier.status <> 'locked'
            ) THEN
                RAISE EXCEPTION 'earlier accounting periods must be locked first'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_periods_lock_order';
            END IF;

            IF NEW.fiscal_year_id IS NOT NULL
               AND NOT EXISTS (
                    SELECT 1
                      FROM accounting.periods AS later
                     WHERE later.org_id = NEW.org_id
                       AND later.fiscal_year_id = NEW.fiscal_year_id
                       AND later.period_no > NEW.period_no
               )
               AND parent_year.annual_close_status
                   NOT IN ('posted', 'not_required') THEN
                RAISE EXCEPTION 'annual close is pending'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_periods_annual_close_pending';
            END IF;
        END IF;

        IF (
            (OLD.status = 'locked' AND NEW.status = 'soft_closed')
            OR
            (OLD.status = 'soft_closed' AND NEW.status = 'open')
        ) AND NEW.fiscal_year_id IS NOT NULL AND EXISTS (
            SELECT 1
              FROM accounting.periods AS later
             WHERE later.org_id = NEW.org_id
               AND later.fiscal_year_id = NEW.fiscal_year_id
               AND later.period_no > NEW.period_no
               AND later.status <> 'open'
        ) THEN
            RAISE EXCEPTION 'accounting periods must reopen in reverse order'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_periods_reopen_order';
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
        occurred_at,
        from_version,
        to_version,
        idempotency_key
    )
    VALUES (
        NEW.org_id,
        NEW.id,
        OLD.status,
        NEW.status,
        NEW.status_changed_by,
        NEW.transition_reason,
        now(),
        OLD.version,
        NEW.version,
        nullif(
            btrim(
                current_setting(
                    'app.accounting_idempotency_key',
                    true
                )
            ),
            ''
        )
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

    IF completed_checks <> 7 OR EXISTS (
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

    IF EXISTS (
        SELECT 1
          FROM accounting.drafts AS draft
         WHERE draft.org_id = NEW.org_id
           AND draft.status = 'active'
           AND draft.entry_date BETWEEN NEW.start_date AND NEW.end_date
    ) THEN
        RAISE EXCEPTION 'period has pending journal drafts'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_periods_pending_drafts';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_annual_close_posting_freeze()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    close_status text;
    close_entry_id uuid;
BEGIN
    SELECT
        fiscal_year.annual_close_status,
        fiscal_year.annual_close_entry_id
      INTO close_status, close_entry_id
      FROM accounting.periods AS period
      JOIN accounting.fiscal_years AS fiscal_year
        ON fiscal_year.org_id = period.org_id
       AND fiscal_year.id = period.fiscal_year_id
     WHERE period.org_id = NEW.org_id
       AND period.id = NEW.period_id
       AND NOT EXISTS (
            SELECT 1
              FROM accounting.periods AS later
             WHERE later.org_id = period.org_id
               AND later.fiscal_year_id = period.fiscal_year_id
               AND later.end_date > period.end_date
       );

    IF NOT FOUND OR close_status NOT IN (
        'draft',
        'posted',
        'not_required'
    ) THEN
        RETURN NULL;
    END IF;

    IF close_status <> 'posted'
       OR close_entry_id IS DISTINCT FROM NEW.id THEN
        RAISE EXCEPTION
            'annual close freezes postings in the final accounting period'
            USING ERRCODE = '23514',
                  CONSTRAINT =
                      'accounting_journal_entries_annual_close_frozen';
    END IF;

    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_fiscal_year_period_set(
    target_org_id uuid,
    target_fiscal_year_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
DECLARE
    fiscal_year accounting.fiscal_years%ROWTYPE;
    period_count integer;
    invalid_periods integer;
BEGIN
    SELECT *
      INTO fiscal_year
      FROM accounting.fiscal_years
     WHERE org_id = target_org_id
       AND id = target_fiscal_year_id;

    IF NOT FOUND OR fiscal_year.is_legacy THEN
        RETURN;
    END IF;

    SELECT
        count(*),
        count(*) FILTER (
            WHERE period.period_no IS NULL
               OR period.period_no NOT BETWEEN 1 AND 12
               OR period.is_legacy
               OR period.start_date <> (
                    fiscal_year.start_date
                    + make_interval(months => period.period_no - 1)
               )::date
               OR period.end_date <> (
                    fiscal_year.start_date
                    + make_interval(months => period.period_no)
                    - interval '1 day'
               )::date
        )
      INTO period_count, invalid_periods
      FROM accounting.periods AS period
     WHERE period.org_id = target_org_id
       AND period.fiscal_year_id = target_fiscal_year_id;

    IF period_count <> 12 OR invalid_periods <> 0 THEN
        RAISE EXCEPTION 'fiscal year must contain twelve monthly periods'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_fiscal_years_monthly_periods';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.check_fiscal_year_period_set()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, accounting
AS $function$
BEGIN
    IF TG_TABLE_NAME = 'fiscal_years' THEN
        PERFORM accounting.validate_fiscal_year_period_set(NEW.org_id, NEW.id);
    ELSE
        IF TG_OP <> 'DELETE' AND NEW.fiscal_year_id IS NOT NULL THEN
            PERFORM accounting.validate_fiscal_year_period_set(
                NEW.org_id,
                NEW.fiscal_year_id
            );
        END IF;
        IF TG_OP <> 'INSERT'
           AND OLD.fiscal_year_id IS NOT NULL
           AND (
                TG_OP = 'DELETE'
                OR OLD.fiscal_year_id IS DISTINCT FROM NEW.fiscal_year_id
           ) THEN
            PERFORM accounting.validate_fiscal_year_period_set(
                OLD.org_id,
                OLD.fiscal_year_id
            );
        END IF;
    END IF;
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.reject_period_or_fiscal_year_delete()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
    IF current_setting(
        'accounting.calendar_regeneration',
        true
    ) = 'on' THEN
        RETURN OLD;
    END IF;

    RAISE EXCEPTION 'accounting periods and fiscal years cannot be deleted'
        USING ERRCODE = '55000',
              CONSTRAINT = 'accounting_periods_delete_forbidden';
END
$function$;

REVOKE ALL ON FUNCTION accounting.validate_fiscal_year() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.audit_fiscal_year() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.validate_period() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.audit_period_transition() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.validate_period_lock_checks() FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_annual_close_posting_freeze()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.validate_fiscal_year_period_set(uuid, uuid)
FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.check_fiscal_year_period_set() FROM PUBLIC;
REVOKE ALL
ON FUNCTION accounting.reject_period_or_fiscal_year_delete()
FROM PUBLIC;

CREATE TRIGGER accounting_fiscal_years_guard
BEFORE INSERT OR UPDATE
ON accounting.fiscal_years
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_fiscal_year();

CREATE TRIGGER accounting_fiscal_years_audit_insert
AFTER INSERT
ON accounting.fiscal_years
FOR EACH ROW
EXECUTE FUNCTION accounting.audit_fiscal_year();

CREATE TRIGGER accounting_fiscal_years_audit_transition
AFTER UPDATE OF annual_close_status
ON accounting.fiscal_years
FOR EACH ROW
WHEN (OLD.annual_close_status IS DISTINCT FROM NEW.annual_close_status)
EXECUTE FUNCTION accounting.audit_fiscal_year();

CREATE CONSTRAINT TRIGGER accounting_fiscal_years_period_set
AFTER INSERT OR UPDATE
ON accounting.fiscal_years
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.check_fiscal_year_period_set();

CREATE CONSTRAINT TRIGGER accounting_periods_fiscal_year_period_set
AFTER INSERT OR UPDATE OR DELETE
ON accounting.periods
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.check_fiscal_year_period_set();

CREATE TRIGGER accounting_fiscal_years_no_delete
BEFORE DELETE
ON accounting.fiscal_years
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_period_or_fiscal_year_delete();

CREATE TRIGGER accounting_periods_no_delete
BEFORE DELETE
ON accounting.periods
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_period_or_fiscal_year_delete();

CREATE CONSTRAINT TRIGGER accounting_journal_entries_annual_close_freeze
AFTER INSERT
ON accounting.journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION accounting.validate_annual_close_posting_freeze();

CREATE TRIGGER accounting_fiscal_year_events_immutable
BEFORE UPDATE OR DELETE
ON accounting.fiscal_year_events
FOR EACH ROW
EXECUTE FUNCTION accounting.reject_immutable_change();

CREATE OR REPLACE FUNCTION accounting.ensure_fiscal_year(
    target_org_id uuid,
    target_start_date date,
    target_actor text
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    fiscal_year_id uuid;
    fiscal_end date;
    fiscal_code text;
BEGIN
    PERFORM app.assert_org_context(target_org_id);

    IF target_start_date <> date_trunc('month', target_start_date)::date THEN
        RAISE EXCEPTION 'fiscal year must start on the first day of a month'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_fiscal_years_start_month';
    END IF;
    IF target_actor IS NULL OR btrim(target_actor) = '' THEN
        RAISE EXCEPTION 'fiscal year creation requires an actor'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_fiscal_years_created_by';
    END IF;

    PERFORM pg_advisory_xact_lock(
        hashtextextended(target_org_id::text, 910024)
    );

    fiscal_end := (target_start_date + interval '1 year - 1 day')::date;
    fiscal_code := CASE
        WHEN extract(month FROM target_start_date)::integer = 1
        THEN extract(year FROM target_start_date)::integer::text
        ELSE
            extract(year FROM target_start_date)::integer::text
            || '/'
            || extract(year FROM fiscal_end)::integer::text
    END;

    SELECT id
      INTO fiscal_year_id
      FROM accounting.fiscal_years
     WHERE org_id = target_org_id
       AND start_date = target_start_date
       AND end_date = fiscal_end;

    IF FOUND THEN
        RETURN fiscal_year_id;
    END IF;

    fiscal_year_id := gen_random_uuid();
    INSERT INTO accounting.fiscal_years (
        org_id,
        id,
        idempotency_key,
        code,
        start_date,
        end_date,
        created_by
    )
    VALUES (
        target_org_id,
        fiscal_year_id,
        'system:fiscal-year:' || to_char(target_start_date, 'YYYY-MM-DD'),
        fiscal_code,
        target_start_date,
        fiscal_end,
        btrim(target_actor)
    );

    INSERT INTO accounting.periods (
        org_id,
        code,
        start_date,
        end_date,
        fiscal_year_id,
        period_no,
        is_legacy
    )
    SELECT
        target_org_id,
        to_char(
            target_start_date + make_interval(months => month_offset),
            'YYYY-MM'
        ),
        (target_start_date + make_interval(months => month_offset))::date,
        (
            target_start_date
            + make_interval(months => month_offset + 1)
            - interval '1 day'
        )::date,
        fiscal_year_id,
        month_offset + 1,
        false
      FROM generate_series(0, 11) AS month_offset;

    RETURN fiscal_year_id;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.ensure_fiscal_year(uuid, date, text)
FROM PUBLIC;

CREATE OR REPLACE FUNCTION accounting.replace_empty_fiscal_calendar(
    target_org_id uuid,
    target_start_month smallint,
    expected_version bigint,
    target_actor text,
    target_idempotency_key text
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    setting_record accounting.organization_settings%ROWTYPE;
    fiscal_year_record accounting.fiscal_years%ROWTYPE;
    fiscal_year_count integer;
    fiscal_start date;
    fiscal_end date;
    fiscal_code text;
    local_today date;
    start_year integer;
BEGIN
    PERFORM app.assert_org_context(target_org_id);

    IF target_start_month NOT BETWEEN 1 AND 12 THEN
        RAISE EXCEPTION 'fiscal year start month must be between 1 and 12'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_settings_fiscal_start_month';
    END IF;
    IF target_actor IS NULL OR btrim(target_actor) = '' THEN
        RAISE EXCEPTION 'fiscal calendar change requires an actor'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_settings_calendar_actor';
    END IF;
    IF target_idempotency_key IS NULL
       OR btrim(target_idempotency_key) = '' THEN
        RAISE EXCEPTION 'fiscal calendar change requires an idempotency key'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_settings_calendar_idempotency';
    END IF;

    PERFORM pg_advisory_xact_lock(
        hashtextextended(target_org_id::text, 910024)
    );

    SELECT *
      INTO setting_record
      FROM accounting.organization_settings
     WHERE org_id = target_org_id
     FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'accounting settings do not exist'
            USING ERRCODE = 'P0002',
                  CONSTRAINT = 'accounting_settings_not_found';
    END IF;

    IF setting_record.fiscal_calendar_idempotency_key =
       btrim(target_idempotency_key) THEN
        IF setting_record.fiscal_year_start_month <> target_start_month THEN
            RAISE EXCEPTION 'idempotency key was already used for another fiscal calendar'
                USING ERRCODE = '23505',
                      CONSTRAINT =
                          'accounting_settings_calendar_idempotency_reuse';
        END IF;
        SELECT id
          INTO fiscal_year_record.id
          FROM accounting.fiscal_years
         WHERE org_id = target_org_id
         ORDER BY start_date DESC, id DESC
         LIMIT 1;
        RETURN fiscal_year_record.id;
    END IF;

    IF setting_record.version <> expected_version THEN
        RAISE EXCEPTION 'accounting settings version conflict'
            USING ERRCODE = '40001',
                  CONSTRAINT = 'accounting_settings_version_conflict';
    END IF;

    SELECT count(*)
      INTO fiscal_year_count
      FROM accounting.fiscal_years AS fiscal_year
     WHERE fiscal_year.org_id = target_org_id;

    IF fiscal_year_count > 1 THEN
        RAISE EXCEPTION 'fiscal calendar cannot change after multiple fiscal years exist'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_settings_calendar_history';
    END IF;

    IF fiscal_year_count = 1 THEN
        SELECT *
          INTO fiscal_year_record
          FROM accounting.fiscal_years
         WHERE org_id = target_org_id;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM accounting.periods AS period
         WHERE period.org_id = target_org_id
           AND (
                period.status <> 'open'
                OR EXISTS (
                    SELECT 1
                      FROM accounting.journal_entries AS entry
                     WHERE entry.org_id = period.org_id
                       AND entry.period_id = period.id
                )
                OR EXISTS (
                    SELECT 1
                      FROM accounting.period_events AS event
                     WHERE event.org_id = period.org_id
                       AND event.period_id = period.id
                )
                OR EXISTS (
                    SELECT 1
                      FROM accounting.period_close_checks AS check_result
                     WHERE check_result.org_id = period.org_id
                       AND check_result.period_id = period.id
                )
                OR EXISTS (
                    SELECT 1
                      FROM accounting.inflation_runs AS run
                     WHERE run.org_id = period.org_id
                       AND run.period_id = period.id
                )
                OR EXISTS (
                    SELECT 1
                      FROM accounting.currency_revaluation_runs AS run
                     WHERE run.org_id = period.org_id
                       AND run.period_id = period.id
                )
           )
    ) OR EXISTS (
        SELECT 1
          FROM accounting.drafts AS draft
         WHERE draft.org_id = target_org_id
           AND draft.status = 'active'
    ) OR EXISTS (
        SELECT 1
          FROM accounting.fiscal_years AS fiscal_year
         WHERE fiscal_year.org_id = target_org_id
           AND fiscal_year.annual_close_status <> 'not_ready'
    ) THEN
        RAISE EXCEPTION 'fiscal calendar has accounting dependencies'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'accounting_settings_calendar_dependencies';
    END IF;

    local_today := (
        now() AT TIME ZONE setting_record.timezone
    )::date;
    start_year := extract(year FROM local_today)::integer;
    IF extract(month FROM local_today)::integer < target_start_month THEN
        start_year := start_year - 1;
    END IF;
    fiscal_start := make_date(start_year, target_start_month, 1);
    fiscal_end := (fiscal_start + interval '1 year - 1 day')::date;
    fiscal_code := CASE
        WHEN target_start_month = 1 THEN start_year::text
        ELSE start_year::text || '/' || (start_year + 1)::text
    END;

    UPDATE accounting.organization_settings
       SET fiscal_year_start_month = target_start_month,
           fiscal_calendar_idempotency_key =
               btrim(target_idempotency_key),
           fiscal_calendar_changed_by = btrim(target_actor),
           version = version + 1,
           updated_at = now()
     WHERE org_id = target_org_id;

    IF fiscal_year_count = 0 THEN
        RETURN accounting.ensure_fiscal_year(
            target_org_id,
            fiscal_start,
            target_actor
        );
    END IF;

    PERFORM set_config(
        'accounting.calendar_regeneration',
        'on',
        true
    );

    DELETE FROM accounting.periods
     WHERE org_id = target_org_id
       AND fiscal_year_id = fiscal_year_record.id;

    UPDATE accounting.fiscal_years
       SET idempotency_key =
               'calendar:' || btrim(target_idempotency_key),
           code = fiscal_code,
           start_date = fiscal_start,
           end_date = fiscal_end,
           is_legacy = false,
           version = version + 1,
           updated_at = now()
     WHERE org_id = target_org_id
       AND id = fiscal_year_record.id;

    INSERT INTO accounting.periods (
        org_id,
        code,
        start_date,
        end_date,
        fiscal_year_id,
        period_no,
        is_legacy
    )
    SELECT
        target_org_id,
        to_char(
            fiscal_start + make_interval(months => month_offset),
            'YYYY-MM'
        ),
        (fiscal_start + make_interval(months => month_offset))::date,
        (
            fiscal_start
            + make_interval(months => month_offset + 1)
            - interval '1 day'
        )::date,
        fiscal_year_record.id,
        month_offset + 1,
        false
      FROM generate_series(0, 11) AS month_offset;

    INSERT INTO accounting.fiscal_year_events (
        org_id,
        fiscal_year_id,
        event_type,
        from_status,
        to_status,
        from_version,
        to_version,
        actor,
        reason,
        metadata
    )
    VALUES (
        target_org_id,
        fiscal_year_record.id,
        'calendar_changed',
        fiscal_year_record.annual_close_status,
        fiscal_year_record.annual_close_status,
        fiscal_year_record.version,
        fiscal_year_record.version + 1,
        btrim(target_actor),
        'Fiscal calendar regenerated before first accounting use',
        jsonb_build_object(
            'from_start_month',
            setting_record.fiscal_year_start_month,
            'to_start_month',
            target_start_month,
            'idempotency_key',
            btrim(target_idempotency_key)
        )
    );

    PERFORM set_config(
        'accounting.calendar_regeneration',
        'off',
        true
    );

    RETURN fiscal_year_record.id;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.replace_empty_fiscal_calendar(
    uuid,
    smallint,
    bigint,
    text,
    text
)
FROM PUBLIC;

ALTER TABLE accounting.fiscal_years ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting.fiscal_years FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON accounting.fiscal_years
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
);

ALTER TABLE accounting.fiscal_year_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting.fiscal_year_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON accounting.fiscal_year_events
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
);

REVOKE ALL ON TABLE accounting.fiscal_years FROM PUBLIC;
REVOKE ALL ON TABLE accounting.fiscal_year_events FROM PUBLIC;

DO $bootstrap$
DECLARE
    setting_record record;
    fiscal_start date;
    start_year integer;
BEGIN
    FOR setting_record IN
        SELECT
            setting.org_id,
            setting.fiscal_year_start_month,
            setting.timezone,
            (now() AT TIME ZONE setting.timezone)::date AS local_today
          FROM accounting.organization_settings AS setting
         WHERE NOT EXISTS (
                SELECT 1
                  FROM accounting.periods AS period
                 WHERE period.org_id = setting.org_id
                   AND (now() AT TIME ZONE setting.timezone)::date
                       BETWEEN period.start_date AND period.end_date
           )
         ORDER BY setting.org_id
    LOOP
        PERFORM set_config(
            'app.org_id',
            setting_record.org_id::text,
            true
        );
        start_year := extract(year FROM setting_record.local_today)::integer;
        IF extract(month FROM setting_record.local_today)::integer
            < setting_record.fiscal_year_start_month THEN
            start_year := start_year - 1;
        END IF;
        fiscal_start := make_date(
            start_year,
            setting_record.fiscal_year_start_month,
            1
        );
        PERFORM accounting.ensure_fiscal_year(
            setting_record.org_id,
            fiscal_start,
            'system:migration'
        );
    END LOOP;
END
$bootstrap$;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT, INSERT, UPDATE
        ON accounting.fiscal_years
        TO pymes_backend;

        GRANT SELECT
        ON accounting.fiscal_year_events
        TO pymes_backend;

        GRANT EXECUTE
        ON FUNCTION accounting.ensure_fiscal_year(uuid, date, text)
        TO pymes_backend;

        GRANT EXECUTE
        ON FUNCTION accounting.replace_empty_fiscal_calendar(
            uuid,
            smallint,
            bigint,
            text,
            text
        )
        TO pymes_backend;

        REVOKE DELETE ON accounting.periods FROM pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_accounting_worker'
    ) THEN
        GRANT SELECT
        ON accounting.fiscal_years
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
