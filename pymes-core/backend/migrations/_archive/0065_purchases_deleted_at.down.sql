DROP INDEX IF EXISTS idx_purchases_org_deleted_at;

ALTER TABLE purchases
    DROP COLUMN IF EXISTS deleted_at;
