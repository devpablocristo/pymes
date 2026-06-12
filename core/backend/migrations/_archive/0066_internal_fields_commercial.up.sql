-- Campos internos (favoritos + etiquetas) para el resto de CRUDs comerciales.
ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE price_lists
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE recurring_expenses
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';
