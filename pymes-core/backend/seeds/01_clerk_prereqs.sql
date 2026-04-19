-- Bootstrap autónomo del tenant demo local.
-- Si la org/usuario no existen, los crea; si ya existen, los normaliza.
-- El placeholder __SEED_ORG_ID__ se resuelve antes de ejecutar el SQL.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    v_user uuid;
BEGIN
    INSERT INTO orgs (id, external_id, name, slug, created_at, updated_at)
    VALUES (
        v_org,
        '__SEED_ORG_EXTERNAL_ID__',
        '__SEED_ORG_NAME__',
        '__SEED_ORG_SLUG__',
        now(),
        now()
    )
    ON CONFLICT (external_id) DO UPDATE
        SET name = EXCLUDED.name,
            slug = EXCLUDED.slug,
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

    INSERT INTO org_members (org_id, user_id, role, created_at)
    VALUES (v_org, v_user, 'admin', now())
    ON CONFLICT (org_id, user_id) DO UPDATE
        SET role = EXCLUDED.role;

    INSERT INTO tenant_settings (org_id, plan_code)
    VALUES (v_org, 'starter')
    ON CONFLICT (org_id) DO NOTHING;

    INSERT INTO org_api_keys (id, org_id, name, api_key_hash, key_prefix, created_by)
    VALUES (
        '00000000-0000-0000-0000-000000000004',
        v_org,
        'local-dev-key',
        '91678ad136f46807fd001e50281fcc842e4b40388a83a85c5ea069c4383e739a',
        'psk_local_adm',
        'seed'
    )
    ON CONFLICT (api_key_hash) DO UPDATE SET
        org_id = EXCLUDED.org_id,
        name = EXCLUDED.name,
        key_prefix = EXCLUDED.key_prefix;

    INSERT INTO org_api_key_scopes (id, api_key_id, scope) VALUES
        (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:read'),
        (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:write')
    ON CONFLICT (api_key_id, scope) DO NOTHING;
END $$;
