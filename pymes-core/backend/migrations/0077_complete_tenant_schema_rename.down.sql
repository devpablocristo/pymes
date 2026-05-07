DO $$
DECLARE
    tbl text;
BEGIN
    FOR tbl IN
        SELECT table_name
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND column_name = 'tenant_id'
          AND table_name NOT LIKE 'scheduling_%'
          AND table_name NOT IN (
              'modules_scheduling_schema_migrations',
              'tenant_invitations'
          )
        ORDER BY table_name
    LOOP
        EXECUTE format('ALTER TABLE %I RENAME COLUMN tenant_id TO org_id', tbl);
    END LOOP;

    IF to_regclass('public.org_usage_counters') IS NULL AND to_regclass('public.tenant_usage_counters') IS NOT NULL THEN
        ALTER TABLE tenant_usage_counters RENAME TO org_usage_counters;
    END IF;

    IF to_regclass('public.org_api_key_scopes') IS NULL AND to_regclass('public.tenant_api_key_scopes') IS NOT NULL THEN
        ALTER TABLE tenant_api_key_scopes RENAME TO org_api_key_scopes;
    END IF;

    IF to_regclass('public.org_api_keys') IS NULL AND to_regclass('public.tenant_api_keys') IS NOT NULL THEN
        ALTER TABLE tenant_api_keys RENAME TO org_api_keys;
    END IF;

    IF to_regclass('public.org_members') IS NULL AND to_regclass('public.tenant_memberships') IS NOT NULL THEN
        ALTER TABLE tenant_memberships RENAME TO org_members;
    END IF;

    IF to_regclass('public.orgs') IS NULL AND to_regclass('public.tenants') IS NOT NULL THEN
        ALTER TABLE tenants RENAME TO orgs;
    END IF;
END $$;
