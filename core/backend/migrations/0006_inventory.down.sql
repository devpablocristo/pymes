-- 0006_inventory.down.sql

DROP TRIGGER IF EXISTS trg_stock_levels_updated_at ON stock_levels;

DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS stock_levels;
