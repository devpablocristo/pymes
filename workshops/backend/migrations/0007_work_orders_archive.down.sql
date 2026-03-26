DROP INDEX IF EXISTS workshops.workshops_work_orders_org_number_active_idx;

ALTER TABLE workshops.work_orders
    DROP COLUMN IF EXISTS archived_at;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_work_orders_org_number_idx
    ON workshops.work_orders (org_id, number);
