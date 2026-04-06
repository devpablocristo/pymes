DROP INDEX IF EXISTS workshops.workshops_bike_work_orders_org_number_active_idx;
ALTER TABLE workshops.bike_work_orders DROP COLUMN IF EXISTS archived_at;
CREATE UNIQUE INDEX IF NOT EXISTS workshops_bike_work_orders_org_number_idx
    ON workshops.bike_work_orders (org_id, number);

DROP INDEX IF EXISTS workshops.workshops_bicycles_org_frame_active_idx;
ALTER TABLE workshops.bicycles DROP COLUMN IF EXISTS archived_at;
CREATE UNIQUE INDEX IF NOT EXISTS workshops_bicycles_org_frame_idx
    ON workshops.bicycles (org_id, frame_number);
