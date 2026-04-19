-- Demo taller bike_shop: una OT con target_type='bicycle'.
-- target_id es opaco (cada vertical valida). Usamos uuid_generate_v5 determinístico.
-- Depende de 02_core_business (cliente c1).

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    c1 uuid;
    bike1 uuid;
    wo_bike uuid;
    woi_bike1 uuid;
    woi_bike2 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    bike1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/bicycle/1');
    wo_bike := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/bike/1');
    woi_bike1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/bike/1');
    woi_bike2 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/bike/2');

    INSERT INTO workshops.bicycles (
        id, org_id, customer_id, customer_name, frame_number, brand, model, bike_type, size,
        wheel_size_inches, color, notes
    )
    VALUES (
        bike1, v_org, c1, 'Cliente Demo Uno', 'BIKE-DEMO-001', 'Trek', 'Marlin 7', 'mountain', 'M',
        29, 'Negro', 'Bicicleta demo para bike_shop'
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_orders (
        id, org_id, number, target_type, target_id, target_label, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency, metadata,
        subtotal_services, subtotal_parts, tax_total, total, created_by
    )
    VALUES (
        wo_bike, v_org, 'OT-BIKE-001', 'bicycle', bike1, 'Trek Marlin 7', c1, 'Cliente Demo Uno', 'in_progress',
        'Servicio general + cambio de cámara', 'Cámara trasera pinchada, cadena desgastada', 'Bicicleta seed', '', 'ARS',
        jsonb_build_object('vertical', 'workshops', 'segment', 'bike_shop'),
        18000, 12000, 6300, 36300, 'seed'
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.work_order_items (
        id, org_id, work_order_id, item_type, service_id, product_id,
        description, quantity, unit_price, tax_rate, sort_order, metadata
    )
    VALUES
        (woi_bike1, v_org, wo_bike, 'service', NULL, NULL, 'Servicio general bicicleta', 1, 18000, 21, 1, '{}'::jsonb),
        (woi_bike2, v_org, wo_bike, 'part', NULL, NULL, 'Cámara 29x2.1', 1, 12000, 21, 2, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;
END $$;
