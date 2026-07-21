DROP INDEX IF EXISTS workshops.workshops_work_orders_org_asset_idx;

ALTER TABLE workshops.work_orders
    DROP COLUMN IF EXISTS asset_label,
    DROP COLUMN IF EXISTS asset_id,
    DROP COLUMN IF EXISTS asset_type;
