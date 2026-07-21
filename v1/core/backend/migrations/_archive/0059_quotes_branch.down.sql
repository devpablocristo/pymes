DROP INDEX IF EXISTS idx_quotes_org_branch_date;

ALTER TABLE quotes
    DROP COLUMN IF EXISTS branch_id;
