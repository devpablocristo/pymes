-- Demo governance + control: políticas de procurement, empleado (party role=employee),
-- adjuntos, entradas de timeline y audit log.
-- Depende de 02_core_business (clientes) y 04_transversal_modules_demo (compras).

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    sale1 uuid;
    pur1 uuid;
    emp_party uuid;
    pol1 uuid;
    pol2 uuid;
    att1 uuid;
    tl1 uuid;
    tl2 uuid;
    prev_audit_hash text := NULL;
    current_hash text;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    sale1 := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/1');
    pur1 := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/1');
    emp_party := uuid_generate_v5(v_org, 'pymes-seed/v1/employee/1');
    pol1 := uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-policy/1');
    pol2 := uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-policy/2');
    att1 := uuid_generate_v5(v_org, 'pymes-seed/v1/attachment/1');
    tl1 := uuid_generate_v5(v_org, 'pymes-seed/v1/timeline/1');
    tl2 := uuid_generate_v5(v_org, 'pymes-seed/v1/timeline/2');

    -- Empleado (party con rol employee)
    INSERT INTO parties (id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata, created_at, updated_at, deleted_at)
    VALUES (emp_party, v_org, 'person', 'Empleado Demo Uno', 'empleado1@local.dev', '+54-11-4000-0001', '{}'::jsonb, NULL, 'seed', ARRAY['demo'], '{}'::jsonb, now(), now(), NULL)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO party_persons (party_id, first_name, last_name)
    VALUES (emp_party, 'Empleado', 'Demo Uno')
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    VALUES (gen_random_uuid(), emp_party, v_org, 'employee', true, NULL::uuid, jsonb_build_object('position', 'operaciones'), now())
    ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = EXCLUDED.is_active, metadata = EXCLUDED.metadata;

    -- Políticas de procurement
    INSERT INTO procurement_policies (id, org_id, name, expression, effect, priority, mode, enabled, action_filter, system_filter)
    VALUES
        (pol1, v_org, 'Aprobación > 100k', 'amount > 100000', 'require_approval', 10, 'enforce', true, 'procurement.submit', 'pymes'),
        (pol2, v_org, 'Auto-approve < 10k',  'amount < 10000',  'auto_approve',      90, 'enforce', true, 'procurement.submit', 'pymes')
    ON CONFLICT (id) DO NOTHING;

    -- Adjuntos (attachments genéricos adjuntos a venta y compra)
    INSERT INTO attachments (id, org_id, attachable_type, attachable_id, file_name, content_type, size_bytes, storage_key, uploaded_by)
    VALUES
        (att1, v_org, 'sale', sale1, 'comprobante-venta-1.pdf', 'application/pdf', 204800, 'seed/sale/1/comprobante.pdf', 'seed'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/attachment/2'), v_org, 'purchase', pur1, 'factura-proveedor-1.pdf', 'application/pdf', 153600, 'seed/purchase/1/factura.pdf', 'seed')
    ON CONFLICT (id) DO NOTHING;

    -- Timeline entries
    INSERT INTO timeline_entries (id, org_id, entity_type, entity_id, event_type, title, description, actor, metadata)
    VALUES
        (tl1, v_org, 'sale', sale1, 'sale.created', 'Venta registrada', 'Se creó VTA-00001 (seed)', 'seed', jsonb_build_object('amount', 48400)),
        (tl2, v_org, 'purchase', pur1, 'purchase.received', 'Compra recibida', 'CPA-SEED-001 recibida en depósito', 'seed', jsonb_build_object('amount', 12100))
    ON CONFLICT (id) DO NOTHING;

    -- Audit log: entradas hash-encadenadas (prev_hash apunta al hash anterior).
    SELECT hash INTO prev_audit_hash FROM audit_log WHERE org_id = v_org ORDER BY created_at DESC LIMIT 1;

    current_hash := encode(digest(coalesce(prev_audit_hash, '') || 'seed/audit/1', 'sha256'), 'hex');
    INSERT INTO audit_log (id, org_id, actor, action, resource_type, resource_id, payload, prev_hash, hash, created_at)
    VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/audit/1'), v_org, 'seed', 'sale.create', 'sale', sale1::text,
            jsonb_build_object('number', 'VTA-00001', 'total', 48400), prev_audit_hash, current_hash, now() - interval '2 hours')
    ON CONFLICT (id) DO NOTHING;
    prev_audit_hash := current_hash;

    current_hash := encode(digest(coalesce(prev_audit_hash, '') || 'seed/audit/2', 'sha256'), 'hex');
    INSERT INTO audit_log (id, org_id, actor, action, resource_type, resource_id, payload, prev_hash, hash, created_at)
    VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/audit/2'), v_org, 'seed', 'purchase.receive', 'purchase', pur1::text,
            jsonb_build_object('number', 'CPA-SEED-001', 'total', 12100), prev_audit_hash, current_hash, now() - interval '1 hour')
    ON CONFLICT (id) DO NOTHING;
END $$;
