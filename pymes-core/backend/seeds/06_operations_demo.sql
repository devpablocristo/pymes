-- Demo operations: cobros/pagos de ventas y compras, devoluciones, notas de crédito.
-- Depende de 02_core_business (ventas) y 04_transversal_modules_demo (compras).

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    c1 uuid;
    c2 uuid;
    sale1 uuid;
    sale2 uuid;
    pur1 uuid;
    ret1 uuid;
    pay_sale1 uuid;
    pay_sale2 uuid;
    pay_pur1 uuid;
    cn1 uuid;
    cn2 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    sale1 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/1');
    sale2 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/2');
    pur1 := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/1');
    ret1 := uuid_generate_v5(v_org, 'pymes-seed/v1/return/1');
    pay_sale1 := uuid_generate_v5(v_org, 'pymes-seed/v1/payment/sale/1');
    pay_sale2 := uuid_generate_v5(v_org, 'pymes-seed/v1/payment/sale/2');
    pay_pur1 := uuid_generate_v5(v_org, 'pymes-seed/v1/payment/purchase/1');
    cn1 := uuid_generate_v5(v_org, 'pymes-seed/v1/credit-note/1');
    cn2 := uuid_generate_v5(v_org, 'pymes-seed/v1/credit-note/2');

    -- Cobros (ventas) y pagos (compras)
    INSERT INTO payments (id, org_id, reference_type, reference_id, method, amount, notes, created_by)
    VALUES
        (pay_sale1, v_org, 'sale', sale1, 'transfer', 48400.00, 'Cobro total venta 1 (seed)', 'seed'),
        (pay_sale2, v_org, 'sale', sale2, 'cash', 11495.00, 'Cobro total venta 2 (seed)', 'seed'),
        (pay_pur1, v_org, 'purchase', pur1, 'transfer', 12100.00, 'Pago a proveedor (seed)', 'seed')
    ON CONFLICT (id) DO NOTHING;

    -- Devoluciones (sobre ventas completadas)
    INSERT INTO returns (id, org_id, number, sale_id, reason, subtotal, tax_total, total, refund_method, status, notes, created_by)
    VALUES
        (ret1, v_org, 'DEV-SEED-001', sale1, 'defective', 15000.00, 3150.00, 18150.00, 'credit_note', 'completed', 'Devolución de producto defectuoso (seed)', 'seed')
    ON CONFLICT (id) DO NOTHING;

    -- Notas de crédito (una vinculada a la devolución, otra independiente)
    INSERT INTO credit_notes (id, org_id, number, party_id, return_id, amount, used_amount, balance, status)
    VALUES
        (cn1, v_org, 'NC-SEED-001', c1, ret1, 18150.00,     0.00, 18150.00, 'active'),
        (cn2, v_org, 'NC-SEED-002', c2, NULL,  5000.00,  5000.00,     0.00, 'used')
    ON CONFLICT (id) DO NOTHING;
END $$;
