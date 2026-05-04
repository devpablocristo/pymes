-- Clientes, proveedores, productos, cotización, ventas, stock, caja (demo).
-- IDs determinísticos por org (uuid v5) para poder sembrar varias orgs sin colisión de PK global.
-- Requiere extensión uuid-ossp (migración base).

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    c1 uuid;
    c2 uuid;
    c3 uuid;
    s1 uuid;
    s2 uuid;
    s3 uuid;
    p1 uuid;
    p2 uuid;
    p3 uuid;
    svc1 uuid;
    svc2 uuid;
    svc3 uuid;
    q1 uuid;
    q2 uuid;
    q3 uuid;
    sale1 uuid;
    sale2 uuid;
    sale3 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    c3 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/3');
    s1 := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/1');
    s2 := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/2');
    s3 := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/3');
    q1 := uuid_generate_v5(v_org, 'pymes-seed/v1/quote/1');
    q2 := uuid_generate_v5(v_org, 'pymes-seed/v1/quote/2');
    q3 := uuid_generate_v5(v_org, 'pymes-seed/v1/quote/3');
    sale1 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/1');
    sale2 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/2');
    sale3 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/3');

    INSERT INTO parties (id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata, created_at, updated_at, deleted_at)
    VALUES
        (c1, v_org, 'person', 'Cliente Demo Uno', 'cliente1@local.dev', '+54-11-1000-0001', '{}'::jsonb, NULL, 'seed', ARRAY['demo'], '{}'::jsonb, now(), now(), NULL),
        (c2, v_org, 'organization', 'Cliente Demo Dos', 'compras@demo2.local', '+54-11-1000-0002', '{}'::jsonb, '20111222333', 'seed', ARRAY['demo', 'vip'], '{}'::jsonb, now(), now(), NULL),
        (c3, v_org, 'person', 'Cliente Demo Tres', NULL, '+54-11-1000-0003', '{}'::jsonb, NULL, 'seed', ARRAY['demo'], '{}'::jsonb, now(), now(), NULL),
        (s1, v_org, 'organization', 'Proveedor Demo 1', 'ventas@prov1.local', '+54-11-2000-0001', '{}'::jsonb, '30700111223', 'seed', ARRAY['demo'], jsonb_build_object('contact_name', 'Lucia'), now(), now(), NULL),
        (s2, v_org, 'organization', 'Proveedor Demo 2', 'ventas@prov2.local', '+54-11-2000-0002', '{}'::jsonb, NULL, 'seed', ARRAY['demo'], jsonb_build_object('contact_name', 'Martin'), now(), now(), NULL),
        (s3, v_org, 'organization', 'Proveedor Demo 3', 'ventas@prov3.local', '+54-11-2000-0003', '{}'::jsonb, '27933111222', 'seed', ARRAY['demo'], jsonb_build_object('contact_name', 'Paula'), now(), now(), NULL)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO party_persons (party_id, first_name, last_name)
    VALUES
        (c1, 'Cliente', 'Demo Uno'),
        (c3, 'Cliente', 'Demo Tres')
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
    VALUES
        (c2, 'Cliente Demo Dos', 'Cliente Demo Dos', ''),
        (s1, 'Proveedor Demo 1', 'Proveedor Demo 1', ''),
        (s2, 'Proveedor Demo 2', 'Proveedor Demo 2', ''),
        (s3, 'Proveedor Demo 3', 'Proveedor Demo 3', '')
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    VALUES
        (gen_random_uuid(), c1, v_org, 'customer', true, NULL::uuid, '{}'::jsonb, now()),
        (gen_random_uuid(), c2, v_org, 'customer', true, NULL::uuid, '{}'::jsonb, now()),
        (gen_random_uuid(), c3, v_org, 'customer', true, NULL::uuid, '{}'::jsonb, now()),
        (gen_random_uuid(), s1, v_org, 'supplier', true, NULL::uuid, jsonb_build_object('contact_name', 'Lucia'), now()),
        (gen_random_uuid(), s2, v_org, 'supplier', true, NULL::uuid, jsonb_build_object('contact_name', 'Martin'), now()),
        (gen_random_uuid(), s3, v_org, 'supplier', true, NULL::uuid, jsonb_build_object('contact_name', 'Paula'), now())
    ON CONFLICT (party_id, org_id, role) DO UPDATE SET
        is_active = EXCLUDED.is_active,
        price_list_id = EXCLUDED.price_list_id,
        metadata = EXCLUDED.metadata;

    -- Índice único (org_id, sku) es parcial → no sirve para ON CONFLICT; upsert manual.
    SELECT id INTO p1 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-001' AND deleted_at IS NULL LIMIT 1;
    IF p1 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, price_currency, cost_price, tax_rate, track_stock, is_active, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/1'), v_org, 'product', 'DEMO-PROD-001', 'Producto Demo A', 'Producto físico A', 'unit', 15000, 'ARS', 9000, 21, true, true, ARRAY['demo']);
        SELECT id INTO p1 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-001' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO p2 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-002' AND deleted_at IS NULL LIMIT 1;
    IF p2 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, price_currency, cost_price, tax_rate, track_stock, is_active, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/2'), v_org, 'product', 'DEMO-PROD-002', 'Producto Demo B', 'Producto físico B', 'unit', 9500, 'ARS', 6000, 21, true, true, ARRAY['demo']);
        SELECT id INTO p2 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-002' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO p3 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-003' AND deleted_at IS NULL LIMIT 1;
    IF p3 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, price_currency, cost_price, tax_rate, track_stock, is_active, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/3'), v_org, 'product', 'DEMO-PROD-003', 'Producto Demo C', 'Producto físico C', 'unit', 7300, 'ARS', 4200, 21, true, true, ARRAY['demo']);
        SELECT id INTO p3 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-003' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO svc1 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    IF svc1 IS NULL THEN
        INSERT INTO services (id, org_id, code, name, description, category_code, sale_price, cost_price, tax_rate, currency, default_duration_minutes, is_active, tags, metadata)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/4'), v_org, 'DEMO-SVC-001', 'Servicio Demo Instalación', 'Servicio de instalación', 'general', 25000, 12000, 21, 'ARS', 60, true, ARRAY['demo'], '{}'::jsonb);
        SELECT id INTO svc1 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO svc2 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    IF svc2 IS NULL THEN
        INSERT INTO services (id, org_id, code, name, description, category_code, sale_price, cost_price, tax_rate, currency, default_duration_minutes, is_active, tags, metadata)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/5'), v_org, 'DEMO-SVC-002', 'Servicio Demo Mantenimiento', 'Servicio de mantenimiento', 'general', 12000, 7000, 21, 'ARS', 45, true, ARRAY['demo'], '{}'::jsonb);
        SELECT id INTO svc2 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO svc3 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-003' AND deleted_at IS NULL LIMIT 1;
    IF svc3 IS NULL THEN
        INSERT INTO services (id, org_id, code, name, description, category_code, sale_price, cost_price, tax_rate, currency, default_duration_minutes, is_active, tags, metadata)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/6'), v_org, 'DEMO-SVC-003', 'Servicio Demo Express', 'Servicio rápido demo', 'general', 8500, 4000, 21, 'ARS', 30, true, ARRAY['demo'], '{}'::jsonb);
        SELECT id INTO svc3 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-003' AND deleted_at IS NULL LIMIT 1;
    END IF;

    IF p1 IS NULL OR p2 IS NULL OR p3 IS NULL OR svc1 IS NULL OR svc2 IS NULL OR svc3 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing catalog ids after upsert for org %', v_org;
    END IF;

    UPDATE products
       SET deleted_at = COALESCE(deleted_at, now()),
           updated_at = now()
     WHERE org_id = v_org
       AND type = 'service'
       AND sku IN ('DEMO-SVC-001', 'DEMO-SVC-002', 'DEMO-SVC-003');

    -- El índice único es parcial (WHERE branch_id IS NULL), por eso ON CONFLICT
    -- necesita el mismo predicado. Sembramos el nivel legacy (sin branch).
    INSERT INTO stock_levels (org_id, product_id, quantity, min_quantity)
    VALUES
        (v_org, p1, 50, 10),
        (v_org, p2, 30, 8),
        (v_org, p3, 20, 5)
    ON CONFLICT (org_id, product_id) WHERE branch_id IS NULL DO UPDATE
        SET quantity = EXCLUDED.quantity,
            min_quantity = EXCLUDED.min_quantity,
            updated_at = now();

    INSERT INTO quotes (id, org_id, number, party_id, party_name, status, subtotal, tax_total, total, currency, notes, created_by)
    VALUES (q1, v_org, 'PRE-00001', c1, 'Cliente Demo Uno', 'accepted', 40000, 8400, 48400, 'ARS', 'seed quote', 'seed')
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO q1 FROM quotes WHERE org_id = v_org AND number = 'PRE-00001' LIMIT 1;

    IF q1 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing quote PRE-00001 for org %', v_org;
    END IF;

    INSERT INTO quotes (id, org_id, number, party_id, party_name, status, subtotal, tax_total, total, currency, notes, created_by)
    VALUES
        (q2, v_org, 'PRE-00002', c2, 'Cliente Demo Dos', 'draft', 19000, 3990, 22990, 'ARS', 'seed quote 2', 'seed'),
        (q3, v_org, 'PRE-00003', c3, 'Cliente Demo Tres', 'sent', 8500, 1785, 10285, 'ARS', 'seed quote 3', 'seed')
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO q2 FROM quotes WHERE org_id = v_org AND number = 'PRE-00002' LIMIT 1;
    SELECT id INTO q3 FROM quotes WHERE org_id = v_org AND number = 'PRE-00003' LIMIT 1;

    IF q2 IS NULL OR q3 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing quote PRE-00002/PRE-00003 for org %', v_org;
    END IF;

    INSERT INTO quote_items (id, quote_id, product_id, service_id, description, quantity, unit_price, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/quote-item/1'), q1, p1, NULL, 'Producto Demo A', 1, 15000, 21, 15000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/quote-item/2'), q1, NULL, svc1, 'Servicio Demo Instalación', 1, 25000, 21, 25000, 2)
    ON CONFLICT (id) DO UPDATE
        SET product_id = EXCLUDED.product_id,
            service_id = EXCLUDED.service_id,
            description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_price = EXCLUDED.unit_price,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal,
            sort_order = EXCLUDED.sort_order;

    INSERT INTO quote_items (id, quote_id, product_id, service_id, description, quantity, unit_price, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/quote-item/q2/1'), q2, p2, NULL, 'Producto Demo B', 2, 9500, 21, 19000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/quote-item/q3/1'), q3, NULL, svc3, 'Servicio Demo Express', 1, 8500, 21, 8500, 1)
    ON CONFLICT (id) DO UPDATE
        SET product_id = EXCLUDED.product_id,
            service_id = EXCLUDED.service_id,
            description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_price = EXCLUDED.unit_price,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal,
            sort_order = EXCLUDED.sort_order;

    INSERT INTO sales (id, org_id, number, party_id, party_name, quote_id, status, payment_method, subtotal, tax_total, total, currency, notes, created_by)
    VALUES
        (sale1, v_org, 'VTA-00001', c1, 'Cliente Demo Uno', q1, 'completed', 'transfer', 40000, 8400, 48400, 'ARS', 'seed sale 1', 'seed'),
        (sale2, v_org, 'VTA-00002', c2, 'Cliente Demo Dos', NULL, 'completed', 'cash', 9500, 1995, 11495, 'ARS', 'seed sale 2', 'seed'),
        (sale3, v_org, 'VTA-00003', c3, 'Cliente Demo Tres', NULL, 'completed', 'card', 7300, 1533, 8833, 'ARS', 'seed sale 3', 'seed')
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO sale1 FROM sales WHERE org_id = v_org AND number = 'VTA-00001' LIMIT 1;
    SELECT id INTO sale2 FROM sales WHERE org_id = v_org AND number = 'VTA-00002' LIMIT 1;
    SELECT id INTO sale3 FROM sales WHERE org_id = v_org AND number = 'VTA-00003' LIMIT 1;

    IF sale1 IS NULL OR sale2 IS NULL OR sale3 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing sale rows for org %', v_org;
    END IF;

    INSERT INTO sale_items (id, sale_id, product_id, service_id, description, quantity, unit_price, cost_price, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/1'), sale1, p1, NULL, 'Producto Demo A', 1, 15000, 9000, 21, 15000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/2'), sale1, NULL, svc1, 'Servicio Demo Instalación', 1, 25000, 12000, 21, 25000, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/3'), sale2, p2, NULL, 'Producto Demo B', 1, 9500, 6000, 21, 9500, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/4'), sale3, p3, NULL, 'Producto Demo C', 1, 7300, 4200, 21, 7300, 1)
    ON CONFLICT (id) DO UPDATE
        SET product_id = EXCLUDED.product_id,
            service_id = EXCLUDED.service_id,
            description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_price = EXCLUDED.unit_price,
            cost_price = EXCLUDED.cost_price,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal,
            sort_order = EXCLUDED.sort_order;

    INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/1'), v_org, p1, 'out', -1, 'sale', sale1, 'Seed stock movement', 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/2'), v_org, p2, 'out', -1, 'sale', sale2, 'Seed stock movement', 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/3'), v_org, p3, 'out', -1, 'sale', sale3, 'Seed stock movement', 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO cash_movements (id, org_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/1'), v_org, 'income', 48400, 'ARS', 'sale', 'Seed sale income', 'transfer', 'sale', sale1, 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/2'), v_org, 'income', 11495, 'ARS', 'sale', 'Seed sale income', 'cash', 'sale', sale2, 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/3'), v_org, 'income', 8833, 'ARS', 'sale', 'Seed sale income', 'card', 'sale', sale3, 'seed')
    ON CONFLICT (id) DO NOTHING;

    UPDATE tenant_settings
       SET currency = 'ARS',
           tax_rate = 21.00,
           quote_prefix = 'PRE',
           sale_prefix = 'VTA',
           next_quote_number = GREATEST(next_quote_number, 4),
           next_sale_number = GREATEST(next_sale_number, 4),
           allow_negative_stock = true,
           updated_at = now()
     WHERE org_id = v_org;
END $$;
