-- Seed ampliado para CRUDs core que hoy forman parte del producto.
-- Mantiene IDs determinísticos por org para permitir re-ejecución idempotente.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    local_user uuid := '00000000-0000-0000-0000-000000000002';
    c1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    s1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/1');
    s2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/2');
    p1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/product/1');
    p2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/product/2');
    p3 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/product/3');
    svc1 uuid;
    svc2 uuid;
    sale1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/1');
    sale2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/2');
    sale_item1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/sale-item/1');
    q1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/quote/1');
    pl_default uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/price-list/default');
    emp_party uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/employee/1');
    pur1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/1');
    pur2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/2');
    pr1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/procurement/1');
    pr2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/procurement/2');
    acc_receivable uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/account/receivable-c1');
    acc_payable uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/account/payable-s1');
    rec1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/recurring/1');
    rec2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/recurring/2');
    wh1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/webhook/1');
    ret1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/return/1');
    cn1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/credit-note/1');
    pay1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/payment/sale/1');
    pay2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/payment/sale/2');
    pay3 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/payment/purchase/1');
    notif1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/demo-welcome');
    notif2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/review-approval');
    inv1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/1');
    inv2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/2');
    inv3 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/3');
    inv4 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/4');
    inv5 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/5');
    emp1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/employee/1');
    emp2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/employee/2');
    emp3 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/employee/3');
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    SELECT id INTO svc1 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO svc2 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    IF svc1 IS NULL OR svc2 IS NULL THEN
        RAISE EXCEPTION 'pymes full seed: expected demo services for org %', v_org;
    END IF;

    INSERT INTO parties (
        id, org_id, party_type, display_name, email, phone, address,
        tax_id, notes, tags, metadata, created_at, updated_at, deleted_at
    )
    VALUES (
        emp_party, v_org, 'person', 'Empleado Demo', 'empleado@local.dev', '+54-11-4000-0001', '{}'::jsonb,
        NULL, 'seed employee', ARRAY['demo', 'employee'], jsonb_build_object('vertical', 'core'),
        now(), now(), NULL
    )
    ON CONFLICT (id) DO UPDATE
        SET display_name = EXCLUDED.display_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now(),
            deleted_at = NULL;

    INSERT INTO party_persons (party_id, first_name, last_name)
    VALUES (emp_party, 'Empleado', 'Demo')
    ON CONFLICT (party_id) DO UPDATE
        SET first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    VALUES (uuid_generate_v5(v_org, 'pymes-seed/v1/employee/role/1'), emp_party, v_org, 'employee', true, NULL::uuid, '{}'::jsonb, now())
    ON CONFLICT (party_id, org_id, role) DO UPDATE
        SET is_active = EXCLUDED.is_active,
            metadata = EXCLUDED.metadata;

    UPDATE org_members
       SET party_id = emp_party,
           role = 'admin'
     WHERE org_id = v_org
       AND user_id = local_user;

    INSERT INTO price_list_items (price_list_id, product_id, price)
    VALUES
        (pl_default, p1, 15000),
        (pl_default, p2, 9500),
        (pl_default, p3, 7300)
    ON CONFLICT (price_list_id, product_id) DO UPDATE
        SET price = EXCLUDED.price;

    INSERT INTO purchases (
        id, org_id, number, party_id, party_name, status, payment_status,
        subtotal, tax_total, total, currency, notes, received_at, created_by
    )
    VALUES
        (pur1, v_org, 'COMP-00001', s1, 'Proveedor Demo 1', 'received', 'partial', 22000, 4620, 26620, 'ARS', 'Compra demo recibida', now() - interval '3 days', 'seed'),
        (pur2, v_org, 'COMP-00002', s2, 'Proveedor Demo 2', 'draft', 'pending', 12000, 2520, 14520, 'ARS', 'Compra demo borrador', NULL, 'seed')
    ON CONFLICT (org_id, number) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            party_name = EXCLUDED.party_name,
            status = EXCLUDED.status,
            payment_status = EXCLUDED.payment_status,
            subtotal = EXCLUDED.subtotal,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            currency = EXCLUDED.currency,
            notes = EXCLUDED.notes,
            received_at = EXCLUDED.received_at,
            updated_at = now();

    INSERT INTO purchase_items (id, purchase_id, product_id, service_id, description, quantity, unit_cost, tax_rate, subtotal, sort_order)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/purchase-item/1'), pur1, p3, NULL, 'Producto Demo C', 2, 6000, 21, 12000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/purchase-item/2'), pur1, NULL, svc2, 'Servicio Demo Mantenimiento', 1, 10000, 21, 10000, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/purchase-item/3'), pur2, p2, NULL, 'Producto Demo B', 2, 6000, 21, 12000, 1)
    ON CONFLICT (id) DO UPDATE
        SET description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_cost = EXCLUDED.unit_cost,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal,
            sort_order = EXCLUDED.sort_order;

    INSERT INTO accounts (id, org_id, type, party_id, party_name, balance, currency, credit_limit, updated_at)
    VALUES
        (acc_receivable, v_org, 'receivable', c1, 'Cliente Demo Uno', 18400, 'ARS', 0, now()),
        (acc_payable, v_org, 'payable', s1, 'Proveedor Demo 1', 26620, 'ARS', 100000, now())
    ON CONFLICT (id) DO UPDATE
        SET balance = EXCLUDED.balance,
            currency = EXCLUDED.currency,
            credit_limit = EXCLUDED.credit_limit,
            updated_at = now();

    INSERT INTO account_movements (
        id, account_id, org_id, type, amount, balance, description,
        reference_type, reference_id, created_by, created_at
    )
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/account-movement/1'), acc_receivable, v_org, 'charge', 48400, 48400, 'Venta demo VTA-00001', 'sale', sale1, 'seed', now() - interval '5 days'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/account-movement/2'), acc_receivable, v_org, 'payment', 30000, 18400, 'Cobro parcial venta demo', 'sale', sale1, 'seed', now() - interval '4 days'),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/account-movement/3'), acc_payable, v_org, 'charge', 26620, 26620, 'Compra demo COMP-00001', 'purchase', pur1, 'seed', now() - interval '3 days')
    ON CONFLICT (id) DO UPDATE
        SET amount = EXCLUDED.amount,
            balance = EXCLUDED.balance,
            description = EXCLUDED.description,
            created_at = EXCLUDED.created_at;

    INSERT INTO recurring_expenses (
        id, org_id, description, amount, currency, category, payment_method,
        frequency, day_of_month, party_id, is_active, next_due_date, last_paid_date,
        notes, created_by, created_at, updated_at
    )
    VALUES
        (rec1, v_org, 'Alquiler local', 350000, 'ARS', 'rent', 'transfer', 'monthly', 5, s2, true, CURRENT_DATE + 10, CURRENT_DATE - 20, 'Seed recurring rent', 'seed', now(), now()),
        (rec2, v_org, 'Internet oficina', 45000, 'ARS', 'services', 'debit', 'monthly', 12, s1, true, CURRENT_DATE + 17, CURRENT_DATE - 13, 'Seed recurring internet', 'seed', now(), now())
    ON CONFLICT (id) DO UPDATE
        SET description = EXCLUDED.description,
            amount = EXCLUDED.amount,
            category = EXCLUDED.category,
            payment_method = EXCLUDED.payment_method,
            frequency = EXCLUDED.frequency,
            day_of_month = EXCLUDED.day_of_month,
            party_id = EXCLUDED.party_id,
            is_active = EXCLUDED.is_active,
            next_due_date = EXCLUDED.next_due_date,
            last_paid_date = EXCLUDED.last_paid_date,
            notes = EXCLUDED.notes,
            updated_at = now();

    INSERT INTO procurement_policies (
        id, org_id, name, expression, effect, priority, mode, enabled,
        action_filter, system_filter, created_at, updated_at
    )
    VALUES
        (
            uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-policy/1'),
            v_org,
            'Auto aprobar compras chicas',
            'request.estimated_total <= 15000',
            'allow',
            10,
            'enforce',
            true,
            'procurement.submit',
            'pymes',
            now(),
            now()
        ),
        (
            uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-policy/2'),
            v_org,
            'Escalar compras medianas',
            'request.estimated_total > 15000',
            'require_approval',
            20,
            'enforce',
            true,
            'procurement.submit',
            'pymes',
            now(),
            now()
        )
    ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            expression = EXCLUDED.expression,
            effect = EXCLUDED.effect,
            priority = EXCLUDED.priority,
            mode = EXCLUDED.mode,
            enabled = EXCLUDED.enabled,
            action_filter = EXCLUDED.action_filter,
            system_filter = EXCLUDED.system_filter,
            updated_at = now();

    INSERT INTO procurement_requests (
        id, org_id, requester_actor, title, description, category, status,
        estimated_total, currency, evaluation_json, purchase_id, created_at, updated_at, archived_at
    )
    VALUES
        (
            pr1, v_org, 'seed', 'Reposición de insumos taller', 'Compra de reposición para stock crítico',
            'inventory', 'pending_approval', 25000, 'ARS',
            jsonb_build_object('decision', 'require_approval', 'policy', 'Escalar compras medianas'),
            NULL, now() - interval '2 days', now() - interval '1 day', NULL
        ),
        (
            pr2, v_org, 'seed', 'Compra aprobada y convertida', 'Solicitud demo ya materializada en compra',
            'operations', 'approved', 26620, 'ARS',
            jsonb_build_object('decision', 'allow', 'policy', 'Auto aprobar compras chicas'),
            pur1, now() - interval '6 days', now() - interval '3 days', NULL
        )
    ON CONFLICT (id) DO UPDATE
        SET requester_actor = EXCLUDED.requester_actor,
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            category = EXCLUDED.category,
            status = EXCLUDED.status,
            estimated_total = EXCLUDED.estimated_total,
            currency = EXCLUDED.currency,
            evaluation_json = EXCLUDED.evaluation_json,
            purchase_id = EXCLUDED.purchase_id,
            updated_at = now(),
            archived_at = EXCLUDED.archived_at;

    INSERT INTO procurement_request_lines (
        id, request_id, description, product_id, quantity, unit_price_estimate, sort_order
    )
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-line/1'), pr1, 'Filtro de aceite', p1, 1, 12000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-line/2'), pr1, 'Lubricante', p3, 2, 6500, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-line/3'), pr2, 'Servicio Demo Mantenimiento', NULL, 1, 10000, 1)
    ON CONFLICT (id) DO UPDATE
        SET description = EXCLUDED.description,
            product_id = EXCLUDED.product_id,
            quantity = EXCLUDED.quantity,
            unit_price_estimate = EXCLUDED.unit_price_estimate,
            sort_order = EXCLUDED.sort_order;

    INSERT INTO payments (
        id, org_id, reference_type, reference_id, method, amount,
        notes, received_at, created_by, created_at
    )
    VALUES
        (pay1, v_org, 'sale', sale1, 'transfer', 30000, 'Cobro parcial seed', now() - interval '4 days', 'seed', now() - interval '4 days'),
        (pay2, v_org, 'sale', sale2, 'cash', 11495, 'Cobro total seed', now() - interval '3 days', 'seed', now() - interval '3 days'),
        (pay3, v_org, 'purchase', pur1, 'transfer', 12000, 'Pago parcial compra seed', now() - interval '2 days', 'seed', now() - interval '2 days')
    ON CONFLICT (id) DO UPDATE
        SET method = EXCLUDED.method,
            amount = EXCLUDED.amount,
            notes = EXCLUDED.notes,
            received_at = EXCLUDED.received_at,
            created_at = EXCLUDED.created_at;

    INSERT INTO returns (
        id, org_id, number, sale_id, reason, subtotal, tax_total, total,
        refund_method, status, notes, created_by, created_at
    )
    VALUES (
        ret1, v_org, 'DEV-00001', sale1, 'defective', 15000, 3150, 18150,
        'credit_note', 'completed', 'Devolución demo por defecto', 'seed', now() - interval '2 days'
    )
    ON CONFLICT (org_id, number) DO UPDATE
        SET sale_id = EXCLUDED.sale_id,
            reason = EXCLUDED.reason,
            subtotal = EXCLUDED.subtotal,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            refund_method = EXCLUDED.refund_method,
            status = EXCLUDED.status,
            notes = EXCLUDED.notes;

    INSERT INTO return_items (
        id, return_id, sale_item_id, product_id, description, quantity, unit_price, tax_rate, subtotal
    )
    VALUES (
        uuid_generate_v5(v_org, 'pymes-seed/v1/return-item/1'),
        ret1, sale_item1, p1, 'Producto Demo A', 1, 15000, 21, 15000
    )
    ON CONFLICT (id) DO UPDATE
        SET quantity = EXCLUDED.quantity,
            unit_price = EXCLUDED.unit_price,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal;

    INSERT INTO credit_notes (
        id, org_id, number, party_id, return_id, amount, used_amount,
        balance, expires_at, status, created_at
    )
    VALUES (
        cn1, v_org, 'NC-00001', c1, ret1, 18150, 0, 18150,
        now() + interval '90 days', 'active', now() - interval '2 days'
    )
    ON CONFLICT (id) DO UPDATE
        SET amount = EXCLUDED.amount,
            used_amount = EXCLUDED.used_amount,
            balance = EXCLUDED.balance,
            expires_at = EXCLUDED.expires_at,
            status = EXCLUDED.status;

    INSERT INTO audit_log (
        id, org_id, actor, action, resource_type, resource_id, payload,
        prev_hash, hash, created_at, actor_type, actor_id, actor_label
    )
    VALUES
        (
            uuid_generate_v5(v_org, 'pymes-seed/v1/audit/1'),
            v_org, 'seed', 'sale.created', 'sale', sale1::text,
            jsonb_build_object('number', 'VTA-00001', 'total', 48400),
            NULL,
            md5(v_org::text || ':audit:1'),
            now() - interval '5 days',
            'user', local_user, 'Local Admin'
        ),
        (
            uuid_generate_v5(v_org, 'pymes-seed/v1/audit/2'),
            v_org, 'seed', 'purchase.received', 'purchase', pur1::text,
            jsonb_build_object('number', 'COMP-00001', 'total', 26620),
            md5(v_org::text || ':audit:1'),
            md5(v_org::text || ':audit:2'),
            now() - interval '3 days',
            'user', local_user, 'Local Admin'
        )
    ON CONFLICT (id) DO UPDATE
        SET action = EXCLUDED.action,
            resource_type = EXCLUDED.resource_type,
            resource_id = EXCLUDED.resource_id,
            payload = EXCLUDED.payload,
            prev_hash = EXCLUDED.prev_hash,
            hash = EXCLUDED.hash,
            created_at = EXCLUDED.created_at,
            actor_label = EXCLUDED.actor_label;

    INSERT INTO timeline_entries (
        id, org_id, entity_type, entity_id, event_type, title,
        description, actor, metadata, created_at
    )
    VALUES
        (
            uuid_generate_v5(v_org, 'pymes-seed/v1/timeline/1'),
            v_org, 'sales', sale1, 'note', 'Seguimiento comercial',
            'Cliente confirmó recepción del presupuesto y pasó a venta.', 'seed',
            jsonb_build_object('source', 'seed'), now() - interval '5 days'
        ),
        (
            uuid_generate_v5(v_org, 'pymes-seed/v1/timeline/2'),
            v_org, 'purchases', pur1, 'note', 'Recepción parcial',
            'La compra demo quedó recibida parcialmente con saldo pendiente.', 'seed',
            jsonb_build_object('source', 'seed'), now() - interval '3 days'
        )
    ON CONFLICT (id) DO UPDATE
        SET title = EXCLUDED.title,
            description = EXCLUDED.description,
            actor = EXCLUDED.actor,
            metadata = EXCLUDED.metadata,
            created_at = EXCLUDED.created_at;

    INSERT INTO webhook_endpoints (
        id, org_id, url, secret, events, is_active, created_by, created_at, updated_at
    )
    VALUES (
        wh1, v_org, 'https://example.invalid/hooks/pymes', 'seed-secret',
        ARRAY['sale.created', 'customer.updated', 'purchase.received'], true, 'seed', now(), now()
    )
    ON CONFLICT (id) DO UPDATE
        SET url = EXCLUDED.url,
            secret = EXCLUDED.secret,
            events = EXCLUDED.events,
            is_active = EXCLUDED.is_active,
            updated_at = now();

    INSERT INTO pymes_in_app_notifications (
        id, org_id, user_id, title, body, kind, entity_type, entity_id, chat_context, created_at
    )
    VALUES
        (
            notif1, v_org, local_user, 'Bienvenido al demo',
            'Se cargó un set de datos de ejemplo para revisar el producto.', 'system',
            'org', v_org::text, '{}'::jsonb, now() - interval '1 day'
        ),
        (
            notif2, v_org, local_user, 'Solicitud pendiente',
            'La solicitud de compra demo quedó pendiente de aprobación.', 'approval',
            'procurement_request', pr1::text, jsonb_build_object('action', 'approve'), now() - interval '12 hours'
        )
    ON CONFLICT (id) DO UPDATE
        SET title = EXCLUDED.title,
            body = EXCLUDED.body,
            kind = EXCLUDED.kind,
            entity_type = EXCLUDED.entity_type,
            entity_id = EXCLUDED.entity_id,
            chat_context = EXCLUDED.chat_context,
            created_at = EXCLUDED.created_at;

    -- Invoices demo (F1): 5 facturas con line items, cubren los 3 estados (paid/pending/overdue).
    INSERT INTO invoices (
        id, org_id, number, customer_name, issued_date, due_date, status,
        subtotal, discount_percent, tax_percent, total, notes, is_favorite, tags, created_by
    ) VALUES
        (inv1, v_org, 'INV-4001', 'Distribuidora Norte',   DATE '2026-04-05', DATE '2026-04-15', 'paid',    107500, 5,  21, 123661.125, '', true,  ARRAY['mayorista','recurrente']::text[], 'seed'),
        (inv2, v_org, 'INV-4002', 'Café Central',          DATE '2026-04-15', DATE '2026-04-25', 'pending',  90800, 0,  21, 109868,     '', false, ARRAY['gastronomia']::text[],            'seed'),
        (inv3, v_org, 'INV-4003', 'Ferretería Sur',        DATE '2026-03-10', DATE '2026-03-20', 'overdue',  57000, 0,  21, 68970,      '', false, ARRAY['urgente','cobrar']::text[],       'seed'),
        (inv4, v_org, 'INV-4004', 'Taller Mecánica Beta',  DATE '2026-04-20', DATE '2026-05-05', 'pending',  81000, 10, 21, 88209,      '', false, ARRAY['soporte']::text[],                'seed'),
        (inv5, v_org, 'INV-4005', 'Panadería La Esquina',  DATE '2026-04-12', DATE '2026-04-22', 'paid',     92000, 0,  21, 111320,     '', true,  ARRAY['gastronomia','mayorista']::text[], 'seed')
    ON CONFLICT (id) DO UPDATE
        SET number = EXCLUDED.number,
            customer_name = EXCLUDED.customer_name,
            issued_date = EXCLUDED.issued_date,
            due_date = EXCLUDED.due_date,
            status = EXCLUDED.status,
            subtotal = EXCLUDED.subtotal,
            discount_percent = EXCLUDED.discount_percent,
            tax_percent = EXCLUDED.tax_percent,
            total = EXCLUDED.total,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            updated_at = now();

    -- Line items asociados (limpiamos y recreamos idempotente).
    DELETE FROM invoice_line_items WHERE invoice_id IN (inv1, inv2, inv3, inv4, inv5);
    INSERT INTO invoice_line_items (id, invoice_id, description, qty, unit, unit_price, line_total, sort_order) VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/1/line/1'), inv1, 'Instalación de red',         1,  'servicio', 85000, 85000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/1/line/2'), inv1, 'Cable UTP cat 6',            50, 'metro',      450, 22500, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/2/line/1'), inv2, 'Café en grano premium',      10, 'kg',        7800, 78000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/2/line/2'), inv2, 'Vajilla descartable',         4, 'caja',      3200, 12800, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/3/line/1'), inv3, 'Asesoría técnica',            6, 'hora',      9500, 57000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/4/line/1'), inv4, 'Mantenimiento de software',   1, 'servicio', 45000, 45000, 1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/4/line/2'), inv4, 'Capacitación de equipo',      3, 'hora',     12000, 36000, 2),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/invoice/5/line/1'), inv5, 'Harina 000 bolsa 25 kg',      8, 'bolsa',    11500, 92000, 1);

    -- Employees demo (F1): 3 empleados cubriendo estados active / inactive.
    INSERT INTO employees (
        id, org_id, first_name, last_name, email, phone, position, status,
        hire_date, notes, is_favorite, tags, created_by
    ) VALUES
        (emp1, v_org, 'María',    'Gómez',    'maria.gomez@demo.pymes',    '+54 9 11 2345 6789', 'Administración',     'active',   DATE '2024-03-01', '',  true,  ARRAY['administracion']::text[], 'seed'),
        (emp2, v_org, 'Carlos',   'Ramírez',  'carlos.ramirez@demo.pymes', '+54 9 11 4567 8901', 'Operario',           'active',   DATE '2023-11-15', '',  false, ARRAY['operaciones']::text[],    'seed'),
        (emp3, v_org, 'Lucía',    'Fernández','lucia.fernandez@demo.pymes','+54 9 11 6789 0123', 'Atención al cliente','inactive', DATE '2022-07-20', '',  false, ARRAY['comercial']::text[],      'seed')
    ON CONFLICT (id) DO UPDATE
        SET first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            position = EXCLUDED.position,
            status = EXCLUDED.status,
            hire_date = EXCLUDED.hire_date,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            updated_at = now();
END $$;
