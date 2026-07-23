ALTER TABLE iam.memberships
    ADD CONSTRAINT iam_memberships_pymes_role_check
    CHECK (role IN ('owner', 'admin', 'member'));

ALTER TABLE iam.invitations
    ADD CONSTRAINT iam_invitations_pymes_role_check
    CHECK (role IN ('owner', 'admin', 'member')),
    ADD CONSTRAINT iam_invitations_normalized_email_check
    CHECK (email_normalized = lower(btrim(email_normalized)));

CREATE UNIQUE INDEX iam_memberships_single_active_owner_uidx
    ON iam.memberships (org_id)
    WHERE role = 'owner' AND status = 'active';

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

REVOKE ALL ON FUNCTION iam.resolve_active_membership(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION iam.list_active_organizations(text, text) FROM PUBLIC;

CREATE OR REPLACE FUNCTION iam.assert_single_active_owner(target_org_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, iam
AS $function$
DECLARE
    owner_count integer;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM iam.organizations
        WHERE id = target_org_id
          AND status = 'active'
    ) THEN
        RETURN;
    END IF;

    SELECT count(*)
      INTO owner_count
      FROM iam.memberships
     WHERE org_id = target_org_id
       AND role = 'owner'
       AND status = 'active';

    IF owner_count <> 1 THEN
        RAISE EXCEPTION 'active organization % must have exactly one active owner', target_org_id
            USING ERRCODE = '23514',
                  CONSTRAINT = 'iam_organizations_single_active_owner';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION iam.enforce_membership_owner_invariant()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, iam
AS $function$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM iam.assert_single_active_owner(OLD.org_id);
    ELSIF TG_OP = 'INSERT' THEN
        PERFORM iam.assert_single_active_owner(NEW.org_id);
    ELSE
        PERFORM iam.assert_single_active_owner(NEW.org_id);
        IF OLD.org_id IS DISTINCT FROM NEW.org_id THEN
            PERFORM iam.assert_single_active_owner(OLD.org_id);
        END IF;
    END IF;
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION iam.enforce_organization_owner_invariant()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, iam
AS $function$
BEGIN
    PERFORM iam.assert_single_active_owner(NEW.id);
    RETURN NULL;
END
$function$;

REVOKE ALL ON FUNCTION iam.assert_single_active_owner(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION iam.enforce_membership_owner_invariant() FROM PUBLIC;
REVOKE ALL ON FUNCTION iam.enforce_organization_owner_invariant() FROM PUBLIC;

CREATE CONSTRAINT TRIGGER iam_memberships_owner_invariant
AFTER INSERT OR UPDATE OR DELETE ON iam.memberships
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION iam.enforce_membership_owner_invariant();

CREATE CONSTRAINT TRIGGER iam_organizations_owner_invariant
AFTER INSERT OR UPDATE ON iam.organizations
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION iam.enforce_organization_owner_invariant();

ALTER TABLE iam.organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.organizations FORCE ROW LEVEL SECURITY;
ALTER TABLE iam.memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.memberships FORCE ROW LEVEL SECURITY;
ALTER TABLE iam.invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE iam.invitations FORCE ROW LEVEL SECURITY;

CREATE POLICY iam_organizations_tenant_policy
ON iam.organizations
USING (id = nullif(current_setting('app.org_id', true), '')::uuid)
WITH CHECK (id = nullif(current_setting('app.org_id', true), '')::uuid);

CREATE POLICY iam_memberships_tenant_policy
ON iam.memberships
USING (org_id = nullif(current_setting('app.org_id', true), '')::uuid)
WITH CHECK (org_id = nullif(current_setting('app.org_id', true), '')::uuid);

CREATE POLICY iam_invitations_tenant_policy
ON iam.invitations
USING (org_id = nullif(current_setting('app.org_id', true), '')::uuid)
WITH CHECK (org_id = nullif(current_setting('app.org_id', true), '')::uuid);

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT USAGE ON SCHEMA app, iam TO pymes_backend;
        GRANT SELECT, INSERT, UPDATE ON
            iam.organizations,
            iam.users,
            iam.memberships,
            iam.invitations,
            iam.webhook_events
        TO pymes_backend;
        GRANT EXECUTE ON FUNCTION iam.resolve_active_membership(text, text, text)
        TO pymes_backend;
        GRANT EXECUTE ON FUNCTION iam.list_active_organizations(text, text)
        TO pymes_backend;
    END IF;
END
$grant$;
