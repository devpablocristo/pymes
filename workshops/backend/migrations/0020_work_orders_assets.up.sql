ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS asset_type text,
    ADD COLUMN IF NOT EXISTS asset_id uuid,
    ADD COLUMN IF NOT EXISTS asset_label text;

UPDATE workshops.work_orders
   SET asset_type = COALESCE(NULLIF(TRIM(asset_type), ''), target_type),
       asset_id = COALESCE(asset_id, target_id),
       asset_label = COALESCE(NULLIF(TRIM(asset_label), ''), target_label)
 WHERE asset_type IS NULL
    OR asset_id IS NULL
    OR asset_label IS NULL
    OR TRIM(asset_type) = ''
    OR TRIM(asset_label) = '';

ALTER TABLE workshops.work_orders
    ALTER COLUMN asset_type SET NOT NULL,
    ALTER COLUMN asset_id SET NOT NULL,
    ALTER COLUMN asset_label SET NOT NULL,
    ALTER COLUMN asset_label SET DEFAULT '';

CREATE INDEX IF NOT EXISTS workshops_work_orders_org_asset_idx
    ON workshops.work_orders (org_id, asset_type, asset_id);
