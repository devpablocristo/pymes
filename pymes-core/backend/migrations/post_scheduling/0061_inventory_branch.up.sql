ALTER TABLE stock_levels
    ADD COLUMN IF NOT EXISTS branch_id uuid REFERENCES scheduling_branches(id) ON DELETE CASCADE;

ALTER TABLE stock_movements
    ADD COLUMN IF NOT EXISTS branch_id uuid REFERENCES scheduling_branches(id) ON DELETE CASCADE;

WITH one_branch AS (
    SELECT org_id, id AS branch_id
    FROM (
        SELECT
            org_id,
            id,
            COUNT(*) OVER (PARTITION BY org_id) AS branch_count,
            ROW_NUMBER() OVER (PARTITION BY org_id ORDER BY created_at, id) AS rn
        FROM scheduling_branches
        WHERE active = true
    ) ranked
    WHERE branch_count = 1
      AND rn = 1
)
UPDATE stock_levels sl
SET branch_id = ob.branch_id
FROM one_branch ob
WHERE sl.org_id = ob.org_id
  AND sl.branch_id IS NULL;

WITH one_branch AS (
    SELECT org_id, id AS branch_id
    FROM (
        SELECT
            org_id,
            id,
            COUNT(*) OVER (PARTITION BY org_id) AS branch_count,
            ROW_NUMBER() OVER (PARTITION BY org_id ORDER BY created_at, id) AS rn
        FROM scheduling_branches
        WHERE active = true
    ) ranked
    WHERE branch_count = 1
      AND rn = 1
)
UPDATE stock_movements sm
SET branch_id = ob.branch_id
FROM one_branch ob
WHERE sm.org_id = ob.org_id
  AND sm.branch_id IS NULL;

ALTER TABLE stock_levels
    DROP CONSTRAINT IF EXISTS stock_levels_pkey;

CREATE UNIQUE INDEX IF NOT EXISTS ux_stock_levels_org_product_legacy
    ON stock_levels(org_id, product_id)
    WHERE branch_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_stock_levels_org_branch_product
    ON stock_levels(org_id, branch_id, product_id)
    WHERE branch_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stock_levels_org_branch_product
    ON stock_levels(org_id, branch_id, product_id);

DROP INDEX IF EXISTS idx_stock_low;

CREATE INDEX IF NOT EXISTS idx_stock_low
    ON stock_levels(org_id, branch_id)
    WHERE quantity <= min_quantity AND min_quantity > 0;

CREATE INDEX IF NOT EXISTS idx_stock_movements_org_branch
    ON stock_movements(org_id, branch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_stock_movements_branch_product
    ON stock_movements(org_id, branch_id, product_id, created_at DESC);
