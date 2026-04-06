-- Demo bicicletería: bicicletas, servicios, órdenes de trabajo.
-- Cliente/producto: mismas claves uuid v5 que pymes-core/seeds/02_core_business.sql.

DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    c1 uuid;
    c2 uuid;
    p1 uuid;
    svc1 uuid;
    bk1 uuid;
    bk2 uuid;
    bk3 uuid;
    srv1 uuid;
    srv2 uuid;
    srv3 uuid;
    srv4 uuid;
    wo1 uuid;
    wo2 uuid;
    wo3 uuid;
    woi1 uuid;
    woi2 uuid;
    woi3 uuid;
    woi4 uuid;
    woi5 uuid;
BEGIN
    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    p1 := uuid_generate_v5(v_org, 'pymes-seed/v1/product/1');
    bk1 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/bicycle/1');
    bk2 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/bicycle/2');
    bk3 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/bicycle/3');
    srv1 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/service/tune');
    srv2 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/service/brake');
    srv3 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/service/wheel');
    srv4 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/service/ebike');
    wo1 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/wo/1');
    wo2 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/wo/2');
    wo3 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/wo/3');
    woi1 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/woi/1');
    woi2 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/woi/2');
    woi3 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/woi/3');
    woi4 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/woi/4');
    woi5 := uuid_generate_v5(v_org, 'pymes-seed/v1/bike_shop/woi/5');

    -- Bicicletas
    INSERT INTO workshops.bicycles (id, org_id, customer_id, customer_name, frame_number, make, model, bike_type, size, wheel_size_inches, color, ebike_notes, notes)
    VALUES
        (bk1, v_org, c1, 'Cliente Demo Uno', 'SN-MTB-2024-001', 'Trek', 'Marlin 7', 'mtb', 'M', 29, 'Azul', '', 'Mountain bike uso recreativo'),
        (bk2, v_org, c1, 'Cliente Demo Uno', 'SN-ROAD-2023-042', 'Specialized', 'Allez Sport', 'road', 'L', 28, 'Negro/Rojo', '', 'Bici de ruta, uso diario'),
        (bk3, v_org, c2, 'Cliente Demo Dos', 'SN-EBIKE-2025-007', 'Giant', 'Explore E+ 2', 'ebike', 'L', 29, 'Verde', 'Motor Shimano EP8, batería 625Wh, 3200 km recorridos', 'E-bike trekking')
    ON CONFLICT (id) DO NOTHING;
    SELECT id INTO bk1 FROM workshops.bicycles WHERE org_id = v_org AND frame_number = 'SN-MTB-2024-001' LIMIT 1;
    SELECT id INTO bk2 FROM workshops.bicycles WHERE org_id = v_org AND frame_number = 'SN-ROAD-2023-042' LIMIT 1;
    SELECT id INTO bk3 FROM workshops.bicycles WHERE org_id = v_org AND frame_number = 'SN-EBIKE-2025-007' LIMIT 1;
    SELECT id INTO svc1 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;

    -- Servicios (segment = bike_shop)
    INSERT INTO workshops.services (id, org_id, segment, code, name, description, category, estimated_hours, base_price, currency, tax_rate, linked_service_id, is_active)
    VALUES
        (srv1, v_org, 'bike_shop', 'SRV-TUNE', 'Puesta a punto completa', 'Ajuste de cambios, frenos, dirección, lubricación de cadena y revisión general', 'mantenimiento', 2.0, 18000, 'ARS', 21, NULL, true),
        (srv2, v_org, 'bike_shop', 'SRV-BRAKE', 'Cambio de pastillas de freno', 'Reemplazo de pastillas y sangrado de sistema hidráulico', 'frenos', 1.0, 12000, 'ARS', 21, NULL, true),
        (srv3, v_org, 'bike_shop', 'SRV-WHEEL', 'Centrado y tensado de rueda', 'Centrado lateral y radial, verificación de tensión de rayos', 'ruedas', 1.0, 8000, 'ARS', 21, NULL, true),
        (srv4, v_org, 'bike_shop', 'SRV-EBIKE', 'Diagnóstico sistema eléctrico', 'Lectura de errores, verificación de batería, motor y display', 'electrico', 1.5, 15000, 'ARS', 21, svc1, true)
    ON CONFLICT (org_id, code, segment) WHERE archived_at IS NULL DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            category = EXCLUDED.category,
            estimated_hours = EXCLUDED.estimated_hours,
            base_price = EXCLUDED.base_price,
            currency = EXCLUDED.currency,
            tax_rate = EXCLUDED.tax_rate,
            linked_service_id = EXCLUDED.linked_service_id,
            is_active = EXCLUDED.is_active,
            updated_at = now();
    SELECT id INTO srv1 FROM workshops.services WHERE org_id = v_org AND segment = 'bike_shop' AND code = 'SRV-TUNE' LIMIT 1;
    SELECT id INTO srv2 FROM workshops.services WHERE org_id = v_org AND segment = 'bike_shop' AND code = 'SRV-BRAKE' LIMIT 1;
    SELECT id INTO srv3 FROM workshops.services WHERE org_id = v_org AND segment = 'bike_shop' AND code = 'SRV-WHEEL' LIMIT 1;
    SELECT id INTO srv4 FROM workshops.services WHERE org_id = v_org AND segment = 'bike_shop' AND code = 'SRV-EBIKE' LIMIT 1;

    IF bk1 IS NULL OR bk2 IS NULL OR bk3 IS NULL OR srv1 IS NULL THEN
        RAISE EXCEPTION 'bike_shop seed: missing bicycles or services for org %', v_org;
    END IF;

    -- Órdenes de trabajo
    INSERT INTO workshops.bike_work_orders (
        id, org_id, number, bicycle_id, bicycle_label, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency,
        subtotal_services, subtotal_parts, tax_total, total, created_by
    )
    VALUES (
        wo1, v_org, 'BK-SEED-001', bk1, 'Trek Marlin 7 (SN-MTB-2024-001)', c1, 'Cliente Demo Uno', 'received',
        'Puesta a punto completa, rueda trasera descentrada', '', 'Ingresó con ruido en cambios', '', 'ARS',
        26000, 0, 5460, 31460, 'seed'
    )
    ON CONFLICT (org_id, number) WHERE archived_at IS NULL DO NOTHING;
    SELECT id INTO wo1 FROM workshops.bike_work_orders WHERE org_id = v_org AND number = 'BK-SEED-001' LIMIT 1;

    INSERT INTO workshops.bike_work_orders (
        id, org_id, number, bicycle_id, bicycle_label, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency,
        subtotal_services, subtotal_parts, tax_total, total, created_by
    )
    VALUES (
        wo2, v_org, 'BK-SEED-002', bk2, 'Specialized Allez Sport (SN-ROAD-2023-042)', c1, 'Cliente Demo Uno', 'in_progress',
        'Frenos blandos, no frenan bien', 'Pastillas gastadas, líquido contaminado', 'En taller', 'Pedir pastillas Shimano L03A', 'ARS',
        12000, 8500, 4305, 24805, 'seed'
    )
    ON CONFLICT (org_id, number) WHERE archived_at IS NULL DO NOTHING;
    SELECT id INTO wo2 FROM workshops.bike_work_orders WHERE org_id = v_org AND number = 'BK-SEED-002' LIMIT 1;

    INSERT INTO workshops.bike_work_orders (
        id, org_id, number, bicycle_id, bicycle_label, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency,
        subtotal_services, subtotal_parts, tax_total, total, created_by
    )
    VALUES (
        wo3, v_org, 'BK-SEED-003', bk3, 'Giant Explore E+ 2 (SN-EBIKE-2025-007)', c2, 'Cliente Demo Dos', 'diagnosing',
        'Error E010 en display, pérdida de potencia', '', 'E-bike, manipular con cuidado', 'Verificar conector motor-controladora', 'ARS',
        15000, 0, 3150, 18150, 'seed'
    )
    ON CONFLICT (org_id, number) WHERE archived_at IS NULL DO NOTHING;
    SELECT id INTO wo3 FROM workshops.bike_work_orders WHERE org_id = v_org AND number = 'BK-SEED-003' LIMIT 1;

    IF wo1 IS NULL OR wo2 IS NULL OR wo3 IS NULL THEN
        RAISE EXCEPTION 'bike_shop seed: missing work orders for org %', v_org;
    END IF;

    -- Items de órdenes
    INSERT INTO workshops.bike_work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        (woi1, v_org, wo1, 'service', srv1, NULL, 'Puesta a punto completa', 1, 18000, 21, 0, '{}'::jsonb),
        (woi2, v_org, wo1, 'service', srv3, NULL, 'Centrado y tensado de rueda', 1, 8000, 21, 1, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.bike_work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        (woi3, v_org, wo2, 'service', srv2, NULL, 'Cambio de pastillas de freno', 1, 12000, 21, 0, '{}'::jsonb),
        (woi4, v_org, wo2, 'part', NULL, p1, 'Pastillas Shimano L03A (repuesto)', 1, 8500, 21, 1, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO workshops.bike_work_order_items (id, org_id, work_order_id, item_type, service_id, product_id, description, quantity, unit_price, tax_rate, sort_order, metadata)
    VALUES
        (woi5, v_org, wo3, 'service', srv4, NULL, 'Diagnóstico sistema eléctrico', 1, 15000, 21, 0, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;
END $$;
