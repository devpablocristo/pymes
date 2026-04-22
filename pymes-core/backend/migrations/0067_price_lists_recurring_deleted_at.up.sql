ALTER TABLE price_lists
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_price_lists_org_deleted_at
    ON price_lists (org_id, deleted_at);

ALTER TABLE recurring_expenses
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_recurring_expenses_org_deleted_at
    ON recurring_expenses (org_id, deleted_at);
