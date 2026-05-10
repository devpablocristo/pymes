DROP INDEX IF EXISTS workshops.idx_work_orders_org_asset;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema='workshops' AND table_name='work_orders' AND column_name='asset_type'
    ) THEN
        ALTER TABLE workshops.work_orders RENAME COLUMN asset_type TO target_type;
        ALTER TABLE workshops.work_orders RENAME COLUMN asset_id TO target_id;
        ALTER TABLE workshops.work_orders RENAME COLUMN asset_label TO target_label;
    END IF;
END $$;
