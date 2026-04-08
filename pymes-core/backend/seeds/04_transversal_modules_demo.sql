-- Demo transversal: citas, recurrentes, compras, procurement, webhooks, cuentas.
-- Mismas claves uuid v5 que 02_core_business y 03_rbac.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    pl_default uuid;
    c1 uuid;
    c2 uuid;
    s1 uuid;
    p1 uuid;
    p2 uuid;
    p3 uuid;
    ap1 uuid;
    ap2 uuid;
    rec1 uuid;
    pur1 uuid;
    pur2 uuid;
    pr1 uuid;
    wh1 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    s1 := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/1');
    p1 := uuid_generate_v5(v_org, 'pymes-seed/v1/product/1');
    p2 := uuid_generate_v5(v_org, 'pymes-seed/v1/product/2');
    p3 := uuid_generate_v5(v_org, 'pymes-seed/v1/product/3');
    pl_default := uuid_generate_v5(v_org, 'pymes-seed/v1/price-list/default');
    rec1 := uuid_generate_v5(v_org, 'pymes-seed/v1/recurring/1');
    pur1 := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/1');
    pur2 := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/2');
    pr1 := uuid_generate_v5(v_org, 'pymes-seed/v1/procurement/1');
    wh1 := uuid_generate_v5(v_org, 'pymes-seed/v1/webhook/1');

    IF NOT EXISTS (
        SELECT 1
        FROM party_roles
        WHERE party_id = c1 AND org_id = v_org AND role = 'customer' AND is_active = true
    ) THEN
        RETURN;
    END IF;

    INSERT INTO price_list_items (price_list_id, product_id, price)
    VALUES
        (pl_default, p1, 14500.00),
        (pl_default, p2, 9200.00),
        (pl_default, p3, 7000.00)
    ON CONFLICT (price_list_id, product_id) DO UPDATE
        SET price = EXCLUDED.price;

    INSERT INTO recurring_expenses (id, org_id, description, amount, currency, category, payment_method, frequency, day_of_month, party_id, is_active, next_due_date, notes, created_by)
    VALUES
        (rec1, v_org, 'Alquiler local (demo)', 350000.00, 'ARS', 'Operaciones', 'transfer', 'monthly', 5, s1, true, (current_date + interval '15 days')::date, 'seed', 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/recurring/2'), v_org, 'Software contable', 45000.00, 'ARS', 'Administración', 'card', 'monthly', 10, NULL, true, (current_date + interval '20 days')::date, 'seed', 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO purchases (id, org_id, number, party_id, party_name, status, payment_status, subtotal, tax_total, total, currency, notes, created_by)
    VALUES
        (pur1, v_org, 'CPA-SEED-001', s1, 'Proveedor Demo 1', 'received', 'paid', 10000.00, 2100.00, 12100.00, 'ARS', 'Compra semilla recibida', 'seed'),
        (pur2, v_org, 'CPA-SEED-002', s1, 'Proveedor Demo 1', 'draft', 'pending', 5000.00, 1050.00, 6050.00, 'ARS', 'Borrador de compra', 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO purchase_items (id, purchase_id, product_id, description, quantity, unit_cost, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/purchase-item/1'), pur1, p1, 'Producto Demo A', 1, 10000.00, 21, 10000.00, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/purchase-item/2'), pur2, p2, 'Producto Demo B', 2, 2500.00, 21, 5000.00, 1)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO procurement_requests (id, org_id, requester_actor, title, description, category, status, estimated_total, currency, created_at, updated_at)
    VALUES
        (pr1, v_org, 'seed', 'Repuestos taller', 'Solicitud demo para filtros y aceite', 'operaciones', 'draft', 125000.00, 'ARS', now(), now())
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO procurement_request_lines (id, request_id, description, product_id, quantity, unit_price_estimate, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/proc-line/1'), pr1, 'Filtro de aceite + mano de obra', p1, 4, 15000.00, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/proc-line/2'), pr1, 'Aceite sintético 4L', NULL, 6, 8500.00, 2)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO webhook_endpoints (id, org_id, url, secret, events, is_active, created_by)
    VALUES
        (wh1, v_org, 'https://example.local/pymes-webhook-demo', 'seed-secret-change-me', ARRAY['sale.created', 'purchase.received'], false, 'seed')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO accounts (id, org_id, type, party_id, party_name, balance, currency, credit_limit)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/account/receivable-c1'), v_org, 'receivable', c1, 'Cliente Demo Uno', 0.00, 'ARS', 500000.00),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/account/payable-s1'), v_org, 'payable', s1, 'Proveedor Demo 1', 0.00, 'ARS', 0)
    ON CONFLICT (id) DO NOTHING;

    UPDATE tenant_settings
       SET next_purchase_number = GREATEST(next_purchase_number, 3),
           updated_at = now()
     WHERE org_id = v_org;
END $$;
