ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS branch_id UUID NULL;

CREATE INDEX IF NOT EXISTS workshops_work_orders_org_branch_active_idx
    ON workshops.work_orders (org_id, branch_id)
    WHERE archived_at IS NULL;
