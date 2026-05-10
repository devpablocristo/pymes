ALTER TABLE services
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE products
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE parties
    DROP COLUMN IF EXISTS is_favorite;
