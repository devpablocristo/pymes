ALTER TABLE workshops.work_orders
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE workshops.vehicles
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE workshops.bicycles
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
