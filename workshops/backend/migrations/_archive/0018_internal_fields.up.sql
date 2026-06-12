-- Campos internos (favoritos + etiquetas) para los CRUDs de workshops.
ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE workshops.vehicles
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE workshops.bicycles
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';
