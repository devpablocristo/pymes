ALTER TABLE quotes
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE sales
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE price_lists
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE recurring_expenses
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
