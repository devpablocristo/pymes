DO $$
DECLARE
    v_org uuid := '00000000-0000-0000-0000-000000000001';
    c1 uuid := '10000000-0000-0000-0000-000000000001';
    c2 uuid := '10000000-0000-0000-0000-000000000002';
    c3 uuid := '10000000-0000-0000-0000-000000000003';
    s1 uuid := '11000000-0000-0000-0000-000000000001';
    s2 uuid := '11000000-0000-0000-0000-000000000002';
    p1 uuid := '12000000-0000-0000-0000-000000000001';
    p2 uuid := '12000000-0000-0000-0000-000000000002';
    p3 uuid := '12000000-0000-0000-0000-000000000003';
    p4 uuid := '12000000-0000-0000-0000-000000000004';
    p5 uuid := '12000000-0000-0000-0000-000000000005';
    q1 uuid := '13000000-0000-0000-0000-000000000001';
    sale1 uuid := '14000000-0000-0000-0000-000000000001';
    sale2 uuid := '14000000-0000-0000-0000-000000000002';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

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

    INSERT INTO products (id, org_id, type, sku, name, description, unit, price, cost_price, tax_rate, track_stock, tags)
    VALUES
        (p1, v_org, 'product', 'DEMO-PROD-001', 'Producto Demo A', 'Producto físico A', 'unit', 15000, 9000, 21, true, ARRAY['demo']),
        (p2, v_org, 'product', 'DEMO-PROD-002', 'Producto Demo B', 'Producto físico B', 'unit', 9500, 6000, 21, true, ARRAY['demo']),
        (p3, v_org, 'product', 'DEMO-PROD-003', 'Producto Demo C', 'Producto físico C', 'unit', 7300, 4200, 21, true, ARRAY['demo']),
        (p4, v_org, 'service', 'DEMO-SVC-001', 'Servicio Demo Instalación', 'Servicio de instalación', 'hr', 25000, 12000, 21, false, ARRAY['demo']),
        (p5, v_org, 'service', 'DEMO-SVC-002', 'Servicio Demo Mantenimiento', 'Servicio de mantenimiento', 'hr', 12000, 7000, 21, false, ARRAY['demo'])
    ON CONFLICT (id) DO NOTHING;

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
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO quote_items (id, quote_id, product_id, description, quantity, unit_price, tax_rate, subtotal, sort_order)
    VALUES
        ('13000000-0000-0000-0000-000000000011', q1, p1, 'Producto Demo A', 1, 15000, 21, 15000, 1),
        ('13000000-0000-0000-0000-000000000012', q1, p4, 'Servicio Demo Instalación', 1, 25000, 21, 25000, 2)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO sales (id, org_id, number, customer_id, customer_name, quote_id, status, payment_method, subtotal, tax_total, total, currency, notes, created_by)
    VALUES
        (sale1, v_org, 'VTA-00001', c1, 'Cliente Demo Uno', q1, 'completed', 'transfer', 40000, 8400, 48400, 'ARS', 'seed sale 1', 'seed'),
        (sale2, v_org, 'VTA-00002', c2, 'Cliente Demo Dos', NULL, 'completed', 'cash', 9500, 1995, 11495, 'ARS', 'seed sale 2', 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO sale_items (id, sale_id, product_id, description, quantity, unit_price, cost_price, tax_rate, subtotal, sort_order)
    VALUES
        ('14000000-0000-0000-0000-000000000011', sale1, p1, 'Producto Demo A', 1, 15000, 9000, 21, 15000, 1),
        ('14000000-0000-0000-0000-000000000012', sale1, p4, 'Servicio Demo Instalación', 1, 25000, 12000, 21, 25000, 2),
        ('14000000-0000-0000-0000-000000000013', sale2, p2, 'Producto Demo B', 1, 9500, 6000, 21, 9500, 1)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by)
    VALUES
        ('15000000-0000-0000-0000-000000000001', v_org, p1, 'out', -1, 'sale', sale1, 'Seed stock movement', 'seed'),
        ('15000000-0000-0000-0000-000000000002', v_org, p2, 'out', -1, 'sale', sale2, 'Seed stock movement', 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO cash_movements (id, org_id, type, amount, currency, category, description, payment_method, reference_type, reference_id, created_by)
    VALUES
        ('16000000-0000-0000-0000-000000000001', v_org, 'income', 48400, 'ARS', 'sale', 'Seed sale income', 'transfer', 'sale', sale1, 'seed'),
        ('16000000-0000-0000-0000-000000000002', v_org, 'income', 11495, 'ARS', 'sale', 'Seed sale income', 'cash', 'sale', sale2, 'seed')
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
