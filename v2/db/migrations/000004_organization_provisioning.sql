CREATE TABLE app.organization_provisioning_requests (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL UNIQUE,
    provider text NOT NULL CHECK (btrim(provider) <> ''),
    slug text NOT NULL UNIQUE,
    organization_name text NOT NULL CHECK (btrim(organization_name) <> ''),
    owner_email_normalized text NOT NULL
        CHECK (
            owner_email_normalized = lower(btrim(owner_email_normalized))
            AND btrim(owner_email_normalized) <> ''
        ),
    payload_sha256 char(64) NOT NULL
        CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    outbox_message_id text NOT NULL UNIQUE
        CHECK (btrim(outbox_message_id) <> ''),
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'provisioned', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT app_organization_provisioning_requests_slug_normalized
        CHECK (
            slug = lower(btrim(slug))
            AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
        ),
    CONSTRAINT app_organization_provisioning_requests_organization_fk
        FOREIGN KEY (organization_id)
        REFERENCES iam.organizations(id)
        DEFERRABLE INITIALLY DEFERRED
);

REVOKE ALL ON TABLE app.organization_provisioning_requests FROM PUBLIC;

CREATE VIEW app.organization_provisioning_outbox_messages
WITH (security_invoker = true)
AS
SELECT *
FROM public.platform_outbox_messages
WHERE topic = 'iam.organization.provision.requested.v1'
WITH LOCAL CHECK OPTION;

REVOKE ALL ON TABLE app.organization_provisioning_outbox_messages FROM PUBLIC;

CREATE OR REPLACE FUNCTION app.enforce_bootstrap_owner_invitation()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, iam
AS $function$
BEGIN
    IF NEW.role <> 'owner' THEN
        RETURN NEW;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM app.organization_provisioning_requests AS request
        JOIN iam.organizations AS organization
          ON organization.id = request.organization_id
        WHERE request.organization_id = NEW.org_id
          AND request.owner_email_normalized = NEW.email_normalized
          AND request.status IN ('queued', 'provisioned')
          AND organization.status = 'provisioning'
    ) THEN
        RAISE EXCEPTION 'owner invitations are restricted to organization provisioning'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'iam_invitations_owner_requires_provisioning';
    END IF;
    RETURN NEW;
END
$function$;

REVOKE ALL ON FUNCTION app.enforce_bootstrap_owner_invitation() FROM PUBLIC;

CREATE TRIGGER iam_invitations_bootstrap_owner_guard
BEFORE INSERT OR UPDATE OF org_id, email_normalized, role
ON iam.invitations
FOR EACH ROW
EXECUTE FUNCTION app.enforce_bootstrap_owner_invitation();

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_migrator') THEN
        GRANT USAGE ON SCHEMA app TO pymes_migrator;
        GRANT SELECT, INSERT, UPDATE
        ON TABLE app.organization_provisioning_requests
        TO pymes_migrator;
        GRANT SELECT, UPDATE
        ON TABLE app.organization_provisioning_outbox_messages
        TO pymes_migrator;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT USAGE ON SCHEMA app TO pymes_backend;
        GRANT SELECT, UPDATE
        ON TABLE app.organization_provisioning_requests
        TO pymes_backend;
        GRANT SELECT, UPDATE
        ON TABLE app.organization_provisioning_outbox_messages
        TO pymes_backend;
    END IF;
END
$grant$;
