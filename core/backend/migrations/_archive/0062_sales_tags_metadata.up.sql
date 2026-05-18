-- Etiquetas internas y metadata JSON para ventas (paridad CRUD con quotes/purchases).

ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
