UPDATE products
SET deleted_at = COALESCE(deleted_at, now()),
    updated_at = now()
WHERE deleted_at IS NULL
  AND COALESCE(type, '') <> 'product';

ALTER TABLE products
    ADD CONSTRAINT chk_products_active_rows_are_products
    CHECK (deleted_at IS NOT NULL OR type = 'product') NOT VALID;

ALTER TABLE products
    VALIDATE CONSTRAINT chk_products_active_rows_are_products;
