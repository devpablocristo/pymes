ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS branch_id uuid;

CREATE INDEX IF NOT EXISTS idx_sales_org_branch_date
    ON sales(org_id, branch_id, created_at DESC);
