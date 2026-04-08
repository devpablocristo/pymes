-- Roles, permissions, user_roles, default price list (demo).
-- Role/list IDs per org (uuid v5) for multi-tenant.

DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    v_user uuid := '00000000-0000-0000-0000-000000000002';

    r_admin uuid;
    r_seller uuid;
    r_cashier uuid;
    r_accountant uuid;
    r_warehouse uuid;

    pl_default uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    r_admin := uuid_generate_v5(v_org, 'pymes-seed/v1/role/admin');
    r_seller := uuid_generate_v5(v_org, 'pymes-seed/v1/role/vendedor');
    r_cashier := uuid_generate_v5(v_org, 'pymes-seed/v1/role/cajero');
    r_accountant := uuid_generate_v5(v_org, 'pymes-seed/v1/role/contador');
    r_warehouse := uuid_generate_v5(v_org, 'pymes-seed/v1/role/almacenero');
    pl_default := uuid_generate_v5(v_org, 'pymes-seed/v1/price-list/default');

    INSERT INTO roles (id, org_id, name, description, is_system)
    VALUES
        (r_admin, v_org, 'admin', 'Full access', true),
        (r_seller, v_org, 'seller', 'Sales and commercial management', true),
        (r_cashier, v_org, 'cashier', 'Payments and cash register', true),
        (r_accountant, v_org, 'accountant', 'Reports and accounting', true),
        (r_warehouse, v_org, 'warehouse', 'Inventory and products', true)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO role_permissions (id, role_id, resource, action)
    VALUES
        (gen_random_uuid(), r_admin, '*', '*'),

        (gen_random_uuid(), r_seller, 'customers', 'read'),
        (gen_random_uuid(), r_seller, 'customers', 'create'),
        (gen_random_uuid(), r_seller, 'customers', 'update'),
        (gen_random_uuid(), r_seller, 'products', 'read'),
        (gen_random_uuid(), r_seller, 'sales', 'read'),
        (gen_random_uuid(), r_seller, 'sales', 'create'),
        (gen_random_uuid(), r_seller, 'quotes', 'read'),
        (gen_random_uuid(), r_seller, 'quotes', 'create'),
        (gen_random_uuid(), r_seller, 'quotes', 'update'),
        (gen_random_uuid(), r_seller, 'inventory', 'read'),
        (gen_random_uuid(), r_seller, 'scheduling', 'read'),
        (gen_random_uuid(), r_seller, 'scheduling', 'create'),
        (gen_random_uuid(), r_seller, 'scheduling', 'update'),
        (gen_random_uuid(), r_seller, 'returns', 'read'),
        (gen_random_uuid(), r_seller, 'returns', 'create'),
        (gen_random_uuid(), r_seller, 'accounts', 'read'),
        (gen_random_uuid(), r_seller, 'price_lists', 'read'),

        (gen_random_uuid(), r_cashier, 'sales', 'read'),
        (gen_random_uuid(), r_cashier, 'sales', 'create'),
        (gen_random_uuid(), r_cashier, 'cashflow', 'read'),
        (gen_random_uuid(), r_cashier, 'cashflow', 'create'),
        (gen_random_uuid(), r_cashier, 'customers', 'read'),
        (gen_random_uuid(), r_cashier, 'payments', 'read'),
        (gen_random_uuid(), r_cashier, 'payments', 'create'),
        (gen_random_uuid(), r_cashier, 'returns', 'read'),
        (gen_random_uuid(), r_cashier, 'returns', 'create'),
        (gen_random_uuid(), r_cashier, 'accounts', 'read'),
        (gen_random_uuid(), r_cashier, 'scheduling', 'read'),
        (gen_random_uuid(), r_cashier, 'scheduling', 'operate'),

        (gen_random_uuid(), r_accountant, 'reports', 'read'),
        (gen_random_uuid(), r_accountant, 'cashflow', 'read'),
        (gen_random_uuid(), r_accountant, 'sales', 'read'),
        (gen_random_uuid(), r_accountant, 'billing', 'read'),
        (gen_random_uuid(), r_accountant, 'audit', 'read'),
        (gen_random_uuid(), r_accountant, 'audit', 'export'),
        (gen_random_uuid(), r_accountant, 'purchases', 'read'),
        (gen_random_uuid(), r_accountant, 'accounts', 'read'),
        (gen_random_uuid(), r_accountant, 'payments', 'read'),
        (gen_random_uuid(), r_accountant, 'returns', 'read'),
        (gen_random_uuid(), r_accountant, 'recurring', 'read'),
        (gen_random_uuid(), r_accountant, 'price_lists', 'read'),
        (gen_random_uuid(), r_accountant, 'procurement_requests', 'read'),
        (gen_random_uuid(), r_accountant, 'procurement_requests', 'approve'),
        (gen_random_uuid(), r_accountant, 'procurement_requests', 'reject'),
        (gen_random_uuid(), r_accountant, 'procurement_policies', 'read'),

        (gen_random_uuid(), r_warehouse, 'inventory', 'read'),
        (gen_random_uuid(), r_warehouse, 'inventory', 'create'),
        (gen_random_uuid(), r_warehouse, 'inventory', 'update'),
        (gen_random_uuid(), r_warehouse, 'products', 'read'),
        (gen_random_uuid(), r_warehouse, 'purchases', 'read'),
        (gen_random_uuid(), r_warehouse, 'returns', 'read'),
        (gen_random_uuid(), r_warehouse, 'procurement_requests', 'read'),
        (gen_random_uuid(), r_warehouse, 'procurement_requests', 'create'),
        (gen_random_uuid(), r_warehouse, 'procurement_requests', 'update'),
        (gen_random_uuid(), r_warehouse, 'procurement_requests', 'submit'),
        (gen_random_uuid(), r_warehouse, 'procurement_policies', 'read'),
        (gen_random_uuid(), r_warehouse, 'procurement_policies', 'create'),
        (gen_random_uuid(), r_warehouse, 'procurement_policies', 'update'),
        (gen_random_uuid(), r_warehouse, 'procurement_policies', 'delete')
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
    VALUES (pl_default, v_org, 'Retail', 'Default local price list', true, 0, true)
    ON CONFLICT (id) DO NOTHING;
END $$;
