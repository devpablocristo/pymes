-- 0001_restaurants.up.sql (vertical Restaurants — squashed)
-- Schema isolado en `restaurant.*` con FK a orgs(id) en pymes-core.
-- Consolida: 0001..0004 actuales (post `0004_dining_archive` con deleted_at).

CREATE SCHEMA IF NOT EXISTS restaurant;

CREATE TABLE IF NOT EXISTS restaurant.dining_areas (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dining_areas_org_sort
    ON restaurant.dining_areas(org_id, sort_order, id);
CREATE INDEX IF NOT EXISTS idx_dining_areas_deleted_at
    ON restaurant.dining_areas(org_id, deleted_at);

CREATE TRIGGER trg_dining_areas_updated_at
    BEFORE UPDATE ON restaurant.dining_areas FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS restaurant.dining_tables (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    area_id uuid NOT NULL REFERENCES restaurant.dining_areas(id) ON DELETE CASCADE,
    code text NOT NULL,
    label text NOT NULL DEFAULT '',
    capacity integer NOT NULL DEFAULT 4
        CONSTRAINT dining_tables_capacity_check CHECK (capacity >= 1 AND capacity <= 99),
    status text NOT NULL DEFAULT 'available'
        CONSTRAINT dining_tables_status_check
        CHECK (status IN ('available','occupied','reserved','cleaning')),
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dining_tables_org_code
    ON restaurant.dining_tables(org_id, code);
CREATE INDEX IF NOT EXISTS idx_dining_tables_org_area
    ON restaurant.dining_tables(org_id, area_id);
CREATE INDEX IF NOT EXISTS idx_dining_tables_deleted_at
    ON restaurant.dining_tables(org_id, deleted_at);

CREATE TRIGGER trg_dining_tables_updated_at
    BEFORE UPDATE ON restaurant.dining_tables FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS restaurant.table_sessions (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    table_id uuid NOT NULL REFERENCES restaurant.dining_tables(id) ON DELETE CASCADE,
    guest_count integer NOT NULL DEFAULT 1
        CONSTRAINT table_sessions_guests_check CHECK (guest_count >= 1 AND guest_count <= 99),
    party_label text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    opened_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_table_sessions_one_open_per_table
    ON restaurant.table_sessions(table_id) WHERE closed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_table_sessions_org_opened
    ON restaurant.table_sessions(org_id, opened_at DESC);

CREATE TRIGGER trg_table_sessions_updated_at
    BEFORE UPDATE ON restaurant.table_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS restaurant.reservations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    guest_name text NOT NULL DEFAULT '',
    guest_phone text NOT NULL DEFAULT '',
    guest_email text NOT NULL DEFAULT '',
    guest_count integer NOT NULL DEFAULT 1,
    reserved_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT reservations_status_check
        CHECK (status IN ('pending','confirmed','seated','completed','cancelled','no_show')),
    notes text NOT NULL DEFAULT '',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_reservations_org_status
    ON restaurant.reservations(org_id, status, reserved_at DESC);
CREATE INDEX IF NOT EXISTS idx_reservations_org_date
    ON restaurant.reservations(org_id, reserved_at DESC);
CREATE INDEX IF NOT EXISTS idx_reservations_deleted_at
    ON restaurant.reservations(org_id, deleted_at);

CREATE TRIGGER trg_reservations_updated_at
    BEFORE UPDATE ON restaurant.reservations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
