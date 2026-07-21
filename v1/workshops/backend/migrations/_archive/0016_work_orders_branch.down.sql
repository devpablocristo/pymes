DROP INDEX IF EXISTS workshops_work_orders_org_branch_active_idx;

ALTER TABLE workshops.work_orders
    DROP COLUMN IF EXISTS branch_id;
