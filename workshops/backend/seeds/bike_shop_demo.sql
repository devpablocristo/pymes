-- Demo taller bike_shop: 10 bicicletas y 10 OT.
-- Depende de los clientes de pymes-core/seeds.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tenants WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO workshops.customer_assets (
        id, tenant_id, asset_type, customer_id, customer_name, label, brand, model, serial_number,
        color, notes, metadata, is_favorite, tags, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/bicycle/' || gs::text),
        v_org,
        'bicycle',
        uuid_generate_v5(v_org, 'pymes-seed/v1/customer/' || (((gs - 1) % 10) + 1)::text),
        (ARRAY[
            'Cliente Demo Uno', 'Mercado Plaza', 'Panaderia La Esquina', 'Distribuidora Norte',
            'Almacen Don Luis', 'Ferreteria Central', 'Kiosco Avenida', 'Libreria Sur',
            'Gimnasio Activo', 'Cafe Martinez Demo'
        ])[gs],
        (ARRAY['Trek', 'Specialized', 'Giant', 'Scott', 'Cannondale', 'Merida', 'Venzo', 'Raleigh', 'Bianchi', 'Vairo'])[gs] || ' ' ||
            (ARRAY['Marlin 7', 'Rockhopper', 'Talon 2', 'Aspect 940', 'Trail 6', 'Big Nine', 'Raptor', 'Mojave', 'Impulso', 'XR 4.0'])[gs],
        (ARRAY['Trek', 'Specialized', 'Giant', 'Scott', 'Cannondale', 'Merida', 'Venzo', 'Raleigh', 'Bianchi', 'Vairo'])[gs],
        (ARRAY['Marlin 7', 'Rockhopper', 'Talon 2', 'Aspect 940', 'Trail 6', 'Big Nine', 'Raptor', 'Mojave', 'Impulso', 'XR 4.0'])[gs],
        'BIKE-DEMO-' || lpad(gs::text, 3, '0'),
        (ARRAY['Negro', 'Rojo', 'Azul', 'Gris', 'Verde', 'Blanco', 'Amarillo', 'Naranja', 'Celeste', 'Grafito'])[gs],
        'Bicicleta seed ' || gs::text,
        jsonb_build_object(
            'frame_number', 'BIKE-DEMO-' || lpad(gs::text, 3, '0'),
            'bike_type', (ARRAY['mountain', 'mountain', 'urban', 'road', 'mountain', 'gravel', 'urban', 'kids', 'road', 'mountain'])[gs],
            'size', (ARRAY['M', 'L', 'S', 'M', 'L', 'M', 'S', 'XS', 'L', 'M'])[gs],
            'wheel_size_inches', (ARRAY[29, 29, 28, 28, 27, 28, 26, 24, 28, 29])[gs],
            'ebike_notes', ''
        ),
        gs IN (4, 7),
        ARRAY['seed', 'bike_shop'],
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
            color = EXCLUDED.color,
            notes = EXCLUDED.notes,
            metadata = EXCLUDED.metadata,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            archived_at = NULL,
            updated_at = now();

    INSERT INTO workshops.work_orders (
        id, tenant_id, number, asset_type, asset_id, asset_label, customer_id, customer_name, status,
        requested_work, diagnosis, notes, internal_notes, currency, metadata,
        subtotal_services, subtotal_parts, tax_total, total, opened_at, promised_at, ready_at, delivered_at,
        is_favorite, tags, created_by, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/bike/' || gs::text),
        v_org,
        'OT-BIKE-' || lpad(gs::text, 3, '0'),
        'bicycle',
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/bicycle/' || gs::text),
        (ARRAY['Trek Marlin 7', 'Specialized Rockhopper', 'Giant Talon 2', 'Scott Aspect 940', 'Cannondale Trail 6', 'Merida Big Nine', 'Venzo Raptor', 'Raleigh Mojave', 'Bianchi Impulso', 'Vairo XR 4.0'])[gs],
        uuid_generate_v5(v_org, 'pymes-seed/v1/customer/' || (((gs - 1) % 10) + 1)::text),
        (ARRAY[
            'Cliente Demo Uno', 'Mercado Plaza', 'Panaderia La Esquina', 'Distribuidora Norte',
            'Almacen Don Luis', 'Ferreteria Central', 'Kiosco Avenida', 'Libreria Sur',
            'Gimnasio Activo', 'Cafe Martinez Demo'
        ])[gs],
        (ARRAY['received', 'diagnosing', 'quote_pending', 'awaiting_parts', 'in_progress', 'quality_check', 'ready_for_pickup', 'delivered', 'on_hold', 'in_progress'])[gs],
        (ARRAY[
            'Servicio general y ajuste de cambios',
            'Revision de frenos hidraulicos',
            'Centrado de rueda trasera',
            'Cambio de transmision',
            'Service completo previo a viaje',
            'Control de horquilla y direccion',
            'Reparacion de pinchazo y cubierta',
            'Ajuste infantil y frenos',
            'Revision ruta y calibracion',
            'Diagnostico por ruido en caja'
        ])[gs],
        (ARRAY[
            'Cadena con desgaste medio',
            'Pastillas con poco material',
            'Rayos flojos en rueda trasera',
            'Pinon y plato con desgaste visible',
            'Cubiertas y frenos requieren control',
            'Juego leve en direccion',
            'Camara pinchada',
            'Zapatas desalineadas',
            'Cubiertas con presion baja',
            'Caja pedalera con ruido'
        ])[gs],
        'Orden de bicicleta seed ' || gs::text,
        CASE WHEN gs IN (4, 7) THEN 'Prioridad alta por uso diario' ELSE '' END,
        'ARS',
        jsonb_build_object('vertical', 'workshops', 'segment', 'bike_shop', 'source', 'seed'),
        (12000 + gs * 2500)::double precision,
        (6000 + gs * 1800)::double precision,
        round(((18000 + gs * 4300) * 0.21)::numeric, 2)::double precision,
        round(((18000 + gs * 4300) * 1.21)::numeric, 2)::double precision,
        now() - ((10 - gs) || ' days')::interval,
        now() + ((gs + 1) || ' days')::interval,
        CASE WHEN gs IN (7, 8) THEN now() - '1 day'::interval ELSE NULL END,
        CASE WHEN gs = 8 THEN now() ELSE NULL END,
        gs IN (4, 7),
        ARRAY['seed', 'bike_shop'],
        'seed',
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (tenant_id, number) WHERE archived_at IS NULL DO UPDATE
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
    WHERE tenant_id = v_org
      AND work_order_id IN (
        SELECT uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/bike/' || gs::text)
        FROM generate_series(1, 10) AS gs
      );

    INSERT INTO workshops.work_order_items (
        id, tenant_id, work_order_id, item_type, service_id, product_id,
        description, quantity, unit_price, tax_rate, sort_order, metadata
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/bike/' || gs::text || '/service'),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/bike/' || gs::text),
        'service',
        NULL::uuid,
        NULL::uuid,
        (ARRAY[
            'Servicio general bicicleta', 'Revision de frenos', 'Centrado de rueda',
            'Cambio de transmision', 'Service completo', 'Control de direccion',
            'Reparacion de pinchazo', 'Ajuste infantil', 'Revision ruta', 'Diagnostico caja pedalera'
        ])[gs],
        1,
        (12000 + gs * 2500)::double precision,
        21,
        1,
        '{}'::jsonb
    FROM generate_series(1, 10) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/woi/bike/' || gs::text || '/part'),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/workshop/wo/bike/' || gs::text),
        'part',
        NULL::uuid,
        NULL::uuid,
        (ARRAY[
            'Camara 29x2.1', 'Pastillas de freno', 'Rayo inoxidable',
            'Cadena 10v', 'Cable y funda', 'Juego de direccion',
            'Cubierta urbana', 'Zapatas de freno', 'Cinta manubrio', 'Caja pedalera'
        ])[gs],
        1,
        (6000 + gs * 1800)::double precision,
        21,
        2,
        '{}'::jsonb
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO NOTHING;
END $$;
