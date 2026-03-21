-- RBAC: gestión de políticas CEL de procurement (separado de solicitudes).

DO $$
DECLARE
    r_almacenero uuid := '21000000-0000-0000-0000-000000000005';
    r_contador uuid := '21000000-0000-0000-0000-000000000004';
BEGIN
    INSERT INTO role_permissions (id, role_id, resource, action)
    VALUES
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'read'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'create'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'update'),
        (gen_random_uuid(), r_almacenero, 'procurement_policies', 'delete'),
        (gen_random_uuid(), r_contador, 'procurement_policies', 'read')
    ON CONFLICT (role_id, resource, action) DO NOTHING;
END $$;
