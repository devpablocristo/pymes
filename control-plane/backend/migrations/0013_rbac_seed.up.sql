DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    v_user uuid := '00000000-0000-0000-0000-000000000002';

    r_admin uuid := '21000000-0000-0000-0000-000000000001';
    r_vendedor uuid := '21000000-0000-0000-0000-000000000002';
    r_cajero uuid := '21000000-0000-0000-0000-000000000003';
    r_contador uuid := '21000000-0000-0000-0000-000000000004';
    r_almacenero uuid := '21000000-0000-0000-0000-000000000005';

    pl_default uuid := '22000000-0000-0000-0000-000000000001';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO roles (id, org_id, name, description, is_system)
    VALUES
        (r_admin, v_org, 'admin', 'Acceso total', true),
        (r_vendedor, v_org, 'vendedor', 'Gestión comercial y ventas', true),
        (r_cajero, v_org, 'cajero', 'Cobros y caja', true),
        (r_contador, v_org, 'contador', 'Reportes y contabilidad', true),
        (r_almacenero, v_org, 'almacenero', 'Inventario y productos', true)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO role_permissions (id, role_id, resource, action)
    VALUES
        (gen_random_uuid(), r_admin, '*', '*'),

        (gen_random_uuid(), r_vendedor, 'customers', 'read'),
        (gen_random_uuid(), r_vendedor, 'customers', 'create'),
        (gen_random_uuid(), r_vendedor, 'customers', 'update'),
        (gen_random_uuid(), r_vendedor, 'products', 'read'),
        (gen_random_uuid(), r_vendedor, 'sales', 'read'),
        (gen_random_uuid(), r_vendedor, 'sales', 'create'),
        (gen_random_uuid(), r_vendedor, 'quotes', 'read'),
        (gen_random_uuid(), r_vendedor, 'quotes', 'create'),
        (gen_random_uuid(), r_vendedor, 'quotes', 'update'),
        (gen_random_uuid(), r_vendedor, 'inventory', 'read'),

        (gen_random_uuid(), r_cajero, 'sales', 'read'),
        (gen_random_uuid(), r_cajero, 'sales', 'create'),
        (gen_random_uuid(), r_cajero, 'cashflow', 'read'),
        (gen_random_uuid(), r_cajero, 'cashflow', 'create'),
        (gen_random_uuid(), r_cajero, 'customers', 'read'),

        (gen_random_uuid(), r_contador, 'reports', 'read'),
        (gen_random_uuid(), r_contador, 'cashflow', 'read'),
        (gen_random_uuid(), r_contador, 'sales', 'read'),
        (gen_random_uuid(), r_contador, 'billing', 'read'),
        (gen_random_uuid(), r_contador, 'audit', 'read'),
        (gen_random_uuid(), r_contador, 'audit', 'export'),

        (gen_random_uuid(), r_almacenero, 'inventory', 'read'),
        (gen_random_uuid(), r_almacenero, 'inventory', 'create'),
        (gen_random_uuid(), r_almacenero, 'inventory', 'update'),
        (gen_random_uuid(), r_almacenero, 'products', 'read')
    ON CONFLICT (role_id, resource, action) DO NOTHING;

    IF EXISTS (
        SELECT 1
        FROM users u
        JOIN org_members om ON om.user_id = u.id
        WHERE u.id = v_user AND om.org_id = v_org
    ) THEN
        INSERT INTO user_roles (user_id, org_id, role_id, assigned_by)
        VALUES (v_user, v_org, r_admin, 'seed')
        ON CONFLICT (user_id, org_id) DO UPDATE
            SET role_id = EXCLUDED.role_id,
                assigned_by = EXCLUDED.assigned_by,
                assigned_at = now();
    END IF;

    INSERT INTO price_lists (id, org_id, name, description, is_default, markup, is_active)
    VALUES (pl_default, v_org, 'Minorista', 'Lista base local', true, 0, true)
    ON CONFLICT (id) DO NOTHING;
END $$;
