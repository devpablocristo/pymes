ALTER TABLE restaurant.dining_areas
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE restaurant.dining_tables
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
