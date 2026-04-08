-- Rollback de 0013: drop de las tablas unificadas. Las tablas legacy siguen vivas.
DROP TABLE IF EXISTS workshops.work_order_items_v2 CASCADE;
DROP TABLE IF EXISTS workshops.work_orders_v2 CASCADE;
