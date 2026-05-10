-- 0006_inventory.up.sql
-- Stock levels + movements. branch_id es una referencia LÓGICA a
-- scheduling_branches (creada por la lib `modules/scheduling/go`); no se
-- declara FK explícita para mantener pymes-core autocontenido. La integridad
-- la garantiza el runtime Go en queries cross-tabla.

-- stock_levels: branch_id NULL = stock global. NO usamos PK compuesta porque
-- Postgres requiere NOT NULL en cada column de la PK; usamos unique partial.
CREATE TABLE IF NOT EXISTS stock_levels (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    branch_id uuid,
    quantity numeric(15,2) NOT NULL DEFAULT 0,
    min_quantity numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_levels_global
    ON stock_levels(org_id, product_id) WHERE branch_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_levels_branched
    ON stock_levels(org_id, product_id, branch_id) WHERE branch_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_levels_org_product
    ON stock_levels(org_id, product_id);
CREATE INDEX IF NOT EXISTS idx_stock_levels_org_branch_product
    ON stock_levels(org_id, branch_id, product_id);
CREATE INDEX IF NOT EXISTS idx_stock_levels_low
    ON stock_levels(org_id, branch_id)
    WHERE quantity <= min_quantity AND min_quantity > 0;

CREATE TRIGGER trg_stock_levels_updated_at
    BEFORE UPDATE ON stock_levels FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS stock_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    branch_id uuid,
    type text NOT NULL
        CONSTRAINT stock_movements_type_check
        CHECK (type IN ('in','out','adjustment')),
    quantity numeric(15,2) NOT NULL,
    reason text NOT NULL DEFAULT '',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_stock_movements_org
    ON stock_movements(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_movements_org_product
    ON stock_movements(org_id, product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_movements_org_branch
    ON stock_movements(org_id, branch_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_movements_branch_product
    ON stock_movements(org_id, branch_id, product_id, created_at DESC);

-- Procurement: solicitudes internas de compra/gasto (legacy 0025).
CREATE TABLE IF NOT EXISTS procurement_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    requester_actor text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    estimated_total numeric(18, 4) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    evaluation_json jsonb,
    purchase_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_org ON procurement_requests(org_id);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_status ON procurement_requests(org_id, status);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_deleted ON procurement_requests(org_id, deleted_at);

CREATE TABLE IF NOT EXISTS procurement_request_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id uuid NOT NULL REFERENCES procurement_requests(id) ON DELETE CASCADE,
    description text NOT NULL DEFAULT '',
    product_id uuid,
    quantity numeric(18, 4) NOT NULL DEFAULT 1,
    unit_price_estimate numeric(18, 4) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_procurement_request_lines_request
    ON procurement_request_lines(request_id);

CREATE TRIGGER trg_procurement_requests_updated_at
    BEFORE UPDATE ON procurement_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
