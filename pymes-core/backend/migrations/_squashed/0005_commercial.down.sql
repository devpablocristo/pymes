-- 0005_commercial.down.sql

ALTER TABLE party_roles DROP CONSTRAINT IF EXISTS party_roles_price_list_id_fkey;
ALTER TABLE party_roles DROP COLUMN IF EXISTS price_list_id;

DROP TRIGGER IF EXISTS trg_price_lists_updated_at ON price_lists;
DROP TRIGGER IF EXISTS trg_services_updated_at ON services;
DROP TRIGGER IF EXISTS trg_products_updated_at ON products;

DROP TABLE IF EXISTS service_price_list_items;
DROP TABLE IF EXISTS price_list_items;
DROP TABLE IF EXISTS price_lists;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS products;
