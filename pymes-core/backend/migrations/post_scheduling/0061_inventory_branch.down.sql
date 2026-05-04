CREATE TEMP TABLE stock_levels_merged AS
SELECT
    org_id,
    product_id,
    SUM(quantity) AS quantity,
    MAX(min_quantity) AS min_quantity,
    MAX(updated_at) AS updated_at
FROM stock_levels
GROUP BY org_id, product_id;

ALTER TABLE stock_levels
    DROP CONSTRAINT IF EXISTS stock_levels_pkey;

DROP INDEX IF EXISTS ux_stock_levels_org_product_legacy;
DROP INDEX IF EXISTS ux_stock_levels_org_branch_product;
DROP INDEX IF EXISTS idx_stock_levels_org_branch_product;
DROP INDEX IF EXISTS idx_stock_low;

DELETE FROM stock_levels;

ALTER TABLE stock_levels
    DROP COLUMN IF EXISTS branch_id;

INSERT INTO stock_levels (org_id, product_id, quantity, min_quantity, updated_at)
SELECT org_id, product_id, quantity, min_quantity, updated_at
FROM stock_levels_merged;

ALTER TABLE stock_levels
    ADD PRIMARY KEY (org_id, product_id);

CREATE INDEX IF NOT EXISTS idx_stock_low
    ON stock_levels(org_id)
    WHERE quantity <= min_quantity AND min_quantity > 0;

DROP INDEX IF EXISTS idx_stock_movements_org_branch;
DROP INDEX IF EXISTS idx_stock_movements_branch_product;

ALTER TABLE stock_movements
    DROP COLUMN IF EXISTS branch_id;
