-- 0014: Drop legacy work_orders tables (auto_repair + bike_shop) y rename _v2 → nombre canónico.
-- Pre-condición: 0013 ya copió todos los datos a workshops.work_orders_v2 / work_order_items_v2.

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop tablas legacy bike_shop
-- ─────────────────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS workshops.bike_work_order_items CASCADE;
DROP TABLE IF EXISTS workshops.bike_work_orders CASCADE;

-- ─────────────────────────────────────────────────────────────────────────────
-- Drop tablas legacy auto_repair
-- ─────────────────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS workshops.work_order_items CASCADE;
DROP TABLE IF EXISTS workshops.work_orders CASCADE;

-- ─────────────────────────────────────────────────────────────────────────────
-- Rename _v2 → canonical
-- ─────────────────────────────────────────────────────────────────────────────
ALTER TABLE workshops.work_orders_v2 RENAME TO work_orders;
ALTER TABLE workshops.work_order_items_v2 RENAME TO work_order_items;

-- Rename índices al nombre canónico (sin _v2).
ALTER INDEX IF EXISTS workshops.workshops_work_orders_v2_org_number_active_idx
    RENAME TO workshops_work_orders_org_number_active_idx;
ALTER INDEX IF EXISTS workshops.workshops_work_orders_v2_org_target_idx
    RENAME TO workshops_work_orders_org_target_idx;
ALTER INDEX IF EXISTS workshops.workshops_work_orders_v2_org_status_idx
    RENAME TO workshops_work_orders_org_status_idx;
ALTER INDEX IF EXISTS workshops.workshops_work_order_items_v2_order_idx
    RENAME TO workshops_work_order_items_order_idx;
