-- Pymes adopts Platform's canonical lifecycle for product administration:
--
--   active <-> archived
--   active/archived -> trashed -> active
--   trashed -> purged
--
-- Operational IAM status remains separate. Lifecycle state controls whether a
-- tenant or user participates in authentication and active product workflows.

ALTER TABLE iam.organizations
    ADD COLUMN archived_at timestamptz,
    ADD COLUMN trashed_at timestamptz,
    ADD COLUMN purge_after timestamptz,
    ADD CONSTRAINT iam_organizations_lifecycle_valid CHECK (
        NOT (archived_at IS NOT NULL AND trashed_at IS NOT NULL)
        AND (purge_after IS NULL OR trashed_at IS NOT NULL)
    );

ALTER TABLE iam.users
    ADD COLUMN archived_at timestamptz,
    ADD COLUMN trashed_at timestamptz,
    ADD COLUMN purge_after timestamptz,
    ADD CONSTRAINT iam_users_lifecycle_valid CHECK (
        NOT (archived_at IS NOT NULL AND trashed_at IS NOT NULL)
        AND (purge_after IS NULL OR trashed_at IS NOT NULL)
    );

CREATE INDEX iam_organizations_lifecycle_idx
    ON iam.organizations (archived_at, trashed_at, created_at);
CREATE INDEX iam_users_lifecycle_idx
    ON iam.users (archived_at, trashed_at, created_at);

CREATE TABLE app.lifecycle_audit_events (
    id uuid PRIMARY KEY,
    scope_id text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid NOT NULL,
    action text NOT NULL CHECK (
        action IN ('archive', 'unarchive', 'trash', 'restore', 'purge')
    ),
    actor text NOT NULL,
    reason text,
    from_state text NOT NULL CHECK (
        from_state IN ('active', 'archived', 'trashed', 'purged')
    ),
    to_state text NOT NULL CHECK (
        to_state IN ('active', 'archived', 'trashed', 'purged')
    ),
    retention_expires_at timestamptz,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX app_lifecycle_audit_resource_idx
    ON app.lifecycle_audit_events (resource_type, resource_id, occurred_at DESC);

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
          AND iam_user.archived_at IS NULL
          AND iam_user.trashed_at IS NULL
          AND iam_user.email_verified
          AND global_role.role = 'owner'
          AND global_role.status = 'active'
          AND btrim(requested_provider) <> ''
          AND btrim(requested_subject) <> ''
    )
$function$;

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
      AND organization.archived_at IS NULL
      AND organization.trashed_at IS NULL
      AND iam_user.provider = btrim(requested_provider)
      AND iam_user.external_id = btrim(requested_subject)
      AND iam_user.status = 'active'
      AND iam_user.archived_at IS NULL
      AND iam_user.trashed_at IS NULL
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
      AND iam_user.archived_at IS NULL
      AND iam_user.trashed_at IS NULL
      AND iam_user.email_verified
      AND membership.provider = btrim(requested_provider)
      AND membership.status = 'active'
      AND organization.provider = btrim(requested_provider)
      AND organization.status = 'active'
      AND organization.archived_at IS NULL
      AND organization.trashed_at IS NULL
      AND organization.external_id IS NOT NULL
      AND btrim(requested_provider) <> ''
      AND btrim(requested_subject) <> ''
    ORDER BY lower(organization.name), organization.id
$function$;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT, INSERT
        ON TABLE app.lifecycle_audit_events
        TO pymes_backend;
    END IF;
END
$grant$;
