-- Harden and rename the Pymes access boundary.
-- Pymes is the SaaS application; rows in this boundary are customer tenants,
-- tenant memberships, tenant API keys and tenant invitations.

DO $$
BEGIN
    IF to_regclass('public.tenants') IS NULL AND to_regclass('public.orgs') IS NOT NULL THEN
        ALTER TABLE orgs RENAME TO tenants;
    END IF;
    IF to_regclass('public.tenant_memberships') IS NULL AND to_regclass('public.org_members') IS NOT NULL THEN
        ALTER TABLE org_members RENAME TO tenant_memberships;
    END IF;
    IF to_regclass('public.tenant_api_keys') IS NULL AND to_regclass('public.org_api_keys') IS NOT NULL THEN
        ALTER TABLE org_api_keys RENAME TO tenant_api_keys;
    END IF;
    IF to_regclass('public.tenant_api_key_scopes') IS NULL AND to_regclass('public.org_api_key_scopes') IS NOT NULL THEN
        ALTER TABLE org_api_key_scopes RENAME TO tenant_api_key_scopes;
    END IF;
    IF to_regclass('public.tenant_usage_counters') IS NULL AND to_regclass('public.org_usage_counters') IS NOT NULL THEN
        ALTER TABLE org_usage_counters RENAME TO tenant_usage_counters;
    END IF;
END $$;

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS clerk_org_id text;

UPDATE tenants
SET clerk_org_id = external_id
WHERE clerk_org_id IS NULL
  AND external_id IS NOT NULL
  AND external_id LIKE 'org_%';

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_clerk_org_id
    ON tenants(clerk_org_id)
    WHERE clerk_org_id IS NOT NULL AND clerk_org_id <> '';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_memberships'
          AND column_name = 'org_id'
    ) THEN
        ALTER TABLE tenant_memberships RENAME COLUMN org_id TO tenant_id;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_settings'
          AND column_name = 'org_id'
    ) THEN
        ALTER TABLE tenant_settings RENAME COLUMN org_id TO tenant_id;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_api_keys'
          AND column_name = 'org_id'
    ) THEN
        ALTER TABLE tenant_api_keys RENAME COLUMN org_id TO tenant_id;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_usage_counters'
          AND column_name = 'org_id'
    ) THEN
        ALTER TABLE tenant_usage_counters RENAME COLUMN org_id TO tenant_id;
    END IF;
END $$;

ALTER TABLE tenant_memberships
    ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS removed_at timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

UPDATE tenant_memberships
SET role = CASE
    WHEN role IN ('owner', 'admin', 'member') THEN role
    WHEN role IN ('org:admin', 'admin:console') THEN 'admin'
    ELSE 'member'
END,
status = COALESCE(NULLIF(status, ''), 'active');

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM tenants t
        WHERE NOT EXISTS (
            SELECT 1
            FROM tenant_memberships tm
            WHERE tm.tenant_id = t.id
              AND tm.status = 'active'
        )
    ) THEN
        RAISE EXCEPTION 'tenant_access_model: every tenant must have at least one active member before migration';
    END IF;
END $$;

WITH ranked AS (
    SELECT
        id,
        tenant_id,
        role,
        row_number() OVER (
            PARTITION BY tenant_id
            ORDER BY
                CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
                created_at ASC,
                id ASC
        ) AS rn
    FROM tenant_memberships
    WHERE status = 'active'
)
UPDATE tenant_memberships tm
SET role = CASE WHEN ranked.rn = 1 THEN 'owner' ELSE CASE WHEN ranked.role = 'owner' THEN 'admin' ELSE ranked.role END END,
    updated_at = now()
FROM ranked
WHERE ranked.id = tm.id;

ALTER TABLE tenant_memberships
    DROP CONSTRAINT IF EXISTS org_members_role_check,
    DROP CONSTRAINT IF EXISTS tenant_memberships_role_check,
    ADD CONSTRAINT tenant_memberships_role_check CHECK (role IN ('owner', 'admin', 'member'));

ALTER TABLE tenant_memberships
    DROP CONSTRAINT IF EXISTS org_members_status_check,
    DROP CONSTRAINT IF EXISTS tenant_memberships_status_check,
    ADD CONSTRAINT tenant_memberships_status_check CHECK (status IN ('active', 'removed'));

ALTER TABLE tenant_memberships
    DROP CONSTRAINT IF EXISTS org_members_org_id_user_id_key,
    DROP CONSTRAINT IF EXISTS tenant_memberships_tenant_id_user_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_memberships_one_active_owner
    ON tenant_memberships(tenant_id)
    WHERE role = 'owner' AND status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_memberships_active_user
    ON tenant_memberships(tenant_id, user_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_tenant_memberships_tenant_status
    ON tenant_memberships(tenant_id, status);

CREATE TABLE IF NOT EXISTS tenant_invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email_normalized text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'member')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    token_hash text NOT NULL UNIQUE,
    clerk_invitation_id text,
    invited_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    accepted_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_invitations_pending_email
    ON tenant_invitations(tenant_id, email_normalized)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_tenant_invitations_tenant_status
    ON tenant_invitations(tenant_id, status, created_at DESC);
