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
         WHERE voucher.status IN ('queued', 'uncertain')
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

COMMENT ON FUNCTION fiscal.pending_organizations(integer) IS
'Returns only tenant identifiers with fiscal authorization or fiscal-accounting work; all business reads and writes remain tenant-scoped under RLS.';

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
        ON FUNCTION fiscal.pending_organizations(integer)
        TO pymes_fiscal_worker;
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
