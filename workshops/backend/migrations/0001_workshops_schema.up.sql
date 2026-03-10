CREATE SCHEMA IF NOT EXISTS workshops;

CREATE TABLE IF NOT EXISTS workshops.vehicles (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    customer_id UUID NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    license_plate TEXT NOT NULL,
    vin TEXT NOT NULL DEFAULT '',
    make TEXT NOT NULL,
    model TEXT NOT NULL,
    year INTEGER NOT NULL DEFAULT 0,
    kilometers INTEGER NOT NULL DEFAULT 0,
    color TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS workshops_vehicles_org_plate_idx
    ON workshops.vehicles (org_id, license_plate);

CREATE TABLE IF NOT EXISTS workshops.services (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    estimated_hours DOUBLE PRECISION NOT NULL DEFAULT 0,
    base_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'ARS',
    tax_rate DOUBLE PRECISION NOT NULL DEFAULT 21,
    linked_product_id UUID NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS workshops_services_org_code_idx
    ON workshops.services (org_id, code);

CREATE TABLE IF NOT EXISTS workshops.work_orders (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    number TEXT NOT NULL,
    vehicle_id UUID NOT NULL,
    vehicle_plate TEXT NOT NULL DEFAULT '',
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

CREATE UNIQUE INDEX IF NOT EXISTS workshops_work_orders_org_number_idx
    ON workshops.work_orders (org_id, number);

CREATE INDEX IF NOT EXISTS workshops_work_orders_org_status_idx
    ON workshops.work_orders (org_id, status);

CREATE TABLE IF NOT EXISTS workshops.work_order_items (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    work_order_id UUID NOT NULL REFERENCES workshops.work_orders(id) ON DELETE CASCADE,
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

CREATE INDEX IF NOT EXISTS workshops_work_order_items_order_idx
    ON workshops.work_order_items (work_order_id, sort_order);
