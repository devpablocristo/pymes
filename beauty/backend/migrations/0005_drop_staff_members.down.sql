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
