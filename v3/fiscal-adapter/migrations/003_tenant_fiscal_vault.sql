CREATE TABLE IF NOT EXISTS fiscal.credentials (
  organization_id text NOT NULL,
  credential_id text NOT NULL,
  cuit char(11) NOT NULL,
  environment text NOT NULL,
  legal_name text NOT NULL,
  common_name text NOT NULL,
  status text NOT NULL,
  idempotency_key text NOT NULL,
  request_hash char(64) NOT NULL,
  csr_pem text NOT NULL,
  encrypted_private_key jsonb NOT NULL,
  encrypted_certificate jsonb,
  certificate_fingerprint char(64),
  certificate_valid_from timestamptz,
  certificate_expires_at timestamptz,
  certificate_serial_number text,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, credential_id),
  UNIQUE (organization_id, idempotency_key),
  UNIQUE (organization_id, credential_id, environment),
  CONSTRAINT fiscal_credentials_id_shape
    CHECK (credential_id ~ '^fcred_[A-Za-z0-9_-]{8,80}$'),
  CONSTRAINT fiscal_credentials_cuit_shape CHECK (cuit ~ '^[0-9]{11}$'),
  CONSTRAINT fiscal_credentials_environment
    CHECK (environment IN ('homologation','production')),
  CONSTRAINT fiscal_credentials_status
    CHECK (status IN ('pending_certificate','ready','disabled','expired')),
  CONSTRAINT fiscal_credentials_version CHECK (version > 0),
  CONSTRAINT fiscal_credentials_idempotency_shape
    CHECK (length(idempotency_key) BETWEEN 8 AND 128),
  CONSTRAINT fiscal_credentials_request_hash
    CHECK (request_hash ~ '^[a-f0-9]{64}$'),
  CONSTRAINT fiscal_credentials_ready_material CHECK (
    status <> 'ready'
    OR (
      encrypted_certificate IS NOT NULL
      AND certificate_fingerprint IS NOT NULL
      AND certificate_valid_from IS NOT NULL
      AND certificate_expires_at IS NOT NULL
      AND certificate_serial_number IS NOT NULL
    )
  )
);

CREATE TABLE IF NOT EXISTS fiscal.points_of_sale (
  organization_id text NOT NULL,
  credential_id text NOT NULL,
  environment text NOT NULL,
  point_of_sale integer NOT NULL,
  enabled boolean NOT NULL DEFAULT false,
  validated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, credential_id, environment, point_of_sale),
  FOREIGN KEY (organization_id, credential_id, environment)
    REFERENCES fiscal.credentials(organization_id, credential_id, environment)
    ON DELETE CASCADE,
  CONSTRAINT fiscal_points_of_sale_range
    CHECK (point_of_sale BETWEEN 1 AND 99999)
);

CREATE TABLE IF NOT EXISTS fiscal.wsaa_tickets (
  organization_id text NOT NULL,
  credential_id text NOT NULL,
  environment text NOT NULL,
  service text NOT NULL,
  encrypted_ticket jsonb NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, credential_id, environment, service),
  FOREIGN KEY (organization_id, credential_id, environment)
    REFERENCES fiscal.credentials(organization_id, credential_id, environment)
    ON DELETE CASCADE,
  CONSTRAINT fiscal_wsaa_service_shape
    CHECK (service ~ '^[a-z0-9_]{2,64}$')
);

CREATE TABLE IF NOT EXISTS fiscal.encrypted_artifacts (
  organization_id text NOT NULL,
  artifact_id text NOT NULL,
  request_id text NOT NULL,
  kind text NOT NULL,
  encrypted_payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, artifact_id),
  CONSTRAINT fiscal_artifact_id_shape
    CHECK (artifact_id ~ '^fartifact_[A-Za-z0-9_-]{8,100}$'),
  CONSTRAINT fiscal_artifact_kind
    CHECK (kind IN ('wsfe_authorization','wsfe_consultation'))
);

ALTER TABLE fiscal.mock_authorizations
  ADD COLUMN IF NOT EXISTS organization_id text;

UPDATE fiscal.mock_authorizations
SET organization_id = split_part(voucher_key, '/', 1)
WHERE organization_id IS NULL;

ALTER TABLE fiscal.mock_authorizations
  ALTER COLUMN organization_id SET NOT NULL;

DO $$
DECLARE
  primary_key_name text;
BEGIN
  SELECT conname INTO primary_key_name
    FROM pg_constraint
   WHERE conrelid = 'fiscal.mock_authorizations'::regclass
     AND contype = 'p';
  IF primary_key_name IS NOT NULL
     AND NOT EXISTS (
       SELECT 1
         FROM pg_constraint
        WHERE conrelid = 'fiscal.mock_authorizations'::regclass
          AND conname = 'fiscal_mock_authorizations_tenant_pkey'
     ) THEN
    EXECUTE format(
      'ALTER TABLE fiscal.mock_authorizations DROP CONSTRAINT %I',
      primary_key_name
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conrelid = 'fiscal.mock_authorizations'::regclass
       AND conname = 'fiscal_mock_authorizations_tenant_pkey'
  ) THEN
    ALTER TABLE fiscal.mock_authorizations
      ADD CONSTRAINT fiscal_mock_authorizations_tenant_pkey
      PRIMARY KEY (organization_id, voucher_key);
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS fiscal.metric_counts (
  status text PRIMARY KEY,
  total bigint NOT NULL DEFAULT 0,
  CONSTRAINT fiscal_metric_status
    CHECK (status IN ('authorized','rejected','uncertain','not_found')),
  CONSTRAINT fiscal_metric_total CHECK (total >= 0)
);

INSERT INTO fiscal.metric_counts(status,total)
SELECT status, count(*)
  FROM (
    SELECT result->>'status' AS status
      FROM fiscal.requests
  ) existing
 WHERE status IN ('authorized','rejected','uncertain','not_found')
 GROUP BY status
ON CONFLICT(status) DO UPDATE SET total=EXCLUDED.total;

INSERT INTO fiscal.metric_counts(status,total)
VALUES ('authorized',0),('rejected',0),('uncertain',0),('not_found',0)
ON CONFLICT(status) DO NOTHING;

CREATE OR REPLACE FUNCTION fiscal.update_request_metrics()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $$
DECLARE
  old_status text;
  new_status text;
BEGIN
  old_status := CASE WHEN TG_OP = 'INSERT' THEN NULL ELSE OLD.result->>'status' END;
  new_status := NEW.result->>'status';
  IF old_status IS DISTINCT FROM new_status THEN
    IF old_status IN ('authorized','rejected','uncertain','not_found') THEN
      UPDATE fiscal.metric_counts
         SET total=GREATEST(total-1,0)
       WHERE status=old_status;
    END IF;
    IF new_status IN ('authorized','rejected','uncertain','not_found') THEN
      UPDATE fiscal.metric_counts SET total=total+1 WHERE status=new_status;
    END IF;
  END IF;
  RETURN NEW;
END
$$;

CREATE OR REPLACE FUNCTION fiscal.reset_request_metrics()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $$
BEGIN
  UPDATE fiscal.metric_counts SET total=0;
  RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS fiscal_requests_metrics ON fiscal.requests;
CREATE TRIGGER fiscal_requests_metrics
AFTER INSERT OR UPDATE OF result ON fiscal.requests
FOR EACH ROW EXECUTE FUNCTION fiscal.update_request_metrics();

DROP TRIGGER IF EXISTS fiscal_requests_metrics_truncate ON fiscal.requests;
CREATE TRIGGER fiscal_requests_metrics_truncate
AFTER TRUNCATE ON fiscal.requests
FOR EACH STATEMENT EXECUTE FUNCTION fiscal.reset_request_metrics();

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
    max(total) FILTER (WHERE status='authorized')::integer,
    max(total) FILTER (WHERE status='rejected')::integer,
    max(total) FILTER (WHERE status='uncertain')::integer,
    max(total) FILTER (WHERE status='not_found')::integer
  FROM fiscal.metric_counts
$$;

REVOKE ALL ON FUNCTION fiscal.update_request_metrics() FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.reset_request_metrics() FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.request_metrics() FROM PUBLIC;

ALTER TABLE fiscal.requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.requests FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal.mock_authorizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.mock_authorizations FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal.credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.credentials FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal.points_of_sale ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.points_of_sale FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal.wsaa_tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.wsaa_tickets FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal.encrypted_artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.encrypted_artifacts FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS fiscal_mock_authorizations_tenant_isolation
  ON fiscal.mock_authorizations;
CREATE POLICY fiscal_mock_authorizations_tenant_isolation
  ON fiscal.mock_authorizations
  USING (organization_id = current_setting('app.organization_id', true))
  WITH CHECK (organization_id = current_setting('app.organization_id', true));

DROP POLICY IF EXISTS fiscal_credentials_tenant_isolation
  ON fiscal.credentials;
CREATE POLICY fiscal_credentials_tenant_isolation
  ON fiscal.credentials
  USING (organization_id = current_setting('app.organization_id', true))
  WITH CHECK (organization_id = current_setting('app.organization_id', true));

DROP POLICY IF EXISTS fiscal_points_of_sale_tenant_isolation
  ON fiscal.points_of_sale;
CREATE POLICY fiscal_points_of_sale_tenant_isolation
  ON fiscal.points_of_sale
  USING (organization_id = current_setting('app.organization_id', true))
  WITH CHECK (organization_id = current_setting('app.organization_id', true));

DROP POLICY IF EXISTS fiscal_wsaa_tickets_tenant_isolation
  ON fiscal.wsaa_tickets;
CREATE POLICY fiscal_wsaa_tickets_tenant_isolation
  ON fiscal.wsaa_tickets
  USING (organization_id = current_setting('app.organization_id', true))
  WITH CHECK (organization_id = current_setting('app.organization_id', true));

DROP POLICY IF EXISTS fiscal_artifacts_tenant_isolation
  ON fiscal.encrypted_artifacts;
CREATE POLICY fiscal_artifacts_tenant_isolation
  ON fiscal.encrypted_artifacts
  USING (organization_id = current_setting('app.organization_id', true))
  WITH CHECK (organization_id = current_setting('app.organization_id', true));
