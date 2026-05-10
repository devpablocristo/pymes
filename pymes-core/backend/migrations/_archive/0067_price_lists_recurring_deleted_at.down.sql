DROP INDEX IF EXISTS idx_recurring_expenses_org_deleted_at;

ALTER TABLE recurring_expenses
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS idx_price_lists_org_deleted_at;

ALTER TABLE price_lists
    DROP COLUMN IF EXISTS deleted_at;
