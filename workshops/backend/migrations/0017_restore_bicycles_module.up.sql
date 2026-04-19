-- 0017: restaurar el bounded context bike_shop/bicycles como módulo propio.
CREATE TABLE IF NOT EXISTS workshops.bicycles (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    customer_id UUID NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    frame_number TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    bike_type TEXT NOT NULL DEFAULT '',
    size TEXT NOT NULL DEFAULT '',
    wheel_size_inches INTEGER NOT NULL DEFAULT 0,
    color TEXT NOT NULL DEFAULT '',
    ebike_notes TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    archived_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE workshops.bicycles
    ADD COLUMN IF NOT EXISTS customer_id UUID NULL,
    ADD COLUMN IF NOT EXISTS customer_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS frame_number TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS brand TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bike_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS size TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS wheel_size_inches INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS color TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ebike_notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS workshops_bicycles_org_frame_active_idx
    ON workshops.bicycles (org_id, frame_number)
    WHERE archived_at IS NULL;

CREATE INDEX IF NOT EXISTS workshops_bicycles_org_customer_idx
    ON workshops.bicycles (org_id, customer_id);
