UPDATE workshops.work_orders
   SET asset_type = COALESCE(NULLIF(TRIM(asset_type), ''), target_type),
       asset_id = COALESCE(asset_id, target_id),
       asset_label = COALESCE(NULLIF(TRIM(asset_label), ''), target_label, '')
 WHERE asset_type IS NULL
    OR asset_id IS NULL
    OR asset_label IS NULL
    OR TRIM(asset_type) = '';

ALTER TABLE workshops.work_orders
    ALTER COLUMN asset_type SET NOT NULL,
    ALTER COLUMN asset_id SET NOT NULL,
    ALTER COLUMN asset_label SET NOT NULL,
    ALTER COLUMN asset_label SET DEFAULT '';

DROP INDEX IF EXISTS workshops.workshops_work_orders_org_target_idx;

ALTER TABLE workshops.work_orders
    DROP COLUMN IF EXISTS target_label,
    DROP COLUMN IF EXISTS target_id,
    DROP COLUMN IF EXISTS target_type;
