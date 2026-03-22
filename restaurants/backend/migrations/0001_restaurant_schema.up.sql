CREATE SCHEMA IF NOT EXISTS restaurant;

CREATE TABLE IF NOT EXISTS restaurant.dining_areas (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS dining_areas_org_sort_idx
    ON restaurant.dining_areas (org_id, sort_order, id);

CREATE TABLE IF NOT EXISTS restaurant.dining_tables (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    area_id UUID NOT NULL REFERENCES restaurant.dining_areas (id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    capacity INTEGER NOT NULL DEFAULT 4,
    status TEXT NOT NULL DEFAULT 'available',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT dining_tables_capacity_chk CHECK (capacity >= 1 AND capacity <= 99),
    CONSTRAINT dining_tables_status_chk CHECK (status IN ('available', 'occupied', 'reserved', 'cleaning'))
);

CREATE UNIQUE INDEX IF NOT EXISTS dining_tables_org_code_uidx
    ON restaurant.dining_tables (org_id, code);

CREATE INDEX IF NOT EXISTS dining_tables_org_area_idx
    ON restaurant.dining_tables (org_id, area_id);

CREATE TABLE IF NOT EXISTS restaurant.table_sessions (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    table_id UUID NOT NULL REFERENCES restaurant.dining_tables (id) ON DELETE CASCADE,
    guest_count INTEGER NOT NULL DEFAULT 1,
    party_label TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT table_sessions_guests_chk CHECK (guest_count >= 1 AND guest_count <= 99)
);

CREATE UNIQUE INDEX IF NOT EXISTS table_sessions_one_open_per_table_uidx
    ON restaurant.table_sessions (table_id)
    WHERE closed_at IS NULL;

CREATE INDEX IF NOT EXISTS table_sessions_org_opened_idx
    ON restaurant.table_sessions (org_id, opened_at DESC);
