-- Roles, permisos, user_roles, lista de precios default (demo).
-- IDs de roles/lista por org (uuid v5) para multi-tenant.

DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    v_user uuid := '00000000-0000-0000-0000-000000000002';

    r_admin uuid;
    r_vendedor uuid;
    r_cajero uuid;
    r_contador uuid;
    r_almacenero uuid;

    pl_default uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    r_admin := uuid_generate_v5(v_org, 'pymes-seed/v1/role/admin');
    r_vendedor := uuid_generate_v5(v_org, 'pymes-seed/v1/role/vendedor');
    r_cajero := uuid_generate_v5(v_org, 'pymes-seed/v1/role/cajero');
    r_contador := uuid_generate_v5(v_org, 'pymes-seed/v1/role/contador');
    r_almacenero := uuid_generate_v5(v_org, 'pymes-seed/v1/role/almacenero');
    pl_default := uuid_generate_v5(v_org, 'pymes-seed/v1/price-list/default');

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
        (gen_random_uuid(), r_vendedor, 'appointments', 'read'),
        (gen_random_uuid(), r_vendedor, 'appointments', 'create'),
        (gen_random_uuid(), r_vendedor, 'appointments', 'update'),
        (gen_random_uuid(), r_vendedor, 'scheduling', 'read'),
        (gen_random_uuid(), r_vendedor, 'scheduling', 'create'),
        (gen_random_uuid(), r_vendedor, 'scheduling', 'update'),
        (gen_random_uuid(), r_vendedor, 'returns', 'read'),
        (gen_random_uuid(), r_vendedor, 'returns', 'create'),
        (gen_random_uuid(), r_vendedor, 'accounts', 'read'),
        (gen_random_uuid(), r_vendedor, 'price_lists', 'read'),

        (gen_random_uuid(), r_cajero, 'sales', 'read'),
        (gen_random_uuid(), r_cajero, 'sales', 'create'),
        (gen_random_uuid(), r_cajero, 'cashflow', 'read'),
        (gen_random_uuid(), r_cajero, 'cashflow', 'create'),
        (gen_random_uuid(), r_cajero, 'customers', 'read'),
        (gen_random_uuid(), r_cajero, 'payments', 'read'),
        (gen_random_uuid(), r_cajero, 'payments', 'create'),
        (gen_random_uuid(), r_cajero, 'returns', 'read'),
        (gen_random_uuid(), r_cajero, 'returns', 'create'),
        (gen_random_uuid(), r_cajero, 'accounts', 'read'),
        (gen_random_uuid(), r_cajero, 'appointments', 'read'),
        (gen_random_uuid(), r_cajero, 'scheduling', 'read'),
        (gen_random_uuid(), r_cajero, 'scheduling', 'operate'),

        (gen_random_uuid(), r_contador, 'reports', 'read'),
        (gen_random_uuid(), r_contador, 'cashflow', 'read'),
        (gen_random_uuid(), r_contador, 'sales', 'read'),
        (gen_random_uuid(), r_contador, 'billing', 'read'),
        (gen_random_uuid(), r_contador, 'audit', 'read'),
        (gen_random_uuid(), r_contador, 'audit', 'export'),
        (gen_random_uuid(), r_contador, 'purchases', 'read'),
        (gen_random_uuid(), r_contador, 'accounts', 'read'),
        (gen_random_uuid(), r_contador, 'payments', 'read'),
        (gen_random_uuid(), r_contador, 'returns', 'read'),
        (gen_random_uuid(), r_contador, 'recurring', 'read'),
        (gen_random_uuid(), r_contador, 'price_lists', 'read'),
        (gen_random_uuid(), r_contador, 'procurement_requests', 'read'),
        (gen_random_uuid(), r_contador, 'procurement_requests', 'approve'),
        (gen_random_uuid(), r_contador, 'procurement_requests', 'reject'),
        (gen_random_uuid(), r_contador, 'procurement_policies', 'read'),

        (gen_random_uuid(), r_almacenero, 'inventory', 'read'),
        (gen_random_uuid(), r_almacenero, 'inventory', 'create'),
        (gen_random_uuid(), r_almacenero, 'inventory', 'update'),
        (gen_random_uuid(), r_almacenero, 'products', 'read'),
        (gen_random_uuid(), r_almacenero, 'purchases', 'read'),
        (gen_random_uuid(), r_almacenero, 'returns', 'read'),
        (gen_random_uuid(), r_almacenero, 'procurement_requests', 'read'),
        (gen_random_uuid(), r_almacenero, 'procurement_requests', 'create'),
        (gen_random_uuid(), r_almacenero, 'procurement_requests', 'update'),
        (gen_random_uuid(), r_almacenero, 'procurement_requests', 'submit'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'read'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'create'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'update'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'delete')
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
