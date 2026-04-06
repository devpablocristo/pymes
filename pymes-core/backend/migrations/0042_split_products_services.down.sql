INSERT INTO price_list_items (price_list_id, product_id, price)
SELECT price_list_id, service_id, price
FROM service_price_list_items
ON CONFLICT (price_list_id, product_id) DO NOTHING;

DROP TABLE IF EXISTS service_price_list_items;

DROP INDEX IF EXISTS idx_purchase_items_service_id;
ALTER TABLE purchase_items DROP COLUMN IF EXISTS service_id;

DROP INDEX IF EXISTS idx_quote_items_service_id;
ALTER TABLE quote_items DROP COLUMN IF EXISTS service_id;

DROP INDEX IF EXISTS idx_sale_items_service_id;
ALTER TABLE sale_items DROP COLUMN IF EXISTS service_id;

DROP INDEX IF EXISTS idx_catalog_services_org_name;
DROP INDEX IF EXISTS idx_catalog_services_org;
DROP INDEX IF EXISTS idx_catalog_services_org_code;
DROP TABLE IF EXISTS catalog_services;
