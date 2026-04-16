DROP INDEX IF EXISTS idx_cash_movements_org_branch;

ALTER TABLE cash_movements
    DROP COLUMN IF EXISTS branch_id;
