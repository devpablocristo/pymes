ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS target_type text,
    ADD COLUMN IF NOT EXISTS target_id uuid,
    ADD COLUMN IF NOT EXISTS target_label text;

UPDATE workshops.work_orders
   SET target_type = COALESCE(NULLIF(TRIM(target_type), ''), asset_type),
       target_id = COALESCE(target_id, asset_id),
       target_label = COALESCE(NULLIF(TRIM(target_label), ''), asset_label, '')
 WHERE target_type IS NULL
    OR target_id IS NULL
    OR target_label IS NULL
    OR TRIM(target_type) = '';

ALTER TABLE workshops.work_orders
    ALTER COLUMN target_type SET NOT NULL,
    ALTER COLUMN target_id SET NOT NULL,
    ALTER COLUMN target_label SET NOT NULL,
    ALTER COLUMN target_label SET DEFAULT '';

CREATE INDEX IF NOT EXISTS workshops_work_orders_org_target_idx
    ON workshops.work_orders (tenant_id, target_type)
    WHERE archived_at IS NULL;
