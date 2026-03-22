-- Seed local para taller mecánico (auto_repair): vehículo, servicios, órdenes de trabajo.
-- Usa la misma org que el control plane local: 00000000-0000-0000-0000-000000000001
-- y el cliente demo c1 de pymes-core (0007_core_seed).
-- Requiere 0001_workshops_schema + 0002_bike_shop (columna segment en services).

DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    c1 uuid := '10000000-0000-0000-0000-000000000001';
    p1 uuid := '12000000-0000-0000-0000-000000000001';
    veh1 uuid := '30000000-0000-0000-0000-000000000001';
    srv1 uuid := '30000000-0000-0000-0000-000000000010';
    srv2 uuid := '30000000-0000-0000-0000-000000000011';
    wo1 uuid := '30000000-0000-0000-0000-000000000020';
    wo2 uuid := '30000000-0000-0000-0000-000000000021';
    woi1 uuid := '30000000-0000-0000-0000-000000000030';
    woi2 uuid := '30000000-0000-0000-0000-000000000031';
BEGIN
    INSERT INTO workshops.vehicles (id, org_id, customer_id, customer_name, license_plate, vin, make, model, year, kilometers, color, notes)
    VALUES (
        veh1, v_org, c1, 'Cliente Demo Uno', 'AB 123 CD', '9BWZZZ377VT004251', 'Ford', 'Focus', 2018, 98500, 'Gris',
        'Vehículo semilla taller'
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.services (id, org_id, segment, code, name, description, category, estimated_hours, base_price, currency, tax_rate, linked_product_id, is_active)
    VALUES
        (srv1, v_org, 'auto_repair', 'SRV-OIL', 'Cambio de aceite y filtro', 'Servicio estándar', 'mantenimiento', 0.5, 25000, 'ARS', 21, NULL, true),
        (srv2, v_org, 'auto_repair', 'SRV-BRAKE', 'Revisión de frenos', 'Inspección y ajuste', 'frenos', 1.5, 45000, 'ARS', 21, p1, true)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_orders (
        id, org_id, number, vehicle_id, vehicle_plate, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency,
        subtotal_services, subtotal_parts, tax_total, total, created_by
    )
    VALUES (
        wo1, v_org, 'OT-SEED-001', veh1, 'AB 123 CD', c1, 'Cliente Demo Uno', 'received',
        'Cambio de aceite y ruido al frenar', '', 'Orden abierta (semilla)', '', 'ARS',
        25000, 15000, 8400, 48400, 'seed'
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_orders (
        id, org_id, number, vehicle_id, vehicle_plate, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency,
        subtotal_services, subtotal_parts, tax_total, total, created_by
    )
    VALUES (
        wo2, v_org, 'OT-SEED-002', veh1, 'AB 123 CD', c1, 'Cliente Demo Uno', 'in_progress',
        'Service 20.000 km', 'Pastillas delanteras al límite', 'En taller', 'Prioridad media', 'ARS',
        45000, 0, 9450, 54450, 'seed'
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        (woi1, v_org, wo1, 'service', srv1, NULL, 'Cambio de aceite y filtro', 1, 25000, 21, 0, '{}'::jsonb),
        (woi2, v_org, wo1, 'part', NULL, p1, 'Producto Demo A (repuesto)', 1, 15000, 21, 1, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        ('30000000-0000-0000-0000-000000000032', v_org, wo2, 'service', srv2, NULL, 'Revisión de frenos', 1, 45000, 21, 0, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;
END $$;
