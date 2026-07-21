ALTER TABLE purchases
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
