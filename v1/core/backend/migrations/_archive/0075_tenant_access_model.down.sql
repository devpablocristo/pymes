DROP INDEX IF EXISTS idx_tenant_invitations_tenant_status;
DROP INDEX IF EXISTS idx_tenant_invitations_pending_email;
DROP TABLE IF EXISTS tenant_invitations;

DROP INDEX IF EXISTS idx_tenant_memberships_tenant_status;
DROP INDEX IF EXISTS idx_tenant_memberships_active_user;
DROP INDEX IF EXISTS idx_tenant_memberships_one_active_owner;

ALTER TABLE tenant_memberships
    DROP CONSTRAINT IF EXISTS tenant_memberships_status_check,
    DROP CONSTRAINT IF EXISTS tenant_memberships_role_check;

ALTER TABLE tenant_memberships
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS removed_at,
    DROP COLUMN IF EXISTS status;

DROP INDEX IF EXISTS idx_tenants_clerk_org_id;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS clerk_org_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_memberships'
          AND column_name = 'tenant_id'
    ) THEN
        ALTER TABLE tenant_memberships RENAME COLUMN tenant_id TO tenant_id;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_settings'
          AND column_name = 'tenant_id'
    ) THEN
        ALTER TABLE tenant_settings RENAME COLUMN tenant_id TO tenant_id;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_api_keys'
          AND column_name = 'tenant_id'
    ) THEN
        ALTER TABLE tenant_api_keys RENAME COLUMN tenant_id TO tenant_id;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'tenant_usage_counters'
          AND column_name = 'tenant_id'
    ) THEN
        ALTER TABLE tenant_usage_counters RENAME COLUMN tenant_id TO tenant_id;
    END IF;

    IF to_regclass('public.tenant_usage_counters') IS NULL AND to_regclass('public.tenant_usage_counters') IS NOT NULL THEN
        ALTER TABLE tenant_usage_counters RENAME TO tenant_usage_counters;
    END IF;
    IF to_regclass('public.tenant_api_key_scopes') IS NULL AND to_regclass('public.tenant_api_key_scopes') IS NOT NULL THEN
        ALTER TABLE tenant_api_key_scopes RENAME TO tenant_api_key_scopes;
    END IF;
    IF to_regclass('public.tenant_api_keys') IS NULL AND to_regclass('public.tenant_api_keys') IS NOT NULL THEN
        ALTER TABLE tenant_api_keys RENAME TO tenant_api_keys;
    END IF;
    IF to_regclass('public.tenant_memberships') IS NULL AND to_regclass('public.tenant_memberships') IS NOT NULL THEN
        ALTER TABLE tenant_memberships RENAME TO tenant_memberships;
    END IF;
    IF to_regclass('public.tenants') IS NULL AND to_regclass('public.tenants') IS NOT NULL THEN
        ALTER TABLE tenants RENAME TO tenants;
    END IF;
END $$;
