DROP INDEX IF EXISTS restaurant.idx_dining_tables_deleted_at;
DROP INDEX IF EXISTS restaurant.idx_dining_areas_deleted_at;

ALTER TABLE restaurant.dining_tables
  DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE restaurant.dining_areas
  DROP COLUMN IF EXISTS deleted_at;
