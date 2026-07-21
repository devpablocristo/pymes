ALTER TABLE products
    ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true;

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true;
