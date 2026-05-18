-- Demo bulk: completa las entidades visibles hasta 10 filas por modulo core.
-- Corre despues de 01..05 y mantiene IDs deterministicos por org para re-ejecucion.

DO $$
DECLARE
    v_tenant uuid := '__SEED_TENANT_ID__';
    local_user uuid;
    v_branch uuid;
    v_sched_service uuid;
    v_sched_resource uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_tenant) THEN
        RETURN;
    END IF;

    SELECT user_id INTO local_user
      FROM org_members
     WHERE org_id = v_tenant
       AND role = 'owner'
       AND status = 'active'
     ORDER BY created_at
     LIMIT 1;

    IF local_user IS NULL THEN
        RAISE EXCEPTION 'pymes bulk seed: expected active owner membership for tenant %', v_tenant;
    END IF;

    SELECT id INTO v_branch
      FROM scheduling_branches
     WHERE org_id = v_tenant
     ORDER BY created_at
     LIMIT 1;

    SELECT id INTO v_sched_service
      FROM scheduling_services
     WHERE org_id = v_tenant
       AND active = true
     ORDER BY CASE WHEN code = 'general_consultation' THEN 0 ELSE 1 END, code
     LIMIT 1;

    SELECT id INTO v_sched_resource
      FROM scheduling_resources
     WHERE org_id = v_tenant
       AND active = true
     ORDER BY code
     LIMIT 1;

    -- Clientes 4..10.
    INSERT INTO parties (
        id, org_id, party_type, display_name, email, phone, address,
        tax_id, notes, tags, metadata, created_at, updated_at, deleted_at, is_favorite
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        v_tenant,
        CASE WHEN gs % 3 = 0 THEN 'organization' ELSE 'person' END,
        CASE gs
            WHEN 4 THEN 'Almacen Don Luis'
            WHEN 5 THEN 'Cafeteria Central'
            WHEN 6 THEN 'Distribuidora Norte'
            WHEN 7 THEN 'Ferreteria Sur'
            WHEN 8 THEN 'Panaderia La Esquina'
            WHEN 9 THEN 'Taller Beta'
            ELSE 'Mercado Plaza'
        END,
        'cliente' || gs::text || '@demo.pymes',
        '+54-11-1000-' || lpad(gs::text, 4, '0'),
        jsonb_build_object('city', 'Buenos Aires', 'street', 'Calle Demo ' || gs::text),
        CASE WHEN gs % 3 = 0 THEN '30' || lpad((70000000 + gs)::text, 9, '0') ELSE NULL END,
        'seed bulk customer',
        ARRAY['demo', 'customer'],
        jsonb_build_object('segment', CASE WHEN gs % 2 = 0 THEN 'mayorista' ELSE 'minorista' END),
        now() - make_interval(days => 20 - gs),
        now(),
        NULL,
        gs IN (6, 8)
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET display_name = EXCLUDED.display_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            address = EXCLUDED.address,
            tax_id = EXCLUDED.tax_id,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now(),
            deleted_at = NULL,
            is_favorite = EXCLUDED.is_favorite;

    INSERT INTO party_persons (party_id, first_name, last_name)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        split_part(name, ' ', 1),
        substring(name from position(' ' in name) + 1)
    FROM (
        SELECT gs, CASE gs
            WHEN 4 THEN 'Almacen Don Luis'
            WHEN 5 THEN 'Cafeteria Central'
            WHEN 7 THEN 'Ferreteria Sur'
            WHEN 8 THEN 'Panaderia La Esquina'
            ELSE 'Mercado Plaza'
        END AS name
        FROM generate_series(4, 10) AS gs
        WHERE gs % 3 <> 0
    ) src
    ON CONFLICT (party_id) DO UPDATE
        SET first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name;

    INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        CASE gs WHEN 6 THEN 'Distribuidora Norte SRL' WHEN 9 THEN 'Taller Beta SA' END,
        CASE gs WHEN 6 THEN 'Distribuidora Norte' WHEN 9 THEN 'Taller Beta' END,
        'responsable_inscripto'
    FROM generate_series(4, 10) AS gs
    WHERE gs % 3 = 0
    ON CONFLICT (party_id) DO UPDATE
        SET legal_name = EXCLUDED.legal_name,
            trade_name = EXCLUDED.trade_name,
            tax_condition = EXCLUDED.tax_condition;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer-role/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        v_tenant,
        'customer',
        true,
        NULL::uuid,
        jsonb_build_object('source', 'seed-bulk'),
        now()
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (party_id, org_id, role) DO UPDATE
        SET is_active = EXCLUDED.is_active,
            metadata = EXCLUDED.metadata;

    -- Proveedores 4..10.
    INSERT INTO parties (
        id, org_id, party_type, display_name, email, phone, address,
        tax_id, notes, tags, metadata, created_at, updated_at, deleted_at, is_favorite
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text),
        v_tenant,
        'organization',
        CASE gs
            WHEN 4 THEN 'Insumos Rio'
            WHEN 5 THEN 'Logistica Federal'
            WHEN 6 THEN 'Servicios Tecnicos Delta'
            WHEN 7 THEN 'Papeleria Norte'
            WHEN 8 THEN 'Mayorista Sur'
            WHEN 9 THEN 'Equipos Centro'
            ELSE 'Mantenimiento Express'
        END,
        'proveedor' || gs::text || '@demo.pymes',
        '+54-11-2000-' || lpad(gs::text, 4, '0'),
        jsonb_build_object('city', 'Buenos Aires', 'street', 'Proveedor Demo ' || gs::text),
        '30' || lpad((80000000 + gs)::text, 9, '0'),
        'seed bulk supplier',
        ARRAY['demo', 'supplier'],
        jsonb_build_object('contact_name', 'Contacto ' || gs::text),
        now() - make_interval(days => 25 - gs),
        now(),
        NULL,
        gs IN (4, 8)
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET display_name = EXCLUDED.display_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            address = EXCLUDED.address,
            tax_id = EXCLUDED.tax_id,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now(),
            deleted_at = NULL,
            is_favorite = EXCLUDED.is_favorite;

    INSERT INTO party_organizations (party_id, legal_name, trade_name, tax_condition)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text),
        'Proveedor Demo ' || gs::text || ' SRL',
        CASE gs
            WHEN 4 THEN 'Insumos Rio'
            WHEN 5 THEN 'Logistica Federal'
            WHEN 6 THEN 'Servicios Tecnicos Delta'
            WHEN 7 THEN 'Papeleria Norte'
            WHEN 8 THEN 'Mayorista Sur'
            WHEN 9 THEN 'Equipos Centro'
            ELSE 'Mantenimiento Express'
        END,
        'responsable_inscripto'
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (party_id) DO UPDATE
        SET legal_name = EXCLUDED.legal_name,
            trade_name = EXCLUDED.trade_name,
            tax_condition = EXCLUDED.tax_condition;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier-role/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text),
        v_tenant,
        'supplier',
        true,
        NULL::uuid,
        jsonb_build_object('source', 'seed-bulk'),
        now()
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (party_id, org_id, role) DO UPDATE
        SET is_active = EXCLUDED.is_active,
            metadata = EXCLUDED.metadata;

    -- Productos 4..10.
    INSERT INTO products (
        id, org_id, type, sku, name, description, unit, price, cost_price,
        tax_rate, track_stock, tags, metadata, price_currency, is_active,
        image_url, image_urls, is_favorite, created_at, updated_at, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text),
        v_tenant,
        'product',
        'DEMO-PROD-' || lpad(gs::text, 3, '0'),
        CASE gs
            WHEN 4 THEN 'Kit instalacion premium'
            WHEN 5 THEN 'Cable UTP Cat 6'
            WHEN 6 THEN 'Router empresarial'
            WHEN 7 THEN 'Modulo sensor'
            WHEN 8 THEN 'Insumo embalaje'
            WHEN 9 THEN 'Repuesto critico'
            ELSE 'Pack mantenimiento'
        END,
        'Producto demo ampliado ' || gs::text,
        CASE WHEN gs IN (5, 8) THEN 'metro' ELSE 'unit' END,
        6000 + gs * 1750,
        3500 + gs * 950,
        21,
        true,
        ARRAY['demo', 'product'],
        jsonb_build_object('source', 'seed-bulk'),
        'ARS',
        true,
        '',
        ARRAY[]::text[],
        gs IN (6, 9),
        now() - make_interval(days => 18 - gs),
        now(),
        NULL
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET sku = EXCLUDED.sku,
            name = EXCLUDED.name,
            description = EXCLUDED.description,
            unit = EXCLUDED.unit,
            price = EXCLUDED.price,
            cost_price = EXCLUDED.cost_price,
            tax_rate = EXCLUDED.tax_rate,
            track_stock = EXCLUDED.track_stock,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            price_currency = EXCLUDED.price_currency,
            is_active = EXCLUDED.is_active,
            is_favorite = EXCLUDED.is_favorite,
            updated_at = now(),
            deleted_at = NULL;

    -- Servicios 4..8. Workshops agrega 2 servicios publicos mas, dejando 10 visibles en total.
    INSERT INTO services (
        id, org_id, code, name, description, category_code, sale_price,
        cost_price, tax_rate, currency, default_duration_minutes, tags,
        metadata, is_active, is_favorite, created_at, updated_at, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/service/' || gs::text),
        v_tenant,
        'DEMO-SVC-' || lpad(gs::text, 3, '0'),
        CASE gs
            WHEN 4 THEN 'Diagnostico tecnico'
            WHEN 5 THEN 'Capacitacion express'
            WHEN 6 THEN 'Soporte remoto'
            WHEN 7 THEN 'Mantenimiento mensual'
            WHEN 8 THEN 'Instalacion avanzada'
            WHEN 9 THEN 'Auditoria operativa'
            ELSE 'Puesta en marcha'
        END,
        'Servicio demo ampliado ' || gs::text,
        CASE WHEN gs IN (6, 9) THEN 'consulting' ELSE 'general' END,
        12000 + gs * 3200,
        6000 + gs * 1300,
        21,
        'ARS',
        30 + gs * 5,
        ARRAY['demo', 'service'],
        jsonb_build_object('source', 'seed-bulk'),
        true,
        gs IN (7, 10),
        now() - make_interval(days => 18 - gs),
        now(),
        NULL
    FROM generate_series(4, 8) AS gs
    ON CONFLICT (id) DO UPDATE
        SET code = EXCLUDED.code,
            name = EXCLUDED.name,
            description = EXCLUDED.description,
            category_code = EXCLUDED.category_code,
            sale_price = EXCLUDED.sale_price,
            cost_price = EXCLUDED.cost_price,
            tax_rate = EXCLUDED.tax_rate,
            currency = EXCLUDED.currency,
            default_duration_minutes = EXCLUDED.default_duration_minutes,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            is_active = EXCLUDED.is_active,
            is_favorite = EXCLUDED.is_favorite,
            updated_at = now(),
            deleted_at = NULL;

    INSERT INTO stock_levels (org_id, product_id, quantity, min_quantity)
    SELECT
        v_tenant,
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text),
        12 + gs * 4,
        3 + (gs % 4)
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (org_id, product_id) WHERE branch_id IS NULL DO UPDATE
        SET quantity = EXCLUDED.quantity,
            min_quantity = EXCLUDED.min_quantity,
            updated_at = now();

    -- Presupuestos 4..10 + item asociado.
    INSERT INTO quotes (
        id, org_id, number, party_id, party_name, status, subtotal,
        tax_total, total, currency, notes, valid_until, created_by,
        discount_type, discount_value, discount_total, tags, metadata,
        is_favorite, created_at, updated_at, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/quote/' || gs::text),
        v_tenant,
        'PRE-' || lpad(gs::text, 5, '0'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        (SELECT display_name FROM parties WHERE id = uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)),
        (ARRAY['draft','sent','accepted','rejected','expired'])[(gs % 5) + 1],
        18000 + gs * 3500,
        round((18000 + gs * 3500) * 0.21, 2),
        round((18000 + gs * 3500) * 1.21, 2),
        'ARS',
        'Presupuesto seed ampliado ' || gs::text,
        now() + make_interval(days => 20 + gs),
        'seed',
        'none',
        0,
        0,
        ARRAY['demo', 'quote'],
        jsonb_build_object('source', 'seed-bulk'),
        gs IN (6, 10),
        now() - make_interval(days => 15 - gs),
        now(),
        NULL
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (org_id, number) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            party_name = EXCLUDED.party_name,
            status = EXCLUDED.status,
            subtotal = EXCLUDED.subtotal,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            notes = EXCLUDED.notes,
            valid_until = EXCLUDED.valid_until,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            is_favorite = EXCLUDED.is_favorite,
            updated_at = now(),
            deleted_at = NULL;

    INSERT INTO quote_items (
        id, quote_id, product_id, service_id, description, quantity,
        unit_price, tax_rate, subtotal, sort_order, discount_type, discount_value
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/quote-item/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/quote/' || gs::text),
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text) ELSE NULL END,
        CASE WHEN gs % 2 = 1 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/service/' || LEAST(gs, 8)::text) ELSE NULL END,
        CASE WHEN gs % 2 = 0 THEN 'Producto presupuesto demo ' ELSE 'Servicio presupuesto demo ' END || gs::text,
        CASE WHEN gs % 2 = 0 THEN 2 ELSE 1 END,
        CASE WHEN gs % 2 = 0 THEN 9000 + gs * 800 ELSE 18000 + gs * 1000 END,
        21,
        CASE WHEN gs % 2 = 0 THEN (2 * (9000 + gs * 800)) ELSE (18000 + gs * 1000) END,
        1,
        'none',
        0
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET product_id = EXCLUDED.product_id,
            service_id = EXCLUDED.service_id,
            description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_price = EXCLUDED.unit_price,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal,
            sort_order = EXCLUDED.sort_order;

    -- Ventas 4..10 + item.
    INSERT INTO sales (
        id, org_id, number, party_id, party_name, quote_id, status, payment_method,
        subtotal, tax_total, total, currency, notes, created_by, amount_paid,
        payment_status, discount_type, discount_value, discount_total,
        tags, metadata, is_favorite, created_at, voided_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text),
        v_tenant,
        'VTA-' || lpad(gs::text, 5, '0'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        (SELECT display_name FROM parties WHERE id = uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)),
        CASE WHEN gs IN (4, 6, 8, 10) THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/quote/' || gs::text) ELSE NULL END,
        'completed',
        (ARRAY['cash','card','transfer','check','other','credit','mixed'])[(gs % 7) + 1],
        16000 + gs * 4200,
        round((16000 + gs * 4200) * 0.21, 2),
        round((16000 + gs * 4200) * 1.21, 2),
        'ARS',
        'Venta seed ampliada ' || gs::text,
        'seed',
        CASE WHEN gs % 3 = 0 THEN round((16000 + gs * 4200) * 0.6 * 1.21, 2) ELSE round((16000 + gs * 4200) * 1.21, 2) END,
        CASE WHEN gs % 3 = 0 THEN 'partial' ELSE 'paid' END,
        'none',
        0,
        0,
        ARRAY['demo', 'sale'],
        jsonb_build_object('source', 'seed-bulk'),
        gs IN (5, 9),
        now() - make_interval(days => 14 - gs),
        NULL
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (org_id, number) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            party_name = EXCLUDED.party_name,
            quote_id = EXCLUDED.quote_id,
            status = EXCLUDED.status,
            payment_method = EXCLUDED.payment_method,
            subtotal = EXCLUDED.subtotal,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            notes = EXCLUDED.notes,
            amount_paid = EXCLUDED.amount_paid,
            payment_status = EXCLUDED.payment_status,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            is_favorite = EXCLUDED.is_favorite,
            voided_at = NULL;

    INSERT INTO sale_items (
        id, sale_id, product_id, service_id, description, quantity,
        unit_price, cost_price, tax_rate, subtotal, sort_order, discount_type, discount_value
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale-item/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text),
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text) ELSE NULL END,
        CASE WHEN gs % 2 = 1 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/service/' || LEAST(gs, 8)::text) ELSE NULL END,
        CASE WHEN gs % 2 = 0 THEN 'Producto venta demo ' ELSE 'Servicio venta demo ' END || gs::text,
        CASE WHEN gs % 2 = 0 THEN 2 ELSE 1 END,
        CASE WHEN gs % 2 = 0 THEN 8000 + gs * 900 ELSE 17000 + gs * 900 END,
        CASE WHEN gs % 2 = 0 THEN 4800 + gs * 500 ELSE 8000 + gs * 500 END,
        21,
        CASE WHEN gs % 2 = 0 THEN (2 * (8000 + gs * 900)) ELSE (17000 + gs * 900) END,
        1,
        'none',
        0
    FROM generate_series(4, 10) AS gs
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

    INSERT INTO stock_movements (id, org_id, product_id, type, quantity, reason, reference_id, notes, created_by, created_at)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/stock-move/' || gs::text),
        v_tenant,
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text),
        CASE WHEN gs % 2 = 0 THEN 'out' ELSE 'in' END,
        CASE WHEN gs % 2 = 0 THEN -2 ELSE 8 END,
        CASE WHEN gs % 2 = 0 THEN 'sale' ELSE 'purchase' END,
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text) ELSE NULL END,
        'Movimiento stock seed ampliado ' || gs::text,
        'seed',
        now() - make_interval(days => 14 - gs)
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET quantity = EXCLUDED.quantity,
            reason = EXCLUDED.reason,
            reference_id = EXCLUDED.reference_id,
            notes = EXCLUDED.notes,
            created_at = EXCLUDED.created_at;

    INSERT INTO cash_movements (
        id, org_id, type, amount, currency, category, description,
        payment_method, reference_type, reference_id, created_by,
        created_at, is_favorite, tags, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/cash-move/' || gs::text),
        v_tenant,
        CASE WHEN gs IN (7, 10) THEN 'expense' ELSE 'income' END,
        CASE WHEN gs IN (7, 10) THEN 9000 + gs * 600 ELSE round((16000 + gs * 4200) * 1.21, 2) END,
        'ARS',
        CASE WHEN gs IN (7, 10) THEN 'operations' ELSE 'sale' END,
        'Movimiento caja seed ampliado ' || gs::text,
        (ARRAY['cash','card','transfer','check','other'])[(gs % 5) + 1],
        CASE WHEN gs IN (7, 10) THEN 'expense' ELSE 'sale' END,
        CASE WHEN gs IN (7, 10) THEN NULL ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text) END,
        'seed',
        now() - make_interval(days => 14 - gs),
        gs IN (6, 9),
        ARRAY['demo', 'cashflow'],
        NULL
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET type = EXCLUDED.type,
            amount = EXCLUDED.amount,
            category = EXCLUDED.category,
            description = EXCLUDED.description,
            payment_method = EXCLUDED.payment_method,
            reference_type = EXCLUDED.reference_type,
            reference_id = EXCLUDED.reference_id,
            created_at = EXCLUDED.created_at,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            deleted_at = NULL;

    -- Compras 3..10 + item.
    INSERT INTO purchases (
        id, org_id, number, party_id, party_name, status, payment_status,
        subtotal, tax_total, total, currency, notes, received_at, created_by,
        tags, metadata, is_favorite, created_at, updated_at, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase/' || gs::text),
        v_tenant,
        'COMP-' || lpad(gs::text, 5, '0'),
        CASE
            WHEN gs = 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/supplier/3')
            ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text)
        END,
        (
            SELECT display_name
            FROM parties
            WHERE id = CASE
                WHEN gs = 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/supplier/3')
                ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text)
            END
        ),
        (ARRAY['draft','received','partial'])[(gs % 3) + 1],
        (ARRAY['pending','partial','paid'])[(gs % 3) + 1],
        10000 + gs * 2600,
        round((10000 + gs * 2600) * 0.21, 2),
        round((10000 + gs * 2600) * 1.21, 2),
        'ARS',
        'Compra seed ampliada ' || gs::text,
        CASE WHEN gs % 2 = 0 THEN now() - make_interval(days => 10 - gs) ELSE NULL END,
        'seed',
        ARRAY['demo', 'purchase'],
        jsonb_build_object('source', 'seed-bulk'),
        gs IN (4, 8),
        now() - make_interval(days => 12 - gs),
        now(),
        NULL
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (org_id, number) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            party_name = EXCLUDED.party_name,
            status = EXCLUDED.status,
            payment_status = EXCLUDED.payment_status,
            subtotal = EXCLUDED.subtotal,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            notes = EXCLUDED.notes,
            received_at = EXCLUDED.received_at,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            is_favorite = EXCLUDED.is_favorite,
            updated_at = now(),
            deleted_at = NULL;

    INSERT INTO purchase_items (id, purchase_id, product_id, service_id, description, quantity, unit_cost, tax_rate, subtotal, sort_order)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase-item/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase/' || gs::text),
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text) ELSE NULL END,
        CASE
            WHEN gs = 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/product/6')
            WHEN gs % 2 = 1 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/service/' || LEAST(gs, 8)::text)
            ELSE NULL
        END,
        'Item compra demo ' || gs::text,
        CASE WHEN gs % 2 = 0 THEN 3 ELSE 1 END,
        7000 + gs * 500,
        21,
        CASE WHEN gs % 2 = 0 THEN 3 * (7000 + gs * 500) ELSE 7000 + gs * 500 END,
        1
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET product_id = EXCLUDED.product_id,
            service_id = EXCLUDED.service_id,
            description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_cost = EXCLUDED.unit_cost,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal,
            sort_order = EXCLUDED.sort_order;

    -- Cuentas y movimientos hasta 10.
    INSERT INTO accounts (id, org_id, type, party_id, party_name, balance, currency, credit_limit, updated_at)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/account/' || gs::text),
        v_tenant,
        CASE WHEN gs % 2 = 0 THEN 'receivable' ELSE 'payable' END,
        CASE
            WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)
            WHEN gs = 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/supplier/3')
            ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text)
        END,
        (
            SELECT display_name
            FROM parties
            WHERE id = CASE
                WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)
                WHEN gs = 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/supplier/3')
                ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || gs::text)
            END
        ),
        CASE WHEN gs % 2 = 0 THEN 5000 + gs * 1200 ELSE 8000 + gs * 1600 END,
        'ARS',
        CASE WHEN gs % 2 = 0 THEN 0 ELSE 150000 END,
        now()
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET balance = EXCLUDED.balance,
            credit_limit = EXCLUDED.credit_limit,
            updated_at = now();

    INSERT INTO account_movements (
        id, account_id, org_id, type, amount, balance, description,
        reference_type, reference_id, created_by, created_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/account-movement/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/account/' || gs::text),
        v_tenant,
        CASE WHEN gs % 2 = 0 THEN 'charge' ELSE 'payment' END,
        4000 + gs * 900,
        5000 + gs * 1200,
        'Movimiento cuenta seed ampliado ' || gs::text,
        CASE WHEN gs % 2 = 0 THEN 'sale' ELSE 'purchase' END,
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text) ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase/' || gs::text) END,
        'seed',
        now() - make_interval(days => 12 - gs)
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET amount = EXCLUDED.amount,
            balance = EXCLUDED.balance,
            description = EXCLUDED.description,
            reference_type = EXCLUDED.reference_type,
            reference_id = EXCLUDED.reference_id,
            created_at = EXCLUDED.created_at;

    -- Pagos 4..10.
    INSERT INTO payments (
        id, org_id, reference_type, reference_id, method, amount,
        notes, received_at, created_by, created_at, is_favorite, tags, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/payment/' || gs::text),
        v_tenant,
        CASE WHEN gs IN (7, 10) THEN 'purchase' ELSE 'sale' END,
        CASE WHEN gs IN (7, 10) THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase/' || gs::text) ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text) END,
        (ARRAY['cash','card','transfer','check','other','mercadopago'])[(gs % 6) + 1],
        CASE WHEN gs IN (7, 10) THEN 8000 + gs * 700 ELSE round((16000 + gs * 4200) * 0.7, 2) END,
        'Pago seed ampliado ' || gs::text,
        now() - make_interval(days => 12 - gs),
        'seed',
        now() - make_interval(days => 12 - gs),
        gs IN (5, 8),
        ARRAY['demo', 'payment'],
        NULL
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET method = EXCLUDED.method,
            amount = EXCLUDED.amount,
            notes = EXCLUDED.notes,
            received_at = EXCLUDED.received_at,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            deleted_at = NULL;

    -- Gastos recurrentes 3..10, sumados al seed base quedan 10 visibles.
    INSERT INTO recurring_expenses (
        id, org_id, description, amount, currency, category, payment_method,
        frequency, day_of_month, party_id, is_active, next_due_date,
        last_paid_date, notes, created_by, created_at, updated_at,
        is_favorite, tags, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/recurring/' || gs::text),
        v_tenant,
        (ARRAY['Seguro local','Software gestion','Limpieza','Telefonia','Monitoreo','Alarma','Honorarios','Hosting'])[(gs - 3) + 1],
        25000 + gs * 4500,
        'ARS',
        (ARRAY['insurance','software','services','communications','security','security','fees','software'])[(gs - 3) + 1],
        (ARRAY['transfer','debit','cash','card','debit','transfer','transfer','card'])[(gs - 3) + 1],
        (ARRAY['monthly','monthly','weekly','monthly','monthly','monthly','monthly','yearly'])[(gs - 3) + 1],
        LEAST(28, gs + 3),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/supplier/' || LEAST(10, gs + 1)),
        true,
        CURRENT_DATE + (gs || ' days')::interval,
        CURRENT_DATE - ((20 - gs) || ' days')::interval,
        'Recurrente seed ampliado ' || gs::text,
        'seed',
        now(),
        now(),
        gs IN (4, 7),
        ARRAY['demo', 'recurring'],
        NULL
    FROM generate_series(3, 10) AS gs
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
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            updated_at = now(),
            deleted_at = NULL;

    -- Procurement requests 3..10 + lineas.
    INSERT INTO procurement_requests (
        id, org_id, requester_actor, title, description, category, status,
        estimated_total, currency, evaluation_json, purchase_id, created_at,
        updated_at, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/procurement/' || gs::text),
        v_tenant,
        'seed',
        'Solicitud de compra demo ' || gs::text,
        'Pedido de reposicion y operaciones ampliado ' || gs::text,
        CASE WHEN gs % 2 = 0 THEN 'inventory' ELSE 'operations' END,
        (ARRAY['draft','pending_approval','approved','rejected'])[(gs % 4) + 1],
        12000 + gs * 2600,
        'ARS',
        jsonb_build_object('decision', CASE WHEN gs % 2 = 0 THEN 'require_approval' ELSE 'allow' END, 'source', 'seed-bulk'),
        CASE WHEN gs IN (4, 8) THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase/' || gs::text) ELSE NULL END,
        now() - make_interval(days => 14 - gs),
        now(),
        NULL
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET title = EXCLUDED.title,
            description = EXCLUDED.description,
            category = EXCLUDED.category,
            status = EXCLUDED.status,
            estimated_total = EXCLUDED.estimated_total,
            evaluation_json = EXCLUDED.evaluation_json,
            purchase_id = EXCLUDED.purchase_id,
            updated_at = now(),
            deleted_at = NULL;

    INSERT INTO procurement_request_lines (id, request_id, description, product_id, quantity, unit_price_estimate, sort_order)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/procurement-line/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/procurement/' || gs::text),
        'Linea procurement demo ' || gs::text,
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || gs::text) ELSE NULL END,
        1 + (gs % 3),
        5000 + gs * 900,
        1
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET description = EXCLUDED.description,
            product_id = EXCLUDED.product_id,
            quantity = EXCLUDED.quantity,
            unit_price_estimate = EXCLUDED.unit_price_estimate,
            sort_order = EXCLUDED.sort_order;

    -- Facturas 6..10 + linea.
    INSERT INTO invoices (
        id, org_id, number, party_id, customer_name, issued_date, due_date,
        status, subtotal, discount_percent, tax_percent, total, notes,
        is_favorite, tags, created_by, created_at, updated_at, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/invoice/' || gs::text),
        v_tenant,
        'INV-' || (4000 + gs)::text,
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text),
        (SELECT display_name FROM parties WHERE id = uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)),
        CURRENT_DATE - (20 - gs),
        CURRENT_DATE - (10 - gs),
        (ARRAY['paid','pending','overdue'])[(gs % 3) + 1],
        50000 + gs * 8500,
        CASE WHEN gs % 2 = 0 THEN 5 ELSE 0 END,
        21,
        round((50000 + gs * 8500) * (1 - CASE WHEN gs % 2 = 0 THEN 0.05 ELSE 0 END) * 1.21, 2),
        'Factura seed ampliada ' || gs::text,
        gs IN (6, 9),
        ARRAY['demo', 'invoice'],
        'seed',
        now() - make_interval(days => 20 - gs),
        now(),
        NULL
    FROM generate_series(6, 10) AS gs
    ON CONFLICT (org_id, number) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            customer_name = EXCLUDED.customer_name,
            issued_date = EXCLUDED.issued_date,
            due_date = EXCLUDED.due_date,
            status = EXCLUDED.status,
            subtotal = EXCLUDED.subtotal,
            discount_percent = EXCLUDED.discount_percent,
            tax_percent = EXCLUDED.tax_percent,
            total = EXCLUDED.total,
            notes = EXCLUDED.notes,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            updated_at = now(),
            deleted_at = NULL;

    DELETE FROM invoice_line_items
     WHERE invoice_id IN (
        SELECT uuid_generate_v5(v_tenant, 'pymes-seed/v2/invoice/' || gs::text)
        FROM generate_series(6, 10) AS gs
     );

    INSERT INTO invoice_line_items (id, invoice_id, description, qty, unit, unit_price, line_total, sort_order)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/invoice-line/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/invoice/' || gs::text),
        'Concepto factura demo ' || gs::text,
        1 + (gs % 4),
        CASE WHEN gs % 2 = 0 THEN 'servicio' ELSE 'unidad' END,
        12000 + gs * 1500,
        (1 + (gs % 4)) * (12000 + gs * 1500),
        1
    FROM generate_series(6, 10) AS gs;

    -- Empleados 4..10.
    INSERT INTO employees (
        id, org_id, first_name, last_name, email, phone, position, status,
        hire_date, notes, is_favorite, tags, created_by, created_at,
        updated_at, deleted_at, metadata
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/' || gs::text),
        v_tenant,
        (ARRAY['Diego','Sofia','Martin','Valentina','Nicolas','Camila','Agustin'])[(gs - 4) + 1],
        (ARRAY['Lopez','Suarez','Pereyra','Acosta','Herrera','Molina','Castro'])[(gs - 4) + 1],
        'empleado' || gs::text || '@demo.pymes',
        '+54 9 11 3000 ' || lpad(gs::text, 4, '0'),
        (ARRAY['Ventas','Administracion','Deposito','Soporte','Operaciones','Atencion','Compras'])[(gs - 4) + 1],
        CASE WHEN gs = 10 THEN 'inactive' ELSE 'active' END,
        DATE '2023-01-01' + (gs * 20),
        'Empleado seed ampliado ' || gs::text,
        gs IN (4, 8),
        ARRAY['demo', 'employee'],
        'seed',
        now(),
        now(),
        NULL,
        jsonb_build_object('source', 'seed-bulk')
    FROM generate_series(4, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            position = EXCLUDED.position,
            status = EXCLUDED.status,
            hire_date = EXCLUDED.hire_date,
            notes = EXCLUDED.notes,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            updated_at = now(),
            deleted_at = NULL,
            metadata = EXCLUDED.metadata;

    -- La pantalla /employees lista parties con rol employee; espejamos los 10 employees ahi tambien.
    INSERT INTO parties (
        id, org_id, party_type, display_name, email, phone, address,
        tax_id, notes, is_favorite, tags, metadata, created_at, updated_at, deleted_at
    )
    SELECT
        e.id,
        e.org_id,
        'person',
        trim(e.first_name || ' ' || e.last_name),
        e.email,
        e.phone,
        '{}'::jsonb,
        NULL,
        coalesce(NULLIF(e.notes, ''), 'seed employee'),
        e.is_favorite,
        ARRAY(SELECT DISTINCT tag FROM unnest(coalesce(e.tags, ARRAY[]::text[]) || ARRAY['demo', 'employee']) AS tags(tag)),
        jsonb_build_object('source', 'seed-bulk', 'employee_id', e.id, 'position', e.position, 'status', e.status),
        now(),
        now(),
        NULL
    FROM employees e
    WHERE e.org_id = v_tenant
      AND e.id IN (
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/1'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/2'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/3'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/4'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/5'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/6'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/7'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/8'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/9'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/10')
      )
    ON CONFLICT (id) DO UPDATE
        SET party_type = EXCLUDED.party_type,
            display_name = EXCLUDED.display_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            notes = EXCLUDED.notes,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now(),
            deleted_at = NULL;

    INSERT INTO party_persons (party_id, first_name, last_name)
    SELECT e.id, e.first_name, e.last_name
    FROM employees e
    WHERE e.org_id = v_tenant
      AND e.id IN (
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/1'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/2'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/3'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/4'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/5'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/6'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/7'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/8'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/9'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/10')
      )
    ON CONFLICT (party_id) DO UPDATE
        SET first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee-party-role/' || e.id::text),
        e.id,
        e.org_id,
        'employee',
        true,
        NULL::uuid,
        jsonb_build_object('source', 'seed-bulk', 'employee_id', e.id, 'status', e.status),
        now()
    FROM employees e
    WHERE e.org_id = v_tenant
      AND e.id IN (
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/1'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/2'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v1/employee/3'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/4'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/5'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/6'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/7'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/8'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/9'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/employee/10')
      )
    ON CONFLICT (party_id, org_id, role) DO UPDATE
        SET is_active = true,
            metadata = EXCLUDED.metadata;

    -- Devoluciones y notas de credito 2..10.
    INSERT INTO returns (
        id, org_id, number, sale_id, reason, subtotal, tax_total, total,
        refund_method, status, notes, created_by, created_at, is_favorite,
        tags, deleted_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/return/' || gs::text),
        v_tenant,
        'DEV-' || lpad(gs::text, 5, '0'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || GREATEST(gs, 4)::text),
        (ARRAY['defective','wrong_item','changed_mind','other'])[(gs % 4) + 1],
        5000 + gs * 900,
        round((5000 + gs * 900) * 0.21, 2),
        round((5000 + gs * 900) * 1.21, 2),
        CASE WHEN gs % 2 = 0 THEN 'credit_note' ELSE 'original_method' END,
        'completed',
        'Devolucion seed ampliada ' || gs::text,
        'seed',
        now() - make_interval(days => 10 - gs),
        gs IN (3, 8),
        ARRAY['demo', 'return'],
        NULL
    FROM generate_series(2, 10) AS gs
    ON CONFLICT (org_id, number) DO UPDATE
        SET sale_id = EXCLUDED.sale_id,
            reason = EXCLUDED.reason,
            subtotal = EXCLUDED.subtotal,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            refund_method = EXCLUDED.refund_method,
            status = EXCLUDED.status,
            notes = EXCLUDED.notes,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            deleted_at = NULL;

    INSERT INTO return_items (id, return_id, sale_item_id, product_id, description, quantity, unit_price, tax_rate, subtotal)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/return-item/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/return/' || gs::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale-item/' || GREATEST(gs, 4)::text),
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/product/' || GREATEST(gs, 4)::text) ELSE NULL END,
        'Item devolucion seed ' || gs::text,
        1,
        5000 + gs * 900,
        21,
        5000 + gs * 900
    FROM generate_series(2, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET description = EXCLUDED.description,
            quantity = EXCLUDED.quantity,
            unit_price = EXCLUDED.unit_price,
            tax_rate = EXCLUDED.tax_rate,
            subtotal = EXCLUDED.subtotal;

    INSERT INTO credit_notes (
        id, org_id, number, party_id, return_id, amount, used_amount,
        balance, expires_at, status, created_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/credit-note/' || gs::text),
        v_tenant,
        'NC-' || lpad(gs::text, 5, '0'),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || GREATEST(gs, 4)::text),
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/return/' || gs::text),
        round((5000 + gs * 900) * 1.21, 2),
        CASE WHEN gs % 3 = 0 THEN 1000 ELSE 0 END,
        round((5000 + gs * 900) * 1.21, 2) - CASE WHEN gs % 3 = 0 THEN 1000 ELSE 0 END,
        now() + make_interval(days => 60 + gs),
        CASE WHEN gs % 3 = 0 THEN 'used' ELSE 'active' END,
        now() - make_interval(days => 10 - gs)
    FROM generate_series(2, 10) AS gs
    ON CONFLICT (org_id, number) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            return_id = EXCLUDED.return_id,
            amount = EXCLUDED.amount,
            used_amount = EXCLUDED.used_amount,
            balance = EXCLUDED.balance,
            expires_at = EXCLUDED.expires_at,
            status = EXCLUDED.status;

    -- Notificaciones, timeline y webhooks hasta 10.
    INSERT INTO pymes_in_app_notifications (
        id, org_id, user_id, title, body, kind, entity_type, entity_id,
        chat_context, read_at, created_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/in-app-notif/' || gs::text),
        v_tenant,
        local_user,
        'Notificacion demo ' || gs::text,
        'Evento demo para probar notificaciones y handoff del asistente ' || gs::text,
        CASE WHEN gs % 2 = 0 THEN 'insight' ELSE 'system' END,
        CASE WHEN gs % 2 = 0 THEN 'sale' ELSE 'org' END,
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text)::text ELSE v_tenant::text END,
        jsonb_build_object('scope', CASE WHEN gs % 2 = 0 THEN 'sales_collections' ELSE 'general' END),
        CASE WHEN gs IN (4, 7) THEN now() - make_interval(hours => gs) ELSE NULL END,
        now() - make_interval(hours => gs)
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET title = EXCLUDED.title,
            body = EXCLUDED.body,
            kind = EXCLUDED.kind,
            entity_type = EXCLUDED.entity_type,
            entity_id = EXCLUDED.entity_id,
            chat_context = EXCLUDED.chat_context,
            read_at = EXCLUDED.read_at,
            created_at = EXCLUDED.created_at;

    INSERT INTO timeline_entries (
        id, org_id, entity_type, entity_id, event_type, title,
        description, actor, metadata, created_at
    )
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/timeline/' || gs::text),
        v_tenant,
        CASE WHEN gs % 2 = 0 THEN 'sales' ELSE 'purchases' END,
        CASE WHEN gs % 2 = 0 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v2/sale/' || gs::text) ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/purchase/' || gs::text) END,
        'note',
        'Actividad demo ' || gs::text,
        'Entrada de timeline seed ampliada ' || gs::text,
        'seed',
        jsonb_build_object('source', 'seed-bulk'),
        now() - make_interval(days => 12 - gs)
    FROM generate_series(3, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET entity_type = EXCLUDED.entity_type,
            entity_id = EXCLUDED.entity_id,
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            metadata = EXCLUDED.metadata,
            created_at = EXCLUDED.created_at;

    INSERT INTO webhook_endpoints (id, org_id, url, secret, events, is_active, created_by, created_at, updated_at)
    SELECT
        uuid_generate_v5(v_tenant, 'pymes-seed/v2/webhook/' || gs::text),
        v_tenant,
        'https://example.invalid/hooks/pymes/' || gs::text,
        'seed-secret-' || gs::text,
        ARRAY['sale.created', 'customer.updated', CASE WHEN gs % 2 = 0 THEN 'purchase.received' ELSE 'invoice.created' END],
        gs <> 10,
        'seed',
        now(),
        now()
    FROM generate_series(2, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET url = EXCLUDED.url,
            secret = EXCLUDED.secret,
            events = EXCLUDED.events,
            is_active = EXCLUDED.is_active,
            updated_at = now();

    -- Agenda: 10 turnos por sucursal activa para que cualquier filtro tenga datos.
    IF v_sched_service IS NOT NULL THEN
        INSERT INTO scheduling_resources (id, org_id, branch_id, code, name, kind, capacity, timezone, active)
        SELECT
            uuid_generate_v5(v_tenant, 'modules-scheduling/v2/resource/' || br.code || '/demo'),
            v_tenant,
            br.id,
            'demo_' || br.code,
            'Recurso Demo ' || br.name,
            'professional',
            1,
            br.timezone,
            true
        FROM scheduling_branches br
        WHERE br.org_id = v_tenant
          AND br.active = true
          AND NOT EXISTS (
              SELECT 1
              FROM scheduling_resources existing
              WHERE existing.org_id = v_tenant
                AND existing.branch_id = br.id
                AND existing.active = true
          )
        ON CONFLICT (id) DO UPDATE
            SET branch_id = EXCLUDED.branch_id,
                name = EXCLUDED.name,
                timezone = EXCLUDED.timezone,
                active = EXCLUDED.active,
                updated_at = now();

        INSERT INTO scheduling_service_resources (service_id, resource_id)
        SELECT v_sched_service, r.id
        FROM scheduling_resources r
        WHERE r.org_id = v_tenant
          AND r.active = true
          AND EXISTS (
              SELECT 1
              FROM scheduling_branches br
              WHERE br.id = r.branch_id
                AND br.org_id = v_tenant
                AND br.active = true
          )
        ON CONFLICT (service_id, resource_id) DO NOTHING;

        DELETE FROM scheduling_bookings
         WHERE org_id = v_tenant
           AND created_by = 'seed';

        INSERT INTO scheduling_bookings (
            id, org_id, branch_id, service_id, resource_id, party_id, reference,
            customer_name, customer_phone, customer_email, status, source,
            start_at, end_at, occupies_from, occupies_until, notes, metadata,
            created_by, confirmed_at, created_at, updated_at
        )
        SELECT
            uuid_generate_v5(v_tenant, 'modules-scheduling/v2/booking/' || br.code || '/demo-' || gs::text),
            v_tenant,
            br.id,
            v_sched_service,
            r.id,
            CASE
                WHEN gs <= 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/customer/' || gs::text)
                ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)
            END,
            'DEMO-' || upper(br.code) || '-' || lpad(gs::text, 3, '0'),
            (
                SELECT display_name
                FROM parties
                WHERE id = CASE
                    WHEN gs <= 3 THEN uuid_generate_v5(v_tenant, 'pymes-seed/v1/customer/' || gs::text)
                    ELSE uuid_generate_v5(v_tenant, 'pymes-seed/v2/customer/' || gs::text)
                END
            ),
            '+54911' || lpad((50000000 + (row_number() OVER (ORDER BY br.code, gs)))::text, 8, '0'),
            'agenda-' || br.code || '-' || gs::text || '@demo.pymes',
            CASE WHEN gs IN (7, 10) THEN 'pending_confirmation' ELSE 'confirmed' END,
            'admin',
            ((CURRENT_DATE + ((gs - 1) / 2)) + make_time(9 + (gs % 8), CASE WHEN gs % 2 = 0 THEN 0 ELSE 30 END, 0)) AT TIME ZONE COALESCE(NULLIF(br.timezone, ''), 'America/Argentina/Tucuman'),
            ((CURRENT_DATE + ((gs - 1) / 2)) + make_time(9 + (gs % 8), CASE WHEN gs % 2 = 0 THEN 30 ELSE 0 END, 0) + CASE WHEN gs % 2 = 0 THEN interval '0 minutes' ELSE interval '1 hour' END) AT TIME ZONE COALESCE(NULLIF(br.timezone, ''), 'America/Argentina/Tucuman'),
            ((CURRENT_DATE + ((gs - 1) / 2)) + make_time(9 + (gs % 8), CASE WHEN gs % 2 = 0 THEN 0 ELSE 30 END, 0)) AT TIME ZONE COALESCE(NULLIF(br.timezone, ''), 'America/Argentina/Tucuman'),
            ((CURRENT_DATE + ((gs - 1) / 2)) + make_time(9 + (gs % 8), CASE WHEN gs % 2 = 0 THEN 30 ELSE 0 END, 0) + CASE WHEN gs % 2 = 0 THEN interval '0 minutes' ELSE interval '1 hour' END) AT TIME ZONE COALESCE(NULLIF(br.timezone, ''), 'America/Argentina/Tucuman'),
            'Turno demo ' || br.name || ' #' || gs::text,
            jsonb_build_object('source', 'seed-bulk', 'branch_code', br.code),
            'seed',
            CASE WHEN gs IN (7, 10) THEN NULL ELSE now() END,
            now(),
            now()
        FROM scheduling_branches br
        JOIN LATERAL (
            SELECT r.id
            FROM scheduling_resources r
            WHERE r.org_id = v_tenant
              AND r.branch_id = br.id
              AND r.active = true
            ORDER BY r.code
            LIMIT 1
        ) r ON true
        CROSS JOIN generate_series(1, 10) AS gs
        WHERE br.org_id = v_tenant
          AND br.active = true
        ON CONFLICT (id) DO UPDATE
            SET party_id = EXCLUDED.party_id,
                branch_id = EXCLUDED.branch_id,
                service_id = EXCLUDED.service_id,
                resource_id = EXCLUDED.resource_id,
                reference = EXCLUDED.reference,
                customer_name = EXCLUDED.customer_name,
                customer_phone = EXCLUDED.customer_phone,
                customer_email = EXCLUDED.customer_email,
                status = EXCLUDED.status,
                start_at = EXCLUDED.start_at,
                end_at = EXCLUDED.end_at,
                occupies_from = EXCLUDED.occupies_from,
                occupies_until = EXCLUDED.occupies_until,
                notes = EXCLUDED.notes,
                metadata = EXCLUDED.metadata,
                confirmed_at = EXCLUDED.confirmed_at,
                updated_at = now();
    END IF;

    UPDATE tenant_settings
       SET next_quote_number = GREATEST(next_quote_number, 11),
           next_sale_number = GREATEST(next_sale_number, 11),
           updated_at = now()
     WHERE org_id = v_tenant;
END $$;
