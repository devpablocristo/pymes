ALTER TABLE fiscal.requests
  ADD COLUMN IF NOT EXISTS correlation_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS delegated_actor_ref text,
  ADD COLUMN IF NOT EXISTS workload_issuer text,
  ADD COLUMN IF NOT EXISTS workload_subject text,
  ADD COLUMN IF NOT EXISTS workload_request_id text,
  ADD COLUMN IF NOT EXISTS workload_token_id text;

UPDATE fiscal.requests
SET correlation_id = CASE
      WHEN result->>'correlation_id' ~ '^[A-Za-z0-9:_./-]{1,255}$'
        THEN result->>'correlation_id'
      ELSE 'legacy:unknown'
    END,
    workload_issuer = 'legacy:unknown',
    workload_subject = 'legacy:unknown',
    workload_request_id = 'legacy:unknown',
    workload_token_id = 'legacy:unknown'
WHERE correlation_id IS NULL
   OR workload_issuer IS NULL
   OR workload_subject IS NULL
   OR workload_request_id IS NULL
   OR workload_token_id IS NULL;

ALTER TABLE fiscal.requests
  ALTER COLUMN correlation_id SET NOT NULL,
  ALTER COLUMN workload_issuer SET NOT NULL,
  ALTER COLUMN workload_subject SET NOT NULL,
  ALTER COLUMN workload_request_id SET NOT NULL,
  ALTER COLUMN workload_token_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'fiscal.requests'::regclass
      AND conname = 'fiscal_requests_operational_identity_shape'
  ) THEN
    ALTER TABLE fiscal.requests
      ADD CONSTRAINT fiscal_requests_operational_identity_shape CHECK (
        correlation_id ~ '^[A-Za-z0-9:_./-]{1,255}$'
        AND workload_issuer ~ '^[A-Za-z0-9:_./-]{1,255}$'
        AND workload_subject ~ '^[A-Za-z0-9:_./-]{1,255}$'
        AND workload_request_id ~ '^[A-Za-z0-9:_./-]{1,255}$'
        AND workload_token_id ~ '^[A-Za-z0-9:_./-]{1,255}$'
        AND (actor_ref IS NULL OR actor_ref ~ '^[A-Za-z0-9:_./-]{1,255}$')
        AND (delegated_actor_ref IS NULL OR delegated_actor_ref ~ '^[A-Za-z0-9:_./-]{1,255}$')
        AND (delegated_actor_ref IS NULL OR actor_ref IS NOT NULL)
      );
  END IF;
END
$$;

ALTER TABLE fiscal.requests ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS fiscal_requests_tenant_isolation ON fiscal.requests;
CREATE POLICY fiscal_requests_tenant_isolation ON fiscal.requests
  USING (organization_id = current_setting('app.organization_id', true))
  WITH CHECK (organization_id = current_setting('app.organization_id', true));

CREATE OR REPLACE FUNCTION fiscal.request_metrics()
RETURNS TABLE (
  authorized integer,
  rejected integer,
  uncertain integer,
  not_found integer
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $$
  SELECT
    count(*) FILTER (WHERE result->>'status' = 'authorized')::integer,
    count(*) FILTER (WHERE result->>'status' = 'rejected')::integer,
    count(*) FILTER (WHERE result->>'status' = 'uncertain')::integer,
    count(*) FILTER (WHERE result->>'status' = 'not_found')::integer
  FROM fiscal.requests
$$;

REVOKE ALL ON FUNCTION fiscal.request_metrics() FROM PUBLIC;
