-- 0013: Unificar work_orders auto_repair + bike_shop en una sola tabla con polimorfismo target_type/target_id.
--
-- Estrategia: crear tablas nuevas con sufijo _v2 y copiar datos. Las tablas viejas
-- (workshops.work_orders, workshops.bike_work_orders) NO se tocan en esta migración.
-- Una migración posterior (0014) hará el rename y drop una vez validado el flujo.

-- ─────────────────────────────────────────────────────────────────────────────
-- Tabla unificada: work_orders_v2
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workshops.work_orders_v2 (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    number TEXT NOT NULL,

    -- Polimorfismo: a qué asset apunta esta OT.
    target_type TEXT NOT NULL,           -- 'vehicle' | 'bicycle' (futuro: 'pet', 'asset', etc.)
    target_id UUID NOT NULL,             -- referencia opaca al asset (no FK; cada vertical valida)
    target_label TEXT NOT NULL DEFAULT '', -- denormalizado para listas (patente, "Trek Marlin 7", etc.)

    customer_id UUID NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    booking_id UUID NULL,
    quote_id UUID NULL,
    sale_id UUID NULL,

    status TEXT NOT NULL,
    requested_work TEXT NOT NULL DEFAULT '',
    diagnosis TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    internal_notes TEXT NOT NULL DEFAULT '',

    currency TEXT NOT NULL DEFAULT 'ARS',
    subtotal_services DOUBLE PRECISION NOT NULL DEFAULT 0,
    subtotal_parts DOUBLE PRECISION NOT NULL DEFAULT 0,
    tax_total DOUBLE PRECISION NOT NULL DEFAULT 0,
    total DOUBLE PRECISION NOT NULL DEFAULT 0,

    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    promised_at TIMESTAMPTZ NULL,
    ready_at TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,

    metadata JSONB NOT NULL DEFAULT '{}'::jsonb, -- vertical-specific (segment, custom fields)

    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ NULL
);

-- Único activo por (org, number) — mismo patrón que las tablas legacy.
CREATE UNIQUE INDEX IF NOT EXISTS workshops_work_orders_v2_org_number_active_idx
    ON workshops.work_orders_v2 (org_id, number)
    WHERE archived_at IS NULL;

-- Índices de búsqueda comunes.
CREATE INDEX IF NOT EXISTS workshops_work_orders_v2_org_target_idx
    ON workshops.work_orders_v2 (org_id, target_type)
    WHERE archived_at IS NULL;

CREATE INDEX IF NOT EXISTS workshops_work_orders_v2_org_status_idx
    ON workshops.work_orders_v2 (org_id, status)
    WHERE archived_at IS NULL;

-- ─────────────────────────────────────────────────────────────────────────────
-- Tabla unificada: work_order_items_v2
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS workshops.work_order_items_v2 (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    work_order_id UUID NOT NULL REFERENCES workshops.work_orders_v2(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,             -- 'service' | 'part'
    service_id UUID NULL,                -- → public.services
    product_id UUID NULL,                -- → public.products
    description TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL DEFAULT 1,
    unit_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    tax_rate DOUBLE PRECISION NOT NULL DEFAULT 21,
    sort_order INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS workshops_work_order_items_v2_order_idx
    ON workshops.work_order_items_v2 (work_order_id, sort_order);

-- ─────────────────────────────────────────────────────────────────────────────
-- Copia de datos: auto_repair → work_orders_v2 (target_type='vehicle')
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO workshops.work_orders_v2 (
    id, org_id, number,
    target_type, target_id, target_label,
    customer_id, customer_name,
    booking_id, quote_id, sale_id,
    status, requested_work, diagnosis, notes, internal_notes,
    currency, subtotal_services, subtotal_parts, tax_total, total,
    opened_at, promised_at, ready_at, delivered_at,
    metadata,
    created_by, created_at, updated_at, archived_at
)
SELECT
    id, org_id, number,
    'vehicle', vehicle_id, vehicle_plate,
    customer_id, customer_name,
    booking_id, quote_id, sale_id,
    status, requested_work, diagnosis, notes, internal_notes,
    currency, subtotal_services, subtotal_parts, tax_total, total,
    opened_at, promised_at, ready_at, delivered_at,
    jsonb_build_object('vertical', 'workshops', 'segment', 'auto_repair'),
    created_by, created_at, updated_at, archived_at
FROM workshops.work_orders
ON CONFLICT (id) DO NOTHING;

INSERT INTO workshops.work_order_items_v2 (
    id, org_id, work_order_id, item_type, service_id, product_id,
    description, quantity, unit_price, tax_rate, sort_order, metadata,
    created_at, updated_at
)
SELECT
    id, org_id, work_order_id, item_type, service_id, product_id,
    description, quantity, unit_price, tax_rate, sort_order, metadata,
    created_at, updated_at
FROM workshops.work_order_items
ON CONFLICT (id) DO NOTHING;

-- ─────────────────────────────────────────────────────────────────────────────
-- Copia de datos: bike_shop → work_orders_v2 (target_type='bicycle')
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO workshops.work_orders_v2 (
    id, org_id, number,
    target_type, target_id, target_label,
    customer_id, customer_name,
    booking_id, quote_id, sale_id,
    status, requested_work, diagnosis, notes, internal_notes,
    currency, subtotal_services, subtotal_parts, tax_total, total,
    opened_at, promised_at, ready_at, delivered_at,
    metadata,
    created_by, created_at, updated_at, archived_at
)
SELECT
    id, org_id, number,
    'bicycle', bicycle_id, bicycle_label,
    customer_id, customer_name,
    booking_id, quote_id, sale_id,
    status, requested_work, diagnosis, notes, internal_notes,
    currency, subtotal_services, subtotal_parts, tax_total, total,
    opened_at, promised_at, ready_at, delivered_at,
    jsonb_build_object('vertical', 'workshops', 'segment', 'bike_shop'),
    created_by, created_at, updated_at, archived_at
FROM workshops.bike_work_orders
ON CONFLICT (id) DO NOTHING;

INSERT INTO workshops.work_order_items_v2 (
    id, org_id, work_order_id, item_type, service_id, product_id,
    description, quantity, unit_price, tax_rate, sort_order, metadata,
    created_at, updated_at
)
SELECT
    id, org_id, work_order_id, item_type, service_id, product_id,
    description, quantity, unit_price, tax_rate, sort_order, metadata,
    created_at, updated_at
FROM workshops.bike_work_order_items
ON CONFLICT (id) DO NOTHING;
