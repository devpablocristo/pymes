ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS branch_id uuid;

CREATE INDEX IF NOT EXISTS idx_quotes_org_branch_date
    ON quotes(org_id, branch_id, created_at DESC);
