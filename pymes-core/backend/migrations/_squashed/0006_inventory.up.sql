-- 0006_inventory.up.sql
-- Stock levels + movements. branch_id es una referencia LÓGICA a
-- scheduling_branches (creada por la lib `modules/scheduling/go`); no se
-- declara FK explícita para mantener pymes-core autocontenido. La integridad
-- la garantiza el runtime Go en queries cross-tabla.

CREATE TABLE IF NOT EXISTS stock_levels (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    branch_id uuid,
    quantity numeric(15,2) NOT NULL DEFAULT 0,
    min_quantity numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, product_id, branch_id)
);
-- UNIQUE para "stock global" cuando branch_id IS NULL (reemplaza la PK
-- compuesta para queries de stock no segmentado por branch). Vía índice
-- parcial — PostgreSQL no permite UNIQUE WHERE como column constraint.
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_levels_global_uniq
    ON stock_levels(org_id, product_id) WHERE branch_id IS NULL;

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
