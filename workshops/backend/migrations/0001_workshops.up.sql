-- 0001_workshops.up.sql (vertical Workshops — squashed)
-- Schema isolado en `workshops.*` con FK a orgs(id) en pymes-core.
-- Consolida: 0001..0022 actuales (tras restore-bicycles + customer-assets +
-- work-orders v2 + 0022_complete_tenant_schema_rename).

CREATE SCHEMA IF NOT EXISTS workshops;

CREATE TABLE IF NOT EXISTS workshops.vehicles (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    customer_id uuid,
    customer_name text NOT NULL DEFAULT '',
    license_plate text NOT NULL,
    vin text NOT NULL DEFAULT '',
    make text NOT NULL,
    model text NOT NULL,
    year integer NOT NULL DEFAULT 0,
    kilometers integer NOT NULL DEFAULT 0,
    color text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vehicles_org_plate_active
    ON workshops.vehicles(org_id, license_plate) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_vehicles_org_deleted_at
    ON workshops.vehicles(org_id, archived_at);

CREATE TRIGGER trg_vehicles_updated_at
    BEFORE UPDATE ON workshops.vehicles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workshops.services (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    segment text NOT NULL DEFAULT 'auto_repair',
    estimated_hours numeric(10,2) NOT NULL DEFAULT 0,
    base_price numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    tax_rate numeric(5,2) NOT NULL DEFAULT 21,
    linked_product_id uuid,
    linked_service_id uuid,
    is_active boolean NOT NULL DEFAULT true,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_workshops_services_org_code_segment
    ON workshops.services(org_id, code, segment) WHERE archived_at IS NULL;

CREATE TRIGGER trg_workshops_services_updated_at
    BEFORE UPDATE ON workshops.services FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workshops.bicycles (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    customer_id uuid,
    customer_name text NOT NULL DEFAULT '',
    frame_number text NOT NULL,
    brand text NOT NULL,
    model text NOT NULL,
    bike_type text NOT NULL DEFAULT '',
    size text NOT NULL DEFAULT '',
    wheel_size_inches integer NOT NULL DEFAULT 0,
    color text NOT NULL DEFAULT '',
    ebike_notes text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bicycles_org_frame
    ON workshops.bicycles(org_id, frame_number);
CREATE INDEX IF NOT EXISTS idx_bicycles_org_deleted_at
    ON workshops.bicycles(org_id, archived_at);

CREATE TRIGGER trg_bicycles_updated_at
    BEFORE UPDATE ON workshops.bicycles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workshops.work_orders (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid,
    number text NOT NULL,
    asset_type text NOT NULL,
    asset_id uuid NOT NULL,
    asset_label text NOT NULL DEFAULT '',
    customer_id uuid,
    customer_name text NOT NULL DEFAULT '',
    booking_id uuid,
    quote_id uuid,
    sale_id uuid,
    status text NOT NULL,
    requested_work text NOT NULL DEFAULT '',
    diagnosis text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    internal_notes text NOT NULL DEFAULT '',
    currency text NOT NULL DEFAULT 'ARS',
    subtotal_services numeric(15,2) NOT NULL DEFAULT 0,
    subtotal_parts numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    opened_at timestamptz NOT NULL DEFAULT now(),
    promised_at timestamptz,
    ready_at timestamptz,
    delivered_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_work_orders_org_number_active
    ON workshops.work_orders(org_id, number) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_orders_org_asset
    ON workshops.work_orders(org_id, asset_type) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_orders_org_branch
    ON workshops.work_orders(org_id, branch_id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_orders_org_status
    ON workshops.work_orders(org_id, status) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_orders_org_archived_at
    ON workshops.work_orders(org_id, archived_at);
CREATE INDEX IF NOT EXISTS idx_work_orders_org_is_favorite
    ON workshops.work_orders(org_id, is_favorite) WHERE is_favorite = true;

CREATE TRIGGER trg_work_orders_updated_at
    BEFORE UPDATE ON workshops.work_orders FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workshops.work_order_items (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    work_order_id uuid NOT NULL REFERENCES workshops.work_orders(id) ON DELETE CASCADE,
    item_type text NOT NULL,
    service_id uuid,
    product_id uuid,
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 21,
    sort_order integer NOT NULL DEFAULT 0,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_work_order_items_order
    ON workshops.work_order_items(work_order_id, sort_order);

CREATE TRIGGER trg_work_order_items_updated_at
    BEFORE UPDATE ON workshops.work_order_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workshops.customer_assets (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    asset_type text NOT NULL,
    customer_id uuid,
    customer_name text NOT NULL DEFAULT '',
    label text NOT NULL DEFAULT '',
    brand text NOT NULL DEFAULT '',
    model text NOT NULL DEFAULT '',
    serial_number text NOT NULL DEFAULT '',
    year integer NOT NULL DEFAULT 0,
    color text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_customer_assets_org_type_active
    ON workshops.customer_assets(org_id, asset_type, archived_at);
CREATE INDEX IF NOT EXISTS idx_customer_assets_org_type_id
    ON workshops.customer_assets(org_id, asset_type, id DESC);

CREATE TRIGGER trg_customer_assets_updated_at
    BEFORE UPDATE ON workshops.customer_assets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS workshops.work_order_assets (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    work_order_id uuid NOT NULL REFERENCES workshops.work_orders(id) ON DELETE CASCADE,
    asset_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_work_order_assets_org
    ON workshops.work_order_assets(org_id);
CREATE INDEX IF NOT EXISTS idx_work_order_assets_order
    ON workshops.work_order_assets(work_order_id);
