DROP INDEX IF EXISTS idx_purchases_org_branch_date;

ALTER TABLE purchases
    DROP COLUMN IF EXISTS branch_id;
