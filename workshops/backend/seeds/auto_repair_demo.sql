-- Demo taller auto_repair: 10 vehiculos y 10 ordenes.
-- Cliente/producto: mismas claves uuid v5 que pymes-core/seeds.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    p1 uuid;
    srv1 uuid;
    srv2 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    p1 := uuid_generate_v5(v_org, 'pymes-seed/v1/product/1');
    srv1 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/service/oil');
    srv2 := uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/service/brake');

    INSERT INTO services (
        id, org_id, code, name, description, category_code,
        sale_price, cost_price, tax_rate, currency,
        default_duration_minutes, is_active, tags, metadata
    )
    VALUES
        (srv1, v_org, 'SRV-OIL', 'Cambio de aceite y filtro', 'Servicio estandar', 'mantenimiento', 25000, 0, 21, 'ARS', 30, true, ARRAY['demo', 'workshops'], jsonb_build_object('vertical', 'workshops', 'segment', 'auto_repair')),
        (srv2, v_org, 'SRV-BRAKE', 'Revision de frenos', 'Inspeccion y ajuste', 'frenos', 45000, 0, 21, 'ARS', 90, true, ARRAY['demo', 'workshops'], jsonb_build_object('vertical', 'workshops', 'segment', 'auto_repair'))
    ON CONFLICT (org_id, code) WHERE deleted_at IS NULL AND code IS NOT NULL AND code <> '' DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            category_code = EXCLUDED.category_code,
            sale_price = EXCLUDED.sale_price,
            cost_price = EXCLUDED.cost_price,
            currency = EXCLUDED.currency,
            tax_rate = EXCLUDED.tax_rate,
            default_duration_minutes = EXCLUDED.default_duration_minutes,
            is_active = EXCLUDED.is_active,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now();

    INSERT INTO workshops.customer_assets (
        id, org_id, asset_type, customer_id, customer_name, label, brand, model, serial_number, year,
        color, notes, metadata, is_favorite, tags, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/vehicle/' || gs::text),
        v_org,
        'vehicle',
        uuid_generate_v5(v_org, 'pymes-seed/v1/customer/' || (((gs - 1) % 10) + 1)::text),
        (ARRAY[
            'Cliente Demo Uno', 'Mercado Plaza', 'Panaderia La Esquina', 'Distribuidora Norte',
            'Almacen Don Luis', 'Ferreteria Central', 'Kiosco Avenida', 'Libreria Sur',
            'Gimnasio Activo', 'Cafe Martinez Demo'
        ])[gs],
        (ARRAY['AB 123 CD', 'AC 234 EF', 'AD 345 GH', 'AE 456 IJ', 'AF 567 KL', 'AG 678 MN', 'AH 789 OP', 'AI 890 QR', 'AJ 901 ST', 'AK 012 UV'])[gs],
        (ARRAY['Ford', 'Toyota', 'Volkswagen', 'Chevrolet', 'Renault', 'Peugeot', 'Fiat', 'Nissan', 'Citroen', 'Honda'])[gs],
        (ARRAY['Focus', 'Corolla', 'Gol Trend', 'Cruze', 'Kangoo', 'Partner', 'Cronos', 'Frontier', 'C4 Lounge', 'Fit'])[gs],
        'VINSEEDAUTO' || lpad(gs::text, 6, '0'),
        (ARRAY[2018, 2020, 2017, 2021, 2019, 2016, 2022, 2018, 2017, 2020])[gs],
        (ARRAY['Gris', 'Blanco', 'Rojo', 'Azul', 'Negro', 'Plata', 'Bordo', 'Verde', 'Grafito', 'Celeste'])[gs],
        'Vehiculo seed taller ' || gs::text,
        jsonb_build_object(
            'license_plate', (ARRAY['AB 123 CD', 'AC 234 EF', 'AD 345 GH', 'AE 456 IJ', 'AF 567 KL', 'AG 678 MN', 'AH 789 OP', 'AI 890 QR', 'AJ 901 ST', 'AK 012 UV'])[gs],
            'vin', 'VINSEEDAUTO' || lpad(gs::text, 6, '0'),
            'kilometers', (ARRAY[98500, 45200, 120400, 38000, 76500, 141000, 22000, 88200, 101300, 53400])[gs]
        ),
        gs IN (2, 8),
        ARRAY['seed', 'auto_repair'],
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET asset_type = EXCLUDED.asset_type,
            customer_id = EXCLUDED.customer_id,
            customer_name = EXCLUDED.customer_name,
            label = EXCLUDED.label,
            brand = EXCLUDED.brand,
            model = EXCLUDED.model,
            serial_number = EXCLUDED.serial_number,
            year = EXCLUDED.year,
            color = EXCLUDED.color,
            notes = EXCLUDED.notes,
            metadata = EXCLUDED.metadata,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            archived_at = NULL,
            updated_at = now();

    INSERT INTO workshops.work_orders (
        id, org_id, number, asset_type, asset_id, asset_label, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency, metadata,
        subtotal_services, subtotal_parts, tax_total, total, opened_at, promised_at, ready_at, delivered_at,
        is_favorite, tags, created_by, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/' || gs::text),
        v_org,
        'OT-SEED-' || lpad(gs::text, 3, '0'),
        'vehicle',
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/vehicle/' || gs::text),
        (ARRAY['AB 123 CD', 'AC 234 EF', 'AD 345 GH', 'AE 456 IJ', 'AF 567 KL', 'AG 678 MN', 'AH 789 OP', 'AI 890 QR', 'AJ 901 ST', 'AK 012 UV'])[gs],
        uuid_generate_v5(v_org, 'pymes-seed/v1/customer/' || (((gs - 1) % 10) + 1)::text),
        (ARRAY[
            'Cliente Demo Uno', 'Mercado Plaza', 'Panaderia La Esquina', 'Distribuidora Norte',
            'Almacen Don Luis', 'Ferreteria Central', 'Kiosco Avenida', 'Libreria Sur',
            'Gimnasio Activo', 'Cafe Martinez Demo'
        ])[gs],
        (ARRAY['received', 'diagnosing', 'quote_pending', 'awaiting_parts', 'in_progress', 'quality_check', 'ready_for_pickup', 'delivered', 'on_hold', 'in_progress'])[gs],
        (ARRAY[
            'Cambio de aceite y ruido al frenar',
            'Service 20.000 km',
            'Revision por testigo de motor',
            'Cambio de pastillas y discos',
            'Alineacion y balanceo',
            'Diagnostico electrico',
            'Control pre-viaje',
            'Entrega post service',
            'Falla intermitente en arranque',
            'Cambio de correa auxiliar'
        ])[gs],
        (ARRAY[
            'Filtro y aceite vencidos',
            'Pastillas delanteras al limite',
            'Sensor pendiente de escaneo',
            'Discos marcados',
            'Cubiertas con desgaste irregular',
            'Bateria con baja carga',
            'Frenos y fluidos revisados',
            'Trabajo finalizado',
            'Burro de arranque requiere control',
            'Correa con fisuras visibles'
        ])[gs],
        'Orden de auto seed ' || gs::text,
        CASE WHEN gs IN (3, 6, 9) THEN 'Requiere llamar al cliente antes de avanzar' ELSE '' END,
        'ARS',
        jsonb_build_object('vertical', 'workshops', 'segment', 'auto_repair', 'source', 'seed'),
        (20000 + gs * 5000)::double precision,
        (10000 + gs * 3500)::double precision,
        round(((30000 + gs * 8500) * 0.21)::numeric, 2)::double precision,
        round(((30000 + gs * 8500) * 1.21)::numeric, 2)::double precision,
        now() - ((12 - gs) || ' days')::interval,
        now() + ((gs + 2) || ' days')::interval,
        CASE WHEN gs IN (7, 8) THEN now() - '1 day'::interval ELSE NULL END,
        CASE WHEN gs = 8 THEN now() ELSE NULL END,
        gs IN (3, 6),
        ARRAY['seed', 'auto_repair'],
        'seed',
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (org_id, number) WHERE archived_at IS NULL DO UPDATE
        SET asset_type = EXCLUDED.asset_type,
            asset_id = EXCLUDED.asset_id,
            asset_label = EXCLUDED.asset_label,
            customer_id = EXCLUDED.customer_id,
            customer_name = EXCLUDED.customer_name,
            status = EXCLUDED.status,
            requested_work = EXCLUDED.requested_work,
            diagnosis = EXCLUDED.diagnosis,
            notes = EXCLUDED.notes,
            internal_notes = EXCLUDED.internal_notes,
            currency = EXCLUDED.currency,
            metadata = EXCLUDED.metadata,
            subtotal_services = EXCLUDED.subtotal_services,
            subtotal_parts = EXCLUDED.subtotal_parts,
            tax_total = EXCLUDED.tax_total,
            total = EXCLUDED.total,
            opened_at = EXCLUDED.opened_at,
            promised_at = EXCLUDED.promised_at,
            ready_at = EXCLUDED.ready_at,
            delivered_at = EXCLUDED.delivered_at,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            created_by = EXCLUDED.created_by,
            updated_at = now();

    DELETE FROM workshops.work_order_items
    WHERE org_id = v_org
      AND work_order_id IN (
        SELECT uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/' || gs::text)
        FROM generate_series(1, 10) AS gs
      );

    INSERT INTO workshops.work_order_items (
        id, org_id, work_order_id, item_type, service_id, product_id,
        description, quantity, unit_price, tax_rate, sort_order, metadata
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/' || gs::text || '/service'),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/' || gs::text),
        'service',
        CASE WHEN gs % 2 = 0 THEN srv2 ELSE srv1 END,
        NULL,
        CASE WHEN gs % 2 = 0 THEN 'Revision de frenos' ELSE 'Cambio de aceite y filtro' END,
        1,
        (20000 + gs * 5000)::double precision,
        21,
        1,
        '{}'::jsonb
    FROM generate_series(1, 10) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/' || gs::text || '/part'),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/' || gs::text),
        'part',
        NULL,
        p1,
        (ARRAY[
            'Filtro de aceite', 'Liquido de frenos', 'Sensor generico', 'Pastillas delanteras',
            'Valvula de cubierta', 'Terminal bateria', 'Kit fluidos', 'Insumos entrega',
            'Relay arranque', 'Correa auxiliar'
        ])[gs],
        1,
        (10000 + gs * 3500)::double precision,
        21,
        2,
        '{}'::jsonb
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO NOTHING;
END $$;
