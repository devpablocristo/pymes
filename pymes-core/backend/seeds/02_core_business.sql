-- Clientes, proveedores, productos, cotización, ventas, stock, caja (demo).
-- IDs determinísticos por org (uuid v5) para poder sembrar varias orgs sin colisión de PK global.
-- Requiere extensión uuid-ossp (migración base).

DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    c1 uuid;
    c2 uuid;
    c3 uuid;
    s1 uuid;
    s2 uuid;
    p1 uuid;
    p2 uuid;
    p3 uuid;
    p4 uuid;
    p5 uuid;
    q1 uuid;
    sale1 uuid;
    sale2 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    c3 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/3');
    s1 := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/1');
    s2 := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/2');
    q1 := uuid_generate_v5(v_org, 'pymes-seed/v1/quote/1');
    sale1 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/1');
    sale2 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/2');

    INSERT INTO customers (id, org_id, type, name, tax_id, email, phone, notes, tags)
    VALUES
        (c1, v_org, 'person', 'Cliente Demo Uno', NULL, 'cliente1@local.dev', '+54-11-1000-0001', 'seed', ARRAY['demo']),
        (c2, v_org, 'company', 'Cliente Demo Dos', '20111222333', 'compras@demo2.local', '+54-11-1000-0002', 'seed', ARRAY['demo', 'vip']),
        (c3, v_org, 'person', 'Cliente Demo Tres', NULL, NULL, '+54-11-1000-0003', 'seed', ARRAY['demo'])
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO suppliers (id, org_id, name, tax_id, email, phone, contact_name, notes, tags)
    VALUES
        (s1, v_org, 'Proveedor Demo 1', '30700111223', 'ventas@prov1.local', '+54-11-2000-0001', 'Lucia', 'seed', ARRAY['demo']),
        (s2, v_org, 'Proveedor Demo 2', NULL, 'ventas@prov2.local', '+54-11-2000-0002', 'Martin', 'seed', ARRAY['demo'])
    ON CONFLICT (id) DO NOTHING;

    -- Party model: la API de clientes lista desde parties + party_roles; espejo de customers/suppliers.
    INSERT INTO parties (id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata, created_at, updated_at, deleted_at)
    SELECT
        c.id,
        c.org_id,
        CASE WHEN c.type = 'company' THEN 'organization' ELSE 'person' END,
        c.name,
        NULLIF(TRIM(c.email), ''),
        NULLIF(TRIM(c.phone), ''),
        COALESCE(c.address, '{}'::jsonb),
        NULLIF(TRIM(c.tax_id), ''),
        COALESCE(c.notes, ''),
        COALESCE(c.tags, '{}'::text[]),
        COALESCE(c.metadata, '{}'::jsonb),
        c.created_at,
        c.updated_at,
        c.deleted_at
    FROM customers c
    WHERE c.org_id = v_org AND c.id IN (c1, c2, c3)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO party_persons (party_id, first_name, last_name)
    SELECT
        c.id,
        split_part(TRIM(c.name), ' ', 1),
        NULLIF(TRIM(regexp_replace(TRIM(c.name), '^[^ ]+\s*', '')), '')
    FROM customers c
    WHERE c.org_id = v_org AND c.id IN (c1, c2, c3) AND c.type = 'person'
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
    SELECT
        c.id,
        c.name,
        c.name,
        COALESCE(c.metadata->>'tax_condition', '')
    FROM customers c
    WHERE c.org_id = v_org AND c.id IN (c1, c2, c3) AND c.type = 'company'
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    SELECT gen_random_uuid(), c.id, c.org_id, 'customer', true, c.price_list_id, '{}'::jsonb, c.created_at
    FROM customers c
    WHERE c.org_id = v_org AND c.id IN (c1, c2, c3)
    ON CONFLICT (party_id, org_id, role) DO NOTHING;

    INSERT INTO parties (id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata, created_at, updated_at, deleted_at)
    SELECT
        s.id,
        s.org_id,
        'organization',
        s.name,
        NULLIF(TRIM(s.email), ''),
        NULLIF(TRIM(s.phone), ''),
        COALESCE(s.address, '{}'::jsonb),
        NULLIF(TRIM(s.tax_id), ''),
        COALESCE(s.notes, ''),
        COALESCE(s.tags, '{}'::text[]),
        COALESCE(s.metadata, '{}'::jsonb)
            || CASE
                WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb
                ELSE jsonb_build_object('contact_name', s.contact_name)
            END,
        s.created_at,
        s.updated_at,
        s.deleted_at
    FROM suppliers s
    WHERE s.org_id = v_org AND s.id IN (s1, s2)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
    SELECT
        s.id,
        s.name,
        s.name,
        COALESCE(s.metadata->>'tax_condition', '')
    FROM suppliers s
    WHERE s.org_id = v_org AND s.id IN (s1, s2)
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    SELECT
        gen_random_uuid(),
        s.id,
        s.org_id,
        'supplier',
        true,
        NULL::uuid,
        CASE
            WHEN COALESCE(NULLIF(TRIM(s.contact_name), ''), '') = '' THEN '{}'::jsonb
            ELSE jsonb_build_object('contact_name', s.contact_name)
        END,
        s.created_at
    FROM suppliers s
    WHERE s.org_id = v_org AND s.id IN (s1, s2)
    ON CONFLICT (party_id, org_id, role) DO NOTHING;

    -- Índice único (org_id, sku) es parcial → no sirve para ON CONFLICT; upsert manual.
    SELECT id INTO p1 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-001' AND deleted_at IS NULL LIMIT 1;
    IF p1 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/1'), v_org, 'product', 'DEMO-PROD-001', 'Producto Demo A', 'Producto físico A', 'unit', 15000, 9000, 21, true, ARRAY['demo']);
        SELECT id INTO p1 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-001' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO p2 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-002' AND deleted_at IS NULL LIMIT 1;
    IF p2 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/2'), v_org, 'product', 'DEMO-PROD-002', 'Producto Demo B', 'Producto físico B', 'unit', 9500, 6000, 21, true, ARRAY['demo']);
        SELECT id INTO p2 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-002' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO p3 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-003' AND deleted_at IS NULL LIMIT 1;
    IF p3 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/3'), v_org, 'product', 'DEMO-PROD-003', 'Producto Demo C', 'Producto físico C', 'unit', 7300, 4200, 21, true, ARRAY['demo']);
        SELECT id INTO p3 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-003' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO p4 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    IF p4 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/4'), v_org, 'service', 'DEMO-SVC-001', 'Servicio Demo Instalación', 'Servicio de instalación', 'hr', 25000, 12000, 21, false, ARRAY['demo']);
        SELECT id INTO p4 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    END IF;

    SELECT id INTO p5 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    IF p5 IS NULL THEN
        INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags)
        VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/product/5'), v_org, 'service', 'DEMO-SVC-002', 'Servicio Demo Mantenimiento', 'Servicio de mantenimiento', 'hr', 12000, 7000, 21, false, ARRAY['demo']);
        SELECT id INTO p5 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    END IF;

    IF p1 IS NULL OR p2 IS NULL OR p3 IS NULL OR p4 IS NULL OR p5 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing product ids after upsert for org %', v_org;
    END IF;

    INSERT INTO stock_levels (org_id, product_id, quantity, min_quantity)
    VALUES
        (v_org, p1, 50, 10),
        (v_org, p2, 30, 8),
        (v_org, p3, 20, 5)
    ON CONFLICT (org_id, product_id) DO UPDATE
        SET quantity = EXCLUDED.quantity,
            min_quantity = EXCLUDED.min_quantity,
            updated_at = now();

    INSERT INTO quotes (id, org_id, number, customer_id, customer_name, status, subtotal, tax_total, total, currency, notes, created_by)
    VALUES (q1, v_org, 'PRE-00001', c1, 'Cliente Demo Uno', 'accepted', 40000, 8400, 48400, 'ARS', 'seed quote', 'seed')
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO q1 FROM quotes WHERE org_id = v_org AND number = 'PRE-00001' LIMIT 1;

    IF q1 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing quote PRE-00001 for org %', v_org;
    END IF;

    INSERT INTO quote_items (id, quote_id, product_id, description, quantity, unit_price, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/quote-item/1'), q1, p1, 'Producto Demo A', 1, 15000, 21, 15000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/quote-item/2'), q1, p4, 'Servicio Demo Instalación', 1, 25000, 21, 25000, 2)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO sales (id, org_id, number, customer_id, customer_name, quote_id, status, payment_method, subtotal, tax_total, total, currency, notes, created_by)
    VALUES
        (sale1, v_org, 'VTA-00001', c1, 'Cliente Demo Uno', q1, 'completed', 'transfer', 40000, 8400, 48400, 'ARS', 'seed sale 1', 'seed'),
        (sale2, v_org, 'VTA-00002', c2, 'Cliente Demo Dos', NULL, 'completed', 'cash', 9500, 1995, 11495, 'ARS', 'seed sale 2', 'seed')
    ON CONFLICT (org_id, number) DO NOTHING;
    SELECT id INTO sale1 FROM sales WHERE org_id = v_org AND number = 'VTA-00001' LIMIT 1;
    SELECT id INTO sale2 FROM sales WHERE org_id = v_org AND number = 'VTA-00002' LIMIT 1;

    IF sale1 IS NULL OR sale2 IS NULL THEN
        RAISE EXCEPTION 'pymes seed: missing sale rows for org %', v_org;
    END IF;

    INSERT INTO sale_items (id, sale_id, product_id, description, quantity, unit_price, cost_price, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/1'), sale1, p1, 'Producto Demo A', 1, 15000, 9000, 21, 15000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/2'), sale1, p4, 'Servicio Demo Instalación', 1, 25000, 12000, 21, 25000, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/3'), sale2, p2, 'Producto Demo B', 1, 9500, 6000, 21, 9500, 1)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/1'), v_org, p1, 'out', -1, 'sale', sale1, 'Seed stock movement', 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/2'), v_org, p2, 'out', -1, 'sale', sale2, 'Seed stock movement', 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO cash_movements (id, org_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/1'), v_org, 'income', 48400, 'ARS', 'sale', 'Seed sale income', 'transfer', 'sale', sale1, 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/2'), v_org, 'income', 11495, 'ARS', 'sale', 'Seed sale income', 'cash', 'sale', sale2, 'seed')
    ON CONFLICT (id) DO NOTHING;

    UPDATE tenant_settings
       SET currency = 'ARS',
           tax_rate = 21.00,
           quote_prefix = 'PRE',
           sale_prefix = 'VTA',
           next_quote_number = GREATEST(next_quote_number, 2),
           next_sale_number = GREATEST(next_sale_number, 3),
           allow_negative_stock = true,
           updated_at = now()
     WHERE org_id = v_org;
END $$;
