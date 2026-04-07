-- Rollback de 0014: rename de vuelta a _v2. Las tablas legacy NO se restauran (se perdieron).
ALTER INDEX IF EXISTS workshops.workshops_work_order_items_order_idx
    RENAME TO workshops_work_order_items_v2_order_idx;
ALTER INDEX IF EXISTS workshops.workshops_work_orders_org_status_idx
    RENAME TO workshops_work_orders_v2_org_status_idx;
ALTER INDEX IF EXISTS workshops.workshops_work_orders_org_target_idx
    RENAME TO workshops_work_orders_v2_org_target_idx;
ALTER INDEX IF EXISTS workshops.workshops_work_orders_org_number_active_idx
    RENAME TO workshops_work_orders_v2_org_number_active_idx;

ALTER TABLE workshops.work_order_items RENAME TO work_order_items_v2;
ALTER TABLE workshops.work_orders RENAME TO work_orders_v2;
