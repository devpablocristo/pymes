-- Bootstrap the bounded Argentina accounting MVP for organizations that
-- existed before accounting was introduced. Organizations with any chart
-- data are left untouched and must be configured explicitly.

DO $bootstrap$
DECLARE
    organization_record record;
    local_today date;
    start_year integer;
    start_month integer;
    period_start date;
BEGIN
    FOR organization_record IN
        SELECT organization.id
          FROM iam.organizations AS organization
         WHERE organization.status IN ('active', 'provisioning')
           AND NOT EXISTS (
                SELECT 1
                  FROM accounting.accounts AS account
                 WHERE account.org_id = organization.id
           )
         ORDER BY organization.id
    LOOP
        PERFORM set_config(
            'app.org_id',
            organization_record.id::text,
            true
        );
        PERFORM accounting.install_chart_template(
            organization_record.id,
            'ar-pyme',
            1
        );
    END LOOP;

    FOR organization_record IN
        SELECT
            setting.org_id AS id,
            setting.fiscal_year_start_month AS start_month,
            setting.timezone
          FROM accounting.organization_settings AS setting
         WHERE NOT EXISTS (
                SELECT 1
                  FROM accounting.periods AS period
                 WHERE period.org_id = setting.org_id
           )
         ORDER BY setting.org_id
    LOOP
        PERFORM set_config(
            'app.org_id',
            organization_record.id::text,
            true
        );
        local_today := (now() AT TIME ZONE organization_record.timezone)::date;
        start_month := organization_record.start_month;
        start_year := extract(year FROM local_today)::integer;
        IF extract(month FROM local_today)::integer < start_month THEN
            start_year := start_year - 1;
        END IF;
        period_start := make_date(start_year, start_month, 1);

        INSERT INTO accounting.periods (
            org_id,
            code,
            start_date,
            end_date
        )
        VALUES (
            organization_record.id,
            start_year::text,
            period_start,
            (period_start + interval '1 year - 1 day')::date
        )
        ON CONFLICT DO NOTHING;
    END LOOP;
END
$bootstrap$;

