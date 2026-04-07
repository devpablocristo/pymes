-- Rollback de 0015: recrear la tabla vacia (los datos no se restauran).
CREATE TABLE IF NOT EXISTS workshops.bicycles (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    customer_id UUID,
    customer_name TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '',
    frame_number TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
