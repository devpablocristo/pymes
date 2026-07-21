-- Complete the storage rename for databases that had already passed the
-- access-model migration while still carrying the old org_* table/column names.

DO $$
DECLARE
    tbl text;
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

    FOR tbl IN
        SELECT table_name
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND column_name = 'org_id'
          AND table_name NOT LIKE 'scheduling_%'
          AND table_name NOT IN ('modules_scheduling_schema_migrations')
        ORDER BY table_name
    LOOP
        EXECUTE format('ALTER TABLE %I RENAME COLUMN org_id TO tenant_id', tbl);
    END LOOP;
END $$;

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS clerk_org_id text;

UPDATE tenants
SET clerk_org_id = external_id
WHERE clerk_org_id IS NULL
  AND external_id IS NOT NULL
  AND external_id LIKE 'org_%';

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

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_clerk_org_id
    ON tenants(clerk_org_id)
    WHERE clerk_org_id IS NOT NULL AND clerk_org_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_memberships_one_active_owner
    ON tenant_memberships(tenant_id)
    WHERE role = 'owner' AND status = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_memberships_active_user
    ON tenant_memberships(tenant_id, user_id)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_tenant_memberships_tenant_status
    ON tenant_memberships(tenant_id, status);
