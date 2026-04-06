ALTER TABLE services
    DROP COLUMN IF EXISTS is_active;

ALTER TABLE products
    DROP COLUMN IF EXISTS is_active;
