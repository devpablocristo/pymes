ALTER TABLE purchases
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_purchases_org_deleted_at
    ON purchases (org_id, deleted_at);
