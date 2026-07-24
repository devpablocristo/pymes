CREATE TABLE fiscal.homologation_runs (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    environment text NOT NULL DEFAULT 'homologation'
        CHECK (environment = 'homologation'),
    status text NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'succeeded', 'failed')),
    requested_by text NOT NULL CHECK (btrim(requested_by) <> ''),
    certificate_fingerprint_sha256 char(64)
        CHECK (
            certificate_fingerprint_sha256 IS NULL
            OR certificate_fingerprint_sha256 ~ '^[0-9a-f]{64}$'
        ),
    configuration_sha256 char(64)
        CHECK (
            configuration_sha256 IS NULL
            OR configuration_sha256 ~ '^[0-9a-f]{64}$'
        ),
    point_of_sale_count integer NOT NULL DEFAULT 0
        CHECK (point_of_sale_count >= 0),
    check_count integer NOT NULL DEFAULT 0
        CHECK (check_count >= 0),
    success_count integer NOT NULL DEFAULT 0
        CHECK (success_count >= 0),
    failure_count integer NOT NULL DEFAULT 0
        CHECK (failure_count >= 0),
    evidence_sha256 char(64)
        CHECK (
            evidence_sha256 IS NULL
            OR evidence_sha256 ~ '^[0-9a-f]{64}$'
        ),
    evidence jsonb
        CHECK (evidence IS NULL OR jsonb_typeof(evidence) = 'object'),
    evidence_note text NOT NULL DEFAULT
        'Evidencia técnica de interoperabilidad; no constituye aprobación ni homologación otorgada por ARCA.'
        CHECK (btrim(evidence_note) <> ''),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    PRIMARY KEY (org_id, id),
    CHECK (check_count = success_count + failure_count),
    CHECK (
        (
            status = 'running'
            AND completed_at IS NULL
            AND evidence_sha256 IS NULL
            AND evidence IS NULL
            AND configuration_sha256 IS NULL
            AND check_count = 0
        )
        OR
        (
            status IN ('succeeded', 'failed')
            AND completed_at IS NOT NULL
            AND completed_at >= started_at
            AND evidence_sha256 IS NOT NULL
            AND evidence IS NOT NULL
            AND check_count > 0
        )
    ),
    CHECK (
        status <> 'succeeded'
        OR (
            certificate_fingerprint_sha256 IS NOT NULL
            AND configuration_sha256 IS NOT NULL
            AND failure_count = 0
            AND success_count = check_count
            AND point_of_sale_count > 0
        )
    ),
    CHECK (status <> 'failed' OR failure_count > 0)
);

CREATE INDEX fiscal_homologation_runs_history_idx
    ON fiscal.homologation_runs (org_id, started_at DESC, id DESC);

CREATE TABLE fiscal.homologation_checks (
    org_id uuid NOT NULL,
    run_id uuid NOT NULL,
    ordinal integer NOT NULL CHECK (ordinal > 0),
    kind text NOT NULL CHECK (
        kind IN (
            'configuration',
            'certificate',
            'wsaa',
            'wsfe_last_authorized',
            'local_matrix'
        )
    ),
    name text NOT NULL CHECK (btrim(name) <> ''),
    status text NOT NULL CHECK (status IN ('succeeded', 'failed')),
    point_of_sale integer
        CHECK (point_of_sale IS NULL OR point_of_sale BETWEEN 1 AND 99999),
    voucher_type integer
        CHECK (
            voucher_type IS NULL
            OR voucher_type IN (1, 2, 3, 6, 7, 8, 11, 12, 13)
        ),
    detail_redacted text NOT NULL CHECK (btrim(detail_redacted) <> ''),
    evidence jsonb NOT NULL CHECK (jsonb_typeof(evidence) = 'object'),
    evidence_sha256 char(64) NOT NULL
        CHECK (evidence_sha256 ~ '^[0-9a-f]{64}$'),
    started_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL,
    PRIMARY KEY (org_id, run_id, ordinal),
    CONSTRAINT fiscal_homologation_checks_run_fk
        FOREIGN KEY (org_id, run_id)
        REFERENCES fiscal.homologation_runs(org_id, id)
        ON DELETE RESTRICT,
    CHECK (completed_at >= started_at)
);

CREATE INDEX fiscal_homologation_checks_lookup_idx
    ON fiscal.homologation_checks (
        org_id,
        run_id,
        kind,
        point_of_sale,
        voucher_type
    );

CREATE OR REPLACE FUNCTION fiscal.protect_homologation_run()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    actual_checks integer;
    actual_successes integer;
    actual_failures integer;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION
            'homologation runs are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF OLD.status <> 'running' THEN
        RAISE EXCEPTION
            'finalized homologation runs are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF NEW.org_id IS DISTINCT FROM OLD.org_id
       OR NEW.id IS DISTINCT FROM OLD.id
       OR NEW.environment IS DISTINCT FROM OLD.environment
       OR NEW.requested_by IS DISTINCT FROM OLD.requested_by
       OR NEW.started_at IS DISTINCT FROM OLD.started_at
       OR NEW.evidence_note IS DISTINCT FROM OLD.evidence_note THEN
        RAISE EXCEPTION
            'homologation run identity and provenance are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF NEW.status NOT IN ('succeeded', 'failed') THEN
        RAISE EXCEPTION
            'homologation run may only transition from running to a final state'
            USING ERRCODE = '55000';
    END IF;

    SELECT
        count(*),
        count(*) FILTER (WHERE checks.status = 'succeeded'),
        count(*) FILTER (WHERE checks.status = 'failed')
      INTO actual_checks, actual_successes, actual_failures
      FROM fiscal.homologation_checks AS checks
     WHERE checks.org_id = OLD.org_id
       AND checks.run_id = OLD.id;

    IF NEW.check_count <> actual_checks
       OR NEW.success_count <> actual_successes
       OR NEW.failure_count <> actual_failures THEN
        RAISE EXCEPTION
            'homologation run aggregates do not match immutable checks'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.protect_homologation_check()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    run_status text;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        RAISE EXCEPTION
            'homologation checks are immutable'
            USING ERRCODE = '55000';
    END IF;

    SELECT runs.status
      INTO run_status
      FROM fiscal.homologation_runs AS runs
     WHERE runs.org_id = NEW.org_id
       AND runs.id = NEW.run_id
     FOR KEY SHARE;

    IF run_status IS DISTINCT FROM 'running' THEN
        RAISE EXCEPTION
            'checks may only be appended to a running homologation run'
            USING ERRCODE = '55000';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.invalidate_production_readiness()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal, fiscal_ar
AS $function$
DECLARE
    target_org_id uuid;
    changed_environment text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_org_id := OLD.org_id;
    ELSE
        target_org_id := NEW.org_id;
    END IF;

    IF TG_TABLE_SCHEMA = 'fiscal_ar'
       AND TG_TABLE_NAME = 'settings' THEN
        IF TG_OP = 'DELETE' THEN
            changed_environment := OLD.environment;
        ELSE
            changed_environment := NEW.environment;
        END IF;
        IF changed_environment <> 'homologation' THEN
            IF TG_OP = 'DELETE' THEN
                RETURN OLD;
            END IF;
            RETURN NEW;
        END IF;
    END IF;

    UPDATE fiscal_ar.settings
       SET enabled = false,
           version = version + 1,
           updated_at = now()
     WHERE org_id = target_org_id
       AND environment = 'production'
       AND enabled;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$function$;

CREATE TRIGGER fiscal_homologation_runs_protect
BEFORE UPDATE OR DELETE ON fiscal.homologation_runs
FOR EACH ROW
EXECUTE FUNCTION fiscal.protect_homologation_run();

CREATE TRIGGER fiscal_homologation_checks_protect
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.homologation_checks
FOR EACH ROW
EXECUTE FUNCTION fiscal.protect_homologation_check();

CREATE TRIGGER fiscal_profile_invalidates_production
AFTER INSERT OR UPDATE OR DELETE ON fiscal.profiles
FOR EACH ROW
EXECUTE FUNCTION fiscal.invalidate_production_readiness();

CREATE TRIGGER fiscal_ar_homologation_settings_invalidate_production
AFTER INSERT OR UPDATE OR DELETE ON fiscal_ar.settings
FOR EACH ROW
EXECUTE FUNCTION fiscal.invalidate_production_readiness();

CREATE TRIGGER fiscal_certificates_invalidate_production
AFTER INSERT OR UPDATE OR DELETE ON fiscal.certificates
FOR EACH ROW
EXECUTE FUNCTION fiscal.invalidate_production_readiness();

CREATE TRIGGER fiscal_points_of_sale_invalidate_production
AFTER INSERT OR UPDATE OR DELETE ON fiscal.points_of_sale
FOR EACH ROW
EXECUTE FUNCTION fiscal.invalidate_production_readiness();

ALTER TABLE fiscal.homologation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.homologation_runs FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal.homologation_checks ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal.homologation_checks FORCE ROW LEVEL SECURITY;

CREATE POLICY fiscal_homologation_runs_tenant_policy
ON fiscal.homologation_runs
USING (org_id = app.current_org_id())
WITH CHECK (org_id = app.current_org_id());

CREATE POLICY fiscal_homologation_checks_tenant_policy
ON fiscal.homologation_checks
USING (org_id = app.current_org_id())
WITH CHECK (org_id = app.current_org_id());

COMMENT ON TABLE fiscal.homologation_runs IS
'Read-only ARCA interoperability runs. Evidence is technical and is not an approval or homologation granted by ARCA.';

COMMENT ON TABLE fiscal.homologation_checks IS
'Immutable, redacted evidence produced by read-only ARCA probes and local validation checks.';

REVOKE ALL ON TABLE fiscal.homologation_runs FROM PUBLIC;
REVOKE ALL ON TABLE fiscal.homologation_checks FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.protect_homologation_run() FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.protect_homologation_check() FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.invalidate_production_readiness() FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_backend'
    ) THEN
        GRANT SELECT, INSERT, UPDATE
        ON TABLE fiscal.homologation_runs
        TO pymes_backend;

        GRANT SELECT, INSERT
        ON TABLE fiscal.homologation_checks
        TO pymes_backend;
    END IF;
END
$grant$;
