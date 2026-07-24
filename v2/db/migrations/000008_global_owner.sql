-- Product-wide owners are Pymes authorities, not tenant memberships. Existing
-- tenant owners are promoted once and their organization membership becomes
-- the initial tenant administrator.

CREATE TABLE app.global_user_roles (
    user_id uuid PRIMARY KEY REFERENCES iam.users(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role = 'owner'),
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    disabled_at timestamptz,
    CHECK (
        (status = 'disabled' AND disabled_at IS NOT NULL)
        OR (status = 'active' AND disabled_at IS NULL)
    )
);

INSERT INTO app.global_user_roles (user_id, role, status)
SELECT DISTINCT membership.user_id, 'owner', 'active'
FROM iam.memberships AS membership
JOIN iam.users AS iam_user ON iam_user.id = membership.user_id
WHERE membership.role = 'owner'
  AND membership.status = 'active'
  AND iam_user.status = 'active'
ON CONFLICT (user_id) DO NOTHING;

DROP TRIGGER IF EXISTS iam_memberships_owner_invariant ON iam.memberships;
DROP TRIGGER IF EXISTS iam_organizations_owner_invariant ON iam.organizations;
DROP INDEX IF EXISTS iam.iam_memberships_single_active_owner_uidx;
DROP FUNCTION IF EXISTS iam.enforce_membership_owner_invariant();
DROP FUNCTION IF EXISTS iam.enforce_organization_owner_invariant();
DROP FUNCTION IF EXISTS iam.assert_single_active_owner(uuid);

UPDATE iam.memberships SET role = 'admin', updated_at = now()
WHERE role = 'owner';
UPDATE iam.invitations SET role = 'admin', updated_at = now()
WHERE role = 'owner';

ALTER TABLE iam.memberships
    DROP CONSTRAINT IF EXISTS iam_memberships_pymes_role_check;
ALTER TABLE iam.memberships
    ADD CONSTRAINT iam_memberships_pymes_role_check
    CHECK (role IN ('admin', 'member'));

ALTER TABLE iam.invitations
    DROP CONSTRAINT IF EXISTS iam_invitations_pymes_role_check;
ALTER TABLE iam.invitations
    ADD CONSTRAINT iam_invitations_pymes_role_check
    CHECK (role IN ('admin', 'member'));

DROP TRIGGER IF EXISTS iam_invitations_bootstrap_owner_guard ON iam.invitations;
DROP FUNCTION IF EXISTS app.enforce_bootstrap_owner_invitation();

CREATE OR REPLACE FUNCTION app.is_global_owner(
    requested_provider text,
    requested_subject text
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, app, iam
AS $function$
    SELECT EXISTS (
        SELECT 1
        FROM app.global_user_roles AS global_role
        JOIN iam.users AS iam_user ON iam_user.id = global_role.user_id
        WHERE iam_user.provider = btrim(requested_provider)
          AND iam_user.external_id = btrim(requested_subject)
          AND iam_user.status = 'active'
          AND iam_user.email_verified
          AND global_role.role = 'owner'
          AND global_role.status = 'active'
          AND btrim(requested_provider) <> ''
          AND btrim(requested_subject) <> ''
    )
$function$;

REVOKE ALL ON FUNCTION app.is_global_owner(text, text) FROM PUBLIC;

DROP POLICY IF EXISTS iam_organizations_tenant_policy ON iam.organizations;
CREATE POLICY iam_organizations_tenant_policy
ON iam.organizations
USING (
    id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
)
WITH CHECK (
    id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
);

DROP POLICY IF EXISTS iam_memberships_tenant_policy ON iam.memberships;
CREATE POLICY iam_memberships_tenant_policy
ON iam.memberships
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
);

DROP POLICY IF EXISTS iam_invitations_tenant_policy ON iam.invitations;
CREATE POLICY iam_invitations_tenant_policy
ON iam.invitations
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
    OR app.is_global_owner(
        current_setting('app.actor_provider', true),
        current_setting('app.actor_subject', true)
    )
);

CREATE INDEX app_global_user_roles_status_idx
    ON app.global_user_roles (status, user_id);

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
SET search_path = pg_catalog, app, iam
AS $function$
    SELECT
        membership.id,
        organization.id,
        iam_user.id,
        CASE
            WHEN global_role.status = 'active' THEN 'owner'
            ELSE membership.role
        END
    FROM iam.organizations AS organization
    JOIN iam.memberships AS membership ON membership.org_id = organization.id
    JOIN iam.users AS iam_user ON iam_user.id = membership.user_id
    LEFT JOIN app.global_user_roles AS global_role
      ON global_role.user_id = iam_user.id AND global_role.role = 'owner'
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
SET search_path = pg_catalog, app, iam
AS $function$
    SELECT
        organization.id,
        organization.external_id,
        organization.name,
        organization.slug,
        membership.id,
        CASE
            WHEN global_role.status = 'active' THEN 'owner'
            ELSE membership.role
        END
    FROM iam.users AS iam_user
    JOIN iam.memberships AS membership ON membership.user_id = iam_user.id
    JOIN iam.organizations AS organization ON organization.id = membership.org_id
    LEFT JOIN app.global_user_roles AS global_role
      ON global_role.user_id = iam_user.id AND global_role.role = 'owner'
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

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
        ON TABLE app.global_user_roles
        TO pymes_backend;
        GRANT EXECUTE ON FUNCTION app.is_global_owner(text, text)
        TO pymes_backend;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_iam_worker') THEN
        GRANT SELECT ON TABLE app.global_user_roles TO pymes_iam_worker;
        GRANT EXECUTE ON FUNCTION app.is_global_owner(text, text)
        TO pymes_iam_worker;
    END IF;
END
$grant$;
