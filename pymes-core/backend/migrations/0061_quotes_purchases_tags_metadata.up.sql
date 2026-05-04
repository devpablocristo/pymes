-- Etiquetas internas y metadata JSON (p. ej. favorite) para documentos comerciales en CRUD unificado.

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE purchases
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;
