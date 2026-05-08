-- Bootstrap autónomo del tenant demo local.
-- Si el tenant/usuario no existen, los crea; si ya existen, los normaliza.
-- El placeholder __SEED_TENANT_ID__ se resuelve antes de ejecutar el SQL.

DO $$
DECLARE
    v_tenant uuid := '__SEED_TENANT_ID__';
    v_owner uuid;
    v_user uuid;
BEGIN
    INSERT INTO tenants (id, external_id, name, slug, created_at, updated_at)
    VALUES (
        v_tenant,
        NULLIF('__SEED_TENANT_EXTERNAL_ID__', ''),
        '__SEED_TENANT_NAME__',
        '__SEED_TENANT_SLUG__',
        now(),
        now()
    )
    ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            slug = EXCLUDED.slug,
            external_id = COALESCE(EXCLUDED.external_id, tenants.external_id),
            updated_at = now();

    SELECT id INTO v_owner
    FROM users
    WHERE external_id = '__SEED_OWNER_EXTERNAL_ID__'
    LIMIT 1;

    IF v_owner IS NULL THEN
        INSERT INTO users (
            id, external_id, email, name, avatar_url, phone,
            given_name, family_name, created_at, updated_at
        )
        VALUES (
            gen_random_uuid(),
            '__SEED_OWNER_EXTERNAL_ID__',
            '__SEED_OWNER_EMAIL__',
            trim('__SEED_OWNER_GIVEN_NAME__' || ' ' || '__SEED_OWNER_FAMILY_NAME__'),
            '',
            '',
            '__SEED_OWNER_GIVEN_NAME__',
            '__SEED_OWNER_FAMILY_NAME__',
            now(),
            now()
        )
        RETURNING id INTO v_owner;
    ELSE
        UPDATE users
           SET email = '__SEED_OWNER_EMAIL__',
               name = trim('__SEED_OWNER_GIVEN_NAME__' || ' ' || '__SEED_OWNER_FAMILY_NAME__'),
               avatar_url = '',
               given_name = '__SEED_OWNER_GIVEN_NAME__',
               family_name = '__SEED_OWNER_FAMILY_NAME__',
               deleted_at = NULL,
               updated_at = now()
         WHERE id = v_owner;
    END IF;

    UPDATE tenant_memberships
       SET status = 'removed',
           removed_at = COALESCE(removed_at, now()),
           updated_at = now()
     WHERE tenant_id = v_tenant
       AND role = 'owner'
       AND status = 'active'
       AND user_id <> v_owner;

    INSERT INTO tenant_memberships (tenant_id, user_id, role, status, removed_at, created_at, updated_at)
    VALUES (v_tenant, v_owner, 'owner', 'active', NULL, now(), now())
    ON CONFLICT (tenant_id, user_id) WHERE status = 'active' DO UPDATE
        SET role = 'owner',
            status = 'active',
            removed_at = NULL,
            updated_at = now();

    SELECT id INTO v_user
    FROM users
    WHERE external_id = 'user_local_admin'
    LIMIT 1;

    IF v_user IS NULL THEN
        INSERT INTO users (
            id, external_id, email, name, avatar_url, phone,
            given_name, family_name, created_at, updated_at
        )
        VALUES (
            '00000000-0000-0000-0000-000000000002',
            'user_local_admin',
            'admin@local.dev',
            'Local Admin',
            '',
            '+5493810000000',
            'Local',
            'Admin',
            now(),
            now()
        )
        RETURNING id INTO v_user;
    ELSE
        UPDATE users
           SET email = 'admin@local.dev',
               name = 'Local Admin',
               avatar_url = '',
               phone = '+5493810000000',
               given_name = 'Local',
               family_name = 'Admin',
               deleted_at = NULL,
               updated_at = now()
         WHERE id = v_user;
    END IF;

    INSERT INTO tenant_memberships (tenant_id, user_id, role, created_at)
    VALUES (v_tenant, v_user, 'admin', now())
    ON CONFLICT (tenant_id, user_id) WHERE status = 'active' DO UPDATE
        SET role = EXCLUDED.role,
            updated_at = now();

    INSERT INTO tenant_settings (tenant_id, plan_code)
    VALUES (v_tenant, 'starter')
    ON CONFLICT (tenant_id) DO NOTHING;

    INSERT INTO tenant_api_keys (id, tenant_id, name, api_key_hash, key_prefix, created_by)
    VALUES (
        '00000000-0000-0000-0000-000000000004',
        v_tenant,
        'local-dev-key',
        '91678ad136f46807fd001e50281fcc842e4b40388a83a85c5ea069c4383e739a',
        'psk_local_adm',
        'seed'
    )
    ON CONFLICT (api_key_hash) DO UPDATE SET
        tenant_id = EXCLUDED.tenant_id,
        name = EXCLUDED.name,
        key_prefix = EXCLUDED.key_prefix;

    INSERT INTO tenant_api_key_scopes (id, api_key_id, scope) VALUES
        (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:read'),
        (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:write')
    ON CONFLICT (api_key_id, scope) DO NOTHING;
END $$;
