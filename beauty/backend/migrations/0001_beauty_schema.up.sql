CREATE SCHEMA IF NOT EXISTS beauty;

CREATE TABLE IF NOT EXISTS beauty.staff_members (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '#6366f1',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS beauty_staff_org_active_idx
    ON beauty.staff_members (org_id, is_active);

CREATE TABLE IF NOT EXISTS beauty.salon_services (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    duration_minutes INTEGER NOT NULL DEFAULT 30,
    base_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'ARS',
    tax_rate DOUBLE PRECISION NOT NULL DEFAULT 21,
    linked_product_id UUID NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS beauty_salon_services_org_code_idx
    ON beauty.salon_services (org_id, code);
