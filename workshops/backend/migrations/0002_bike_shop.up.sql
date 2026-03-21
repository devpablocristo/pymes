-- Segmenta el catálogo de servicios por subdominio (auto_repair vs bike_shop).
DROP INDEX IF EXISTS workshops.workshops_services_org_code_idx;

ALTER TABLE workshops.services
    ADD COLUMN IF NOT EXISTS segment TEXT NOT NULL DEFAULT 'auto_repair';

CREATE UNIQUE INDEX IF NOT EXISTS workshops_services_org_segment_code_idx
    ON workshops.services (org_id, segment, code);

CREATE TABLE IF NOT EXISTS workshops.bicycles (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    customer_id UUID NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    frame_number TEXT NOT NULL,
    make TEXT NOT NULL,
    model TEXT NOT NULL,
    bike_type TEXT NOT NULL DEFAULT '',
    size TEXT NOT NULL DEFAULT '',
    wheel_size_inches INTEGER NOT NULL DEFAULT 0,
    color TEXT NOT NULL DEFAULT '',
    ebike_notes TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS workshops_bicycles_org_frame_idx
    ON workshops.bicycles (org_id, frame_number);

CREATE TABLE IF NOT EXISTS workshops.bike_work_orders (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    number TEXT NOT NULL,
    bicycle_id UUID NOT NULL REFERENCES workshops.bicycles(id),
    bicycle_label TEXT NOT NULL DEFAULT '',
    customer_id UUID NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    appointment_id UUID NULL,
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
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS workshops_bike_work_orders_org_number_idx
    ON workshops.bike_work_orders (org_id, number);

CREATE INDEX IF NOT EXISTS workshops_bike_work_orders_org_status_idx
    ON workshops.bike_work_orders (org_id, status);

CREATE TABLE IF NOT EXISTS workshops.bike_work_order_items (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    work_order_id UUID NOT NULL REFERENCES workshops.bike_work_orders(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,
    service_id UUID NULL,
    product_id UUID NULL,
    description TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL DEFAULT 1,
    unit_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    tax_rate DOUBLE PRECISION NOT NULL DEFAULT 21,
    sort_order INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS workshops_bike_work_order_items_order_idx
    ON workshops.bike_work_order_items (work_order_id, sort_order);
