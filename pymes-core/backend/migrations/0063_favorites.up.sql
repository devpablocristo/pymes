ALTER TABLE parties
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false;

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false;

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false;
