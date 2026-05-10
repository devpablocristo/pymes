-- 0002_restaurants_completion.up.sql
-- Columnas faltantes en el squash 0001_restaurants: is_favorite + tags + metadata
-- en dining_areas / dining_tables que el código GORM asume.

ALTER TABLE restaurant.dining_areas
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE restaurant.dining_tables
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
