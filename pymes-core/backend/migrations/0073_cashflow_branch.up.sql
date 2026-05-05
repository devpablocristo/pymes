ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS branch_id uuid;

CREATE INDEX IF NOT EXISTS idx_sales_org_branch_date
    ON sales(org_id, branch_id, created_at DESC);

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
UPDATE sales s
SET branch_id = ob.branch_id
FROM one_branch ob
WHERE s.org_id = ob.org_id
  AND s.branch_id IS NULL;

ALTER TABLE cash_movements
    ADD COLUMN IF NOT EXISTS branch_id uuid REFERENCES scheduling_branches(id) ON DELETE CASCADE;

UPDATE cash_movements cm
SET branch_id = s.branch_id
FROM sales s
WHERE cm.org_id = s.org_id
  AND cm.reference_type = 'sale'
  AND cm.reference_id = s.id
  AND cm.branch_id IS NULL;

UPDATE cash_movements cm
SET branch_id = s.branch_id
FROM returns r
JOIN sales s ON s.id = r.sale_id AND s.org_id = r.org_id
WHERE cm.org_id = r.org_id
  AND cm.reference_type = 'return'
  AND cm.reference_id = r.id
  AND cm.branch_id IS NULL;

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
UPDATE cash_movements cm
SET branch_id = ob.branch_id
FROM one_branch ob
WHERE cm.org_id = ob.org_id
  AND cm.branch_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_cash_movements_org_branch
    ON cash_movements(org_id, branch_id, created_at DESC);
