ALTER TABLE restaurant.dining_areas
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE restaurant.dining_tables
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_dining_areas_deleted_at
  ON restaurant.dining_areas (tenant_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_dining_tables_deleted_at
  ON restaurant.dining_tables (tenant_id, deleted_at);
