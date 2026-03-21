DROP INDEX IF EXISTS workshops.workshops_bike_work_order_items_order_idx;

DROP TABLE IF EXISTS workshops.bike_work_order_items;
DROP TABLE IF EXISTS workshops.bike_work_orders;

DROP INDEX IF EXISTS workshops.workshops_bicycles_org_frame_idx;
DROP TABLE IF EXISTS workshops.bicycles;

DROP INDEX IF EXISTS workshops.workshops_services_org_segment_code_idx;

ALTER TABLE workshops.services DROP COLUMN IF EXISTS segment;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_services_org_code_idx
    ON workshops.services (org_id, code);
