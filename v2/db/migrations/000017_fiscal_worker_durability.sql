-- Fiscal authorization durability:
--   * only one unresolved voucher may own a POS/type series;
--   * an expired processing lease is discoverable and reclaimable;
--   * uncertainty keeps the series blocked until FECompConsultar resolves it.

CREATE UNIQUE INDEX fiscal_vouchers_one_unresolved_series_uidx
    ON fiscal.vouchers (
        org_id,
        environment,
        point_of_sale_id,
        voucher_type
    )
    WHERE status IN ('processing', 'uncertain');

CREATE OR REPLACE FUNCTION fiscal.lease_voucher(
    requested_org_id uuid,
    requested_worker text,
    requested_lease interval
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, fiscal
AS $function$
DECLARE
    leased_voucher_id uuid;
BEGIN
    PERFORM app.assert_org_context(requested_org_id);

    IF btrim(requested_worker) = ''
       OR requested_lease <= interval '0 seconds'
       OR requested_lease > interval '15 minutes' THEN
        RAISE EXCEPTION 'invalid fiscal voucher lease'
            USING ERRCODE = '22023';
    END IF;

    SELECT voucher.id
      INTO leased_voucher_id
      FROM fiscal.vouchers AS voucher
     WHERE voucher.org_id = requested_org_id
       AND (
            voucher.status IN ('queued', 'uncertain')
            OR (
                voucher.status = 'processing'
                AND voucher.lease_until <= now()
            )
       )
       AND (
            voucher.lease_until IS NULL
            OR voucher.lease_until <= now()
       )
       AND NOT EXISTS (
            SELECT 1
              FROM fiscal.vouchers AS blocker
             WHERE blocker.org_id = voucher.org_id
               AND blocker.environment = voucher.environment
               AND blocker.point_of_sale_id = voucher.point_of_sale_id
               AND blocker.voucher_type = voucher.voucher_type
               AND blocker.id <> voucher.id
               AND blocker.status IN ('processing', 'uncertain')
       )
     ORDER BY
        CASE voucher.status
            WHEN 'processing' THEN 0
            WHEN 'uncertain' THEN 1
            ELSE 2
        END,
        voucher.created_at,
        voucher.id
     FOR UPDATE SKIP LOCKED
     LIMIT 1;

    IF leased_voucher_id IS NULL THEN
        RETURN NULL;
    END IF;

    UPDATE fiscal.vouchers
       SET status = 'processing',
           lease_owner = requested_worker,
           lease_until = now() + requested_lease,
           uncertain_at = CASE
                WHEN status = 'uncertain' THEN NULL
                ELSE uncertain_at
           END,
           version = version + 1
     WHERE org_id = requested_org_id
       AND id = leased_voucher_id;

    RETURN leased_voucher_id;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.pending_organizations(
    requested_limit integer
)
RETURNS TABLE (org_id uuid)
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
BEGIN
    IF requested_limit IS NULL
       OR requested_limit < 1
       OR requested_limit > 1000 THEN
        RAISE EXCEPTION
            'pending organization limit must be between 1 and 1000'
            USING ERRCODE = '22023';
    END IF;

    RETURN QUERY
    SELECT pending.org_id
      FROM (
        SELECT
            voucher.org_id,
            min(voucher.created_at) AS ready_at
          FROM fiscal.vouchers AS voucher
         WHERE (
                voucher.status IN ('queued', 'uncertain')
                OR voucher.status = 'processing'
            )
           AND (
                voucher.lease_until IS NULL
                OR voucher.lease_until <= now()
            )
         GROUP BY voucher.org_id

        UNION ALL

        SELECT
            intent.org_id,
            min(intent.updated_at) AS ready_at
          FROM fiscal.accounting_posting_intents AS intent
         WHERE intent.status IN ('pending', 'failed')
         GROUP BY intent.org_id
      ) AS pending
     GROUP BY pending.org_id
     ORDER BY min(pending.ready_at), pending.org_id
     LIMIT requested_limit;
END
$function$;

COMMENT ON INDEX fiscal.fiscal_vouchers_one_unresolved_series_uidx IS
'Durably serializes ARCA numbering: a queued voucher cannot overtake a processing or uncertain voucher for the same series.';

COMMENT ON FUNCTION fiscal.pending_organizations(integer) IS
'Returns tenants with queued, uncertain, or expired processing fiscal work; business access remains tenant-scoped under FORCE RLS.';

REVOKE ALL
ON FUNCTION fiscal.lease_voucher(uuid, text, interval)
FROM PUBLIC;

REVOKE ALL
ON FUNCTION fiscal.pending_organizations(integer)
FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_worker'
    ) THEN
        GRANT EXECUTE
        ON FUNCTION fiscal.lease_voucher(uuid, text, interval)
        TO pymes_fiscal_worker;

        GRANT EXECUTE
        ON FUNCTION fiscal.pending_organizations(integer)
        TO pymes_fiscal_worker;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_backend'
    ) THEN
        REVOKE EXECUTE
        ON FUNCTION fiscal.lock_voucher_series(
            uuid,
            text,
            uuid,
            integer
        )
        FROM pymes_backend;

        REVOKE EXECUTE
        ON FUNCTION fiscal.reserve_voucher_number(uuid, uuid, bigint)
        FROM pymes_backend;

        REVOKE EXECUTE
        ON FUNCTION fiscal.lease_voucher(uuid, text, interval)
        FROM pymes_backend;

        REVOKE EXECUTE
        ON FUNCTION fiscal.pending_organizations(integer)
        FROM pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_accounting_worker'
    ) THEN
        GRANT EXECUTE
        ON FUNCTION fiscal.pending_organizations(integer)
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
