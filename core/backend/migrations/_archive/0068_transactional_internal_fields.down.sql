DROP INDEX IF EXISTS idx_returns_org_deleted_at;
ALTER TABLE returns
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

DROP INDEX IF EXISTS idx_payments_org_deleted_at;
ALTER TABLE payments
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

DROP INDEX IF EXISTS idx_cash_movements_org_deleted_at;
ALTER TABLE cash_movements
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
