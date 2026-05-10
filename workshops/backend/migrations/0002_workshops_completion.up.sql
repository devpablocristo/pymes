-- 0002_workshops_completion.up.sql
-- Alinea schema squash 0001_workshops con código Go (que usa asset_*).

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='workshops' AND table_name='work_orders' AND column_name='target_type'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='workshops' AND table_name='work_orders' AND column_name='asset_type'
    ) THEN
        ALTER TABLE workshops.work_orders RENAME COLUMN target_type TO asset_type;
        ALTER TABLE workshops.work_orders RENAME COLUMN target_id TO asset_id;
        ALTER TABLE workshops.work_orders RENAME COLUMN target_label TO asset_label;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname='workshops' AND indexname='idx_work_orders_org_target') THEN
        DROP INDEX IF EXISTS workshops.idx_work_orders_org_target;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_work_orders_org_asset
    ON workshops.work_orders(org_id, asset_type) WHERE archived_at IS NULL;

-- Columnas faltantes en workshops.work_orders (asumidas por código y seeds)
ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS branch_id uuid,
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[];

CREATE INDEX IF NOT EXISTS idx_work_orders_org_branch
    ON workshops.work_orders(org_id, branch_id) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_orders_org_is_favorite
    ON workshops.work_orders(org_id, is_favorite) WHERE is_favorite = true;
