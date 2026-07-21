DROP INDEX IF EXISTS idx_sales_org_branch_date;

ALTER TABLE sales
    DROP COLUMN IF EXISTS branch_id;
