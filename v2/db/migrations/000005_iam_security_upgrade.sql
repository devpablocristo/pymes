-- Forward-only hardening for development databases that already applied the
-- initial IAM and provisioning migrations.

CREATE OR REPLACE FUNCTION iam.resolve_active_membership(
    requested_provider text,
    requested_subject text,
    requested_external_org_id text
)
RETURNS TABLE (
    membership_id uuid,
    organization_id uuid,
    user_id uuid,
    role text
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, iam
AS $function$
    SELECT
        membership.id,
        organization.id,
        iam_user.id,
        membership.role
    FROM iam.organizations AS organization
    JOIN iam.memberships AS membership
      ON membership.org_id = organization.id
    JOIN iam.users AS iam_user
      ON iam_user.id = membership.user_id
    WHERE organization.provider = btrim(requested_provider)
      AND organization.external_id = btrim(requested_external_org_id)
      AND organization.status = 'active'
      AND iam_user.provider = btrim(requested_provider)
      AND iam_user.external_id = btrim(requested_subject)
      AND iam_user.status = 'active'
      AND iam_user.email_verified
      AND membership.provider = btrim(requested_provider)
      AND membership.status = 'active'
      AND btrim(requested_provider) <> ''
      AND btrim(requested_subject) <> ''
      AND btrim(requested_external_org_id) <> ''
    LIMIT 1
$function$;

CREATE OR REPLACE FUNCTION iam.list_active_organizations(
    requested_provider text,
    requested_subject text
)
RETURNS TABLE (
    organization_id uuid,
    external_organization_id text,
    organization_name text,
    organization_slug text,
    membership_id uuid,
    role text
)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, iam
AS $function$
    SELECT
        organization.id,
        organization.external_id,
        organization.name,
        organization.slug,
        membership.id,
        membership.role
    FROM iam.users AS iam_user
    JOIN iam.memberships AS membership
      ON membership.user_id = iam_user.id
    JOIN iam.organizations AS organization
      ON organization.id = membership.org_id
    WHERE iam_user.provider = btrim(requested_provider)
      AND iam_user.external_id = btrim(requested_subject)
      AND iam_user.status = 'active'
      AND iam_user.email_verified
      AND membership.provider = btrim(requested_provider)
      AND membership.status = 'active'
      AND organization.provider = btrim(requested_provider)
      AND organization.status = 'active'
      AND organization.external_id IS NOT NULL
      AND btrim(requested_provider) <> ''
      AND btrim(requested_subject) <> ''
    ORDER BY lower(organization.name), organization.id
$function$;

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

DROP TRIGGER IF EXISTS iam_invitations_bootstrap_owner_guard ON iam.invitations;
CREATE TRIGGER iam_invitations_bootstrap_owner_guard
BEFORE INSERT OR UPDATE OF org_id, email_normalized, role
ON iam.invitations
FOR EACH ROW
EXECUTE FUNCTION app.enforce_bootstrap_owner_invitation();

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
        ON TABLE public.platform_idempotency_records
        TO pymes_backend;
    END IF;
END
$grant$;
