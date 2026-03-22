-- Demo taller auto_repair: vehículo, servicios, órdenes.
-- Cliente/producto: mismas claves uuid v5 que pymes-core/seeds/02_core_business.sql.

DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    c1 uuid;
    p1 uuid;
    veh1 uuid;
    srv1 uuid;
    srv2 uuid;
    wo1 uuid;
    wo2 uuid;
    woi1 uuid;
    woi2 uuid;
BEGIN
    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    p1 := uuid_generate_v5(v_org, 'pymes-seed/v1/product/1');
    veh1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/vehicle/1');
    srv1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/service/oil');
    srv2 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/service/brake');
    wo1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/1');
    wo2 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/2');
    woi1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/1');
    woi2 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/2');

    INSERT INTO workshops.vehicles (id, org_id, customer_id, customer_name, license_plate, vin, make, model, year, kilometers, color, notes)
    VALUES (
        veh1, v_org, c1, 'Cliente Demo Uno', 'AB 123 CD', '9BWZZZ377VT004251', 'Ford', 'Focus', 2018, 98500, 'Gris',
        'Vehículo semilla taller'
    )
    ON CONFLICT (id) DO NOTHING;
    SELECT id INTO veh1 FROM workshops.vehicles WHERE org_id = v_org AND license_plate = 'AB 123 CD' LIMIT 1;

    INSERT INTO workshops.services (id, org_id, segment, code, name, description, category, estimated_hours, base_price, currency, tax_rate, linked_product_id, is_active)
    VALUES
        (srv1, v_org, 'auto_repair', 'SRV-OIL', 'Cambio de aceite y filtro', 'Servicio estándar', 'mantenimiento', 0.5, 25000, 'ARS', 21, NULL, true),
        (srv2, v_org, 'auto_repair', 'SRV-BRAKE', 'Revisión de frenos', 'Inspección y ajuste', 'frenos', 1.5, 45000, 'ARS', 21, p1, true)
    ON CONFLICT (org_id, segment, code) DO NOTHING;
    SELECT id INTO srv1 FROM workshops.services WHERE org_id = v_org AND segment = 'auto_repair' AND code = 'SRV-OIL' LIMIT 1;
    SELECT id INTO srv2 FROM workshops.services WHERE org_id = v_org AND segment = 'auto_repair' AND code = 'SRV-BRAKE' LIMIT 1;

    IF veh1 IS NULL OR srv1 IS NULL OR srv2 IS NULL THEN
        RAISE EXCEPTION 'workshops seed: missing vehicle or services for org %', v_org;
    END IF;

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
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO wo1 FROM workshops.work_orders WHERE org_id = v_org AND number = 'OT-SEED-001' LIMIT 1;

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
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO wo2 FROM workshops.work_orders WHERE org_id = v_org AND number = 'OT-SEED-002' LIMIT 1;

    IF wo1 IS NULL OR wo2 IS NULL THEN
        RAISE EXCEPTION 'workshops seed: missing work orders for org %', v_org;
    END IF;

    INSERT INTO workshops.work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        (woi1, v_org, wo1, 'service', srv1, NULL, 'Cambio de aceite y filtro', 1, 25000, 21, 0, '{}'::jsonb),
        (woi2, v_org, wo1, 'part', NULL, p1, 'Producto Demo A (repuesto)', 1, 15000, 21, 1, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/3'), v_org, wo2, 'service', srv2, NULL, 'Revisión de frenos', 1, 45000, 21, 0, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;
END $$;
