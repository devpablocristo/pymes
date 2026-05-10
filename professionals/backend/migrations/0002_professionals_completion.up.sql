-- 0002_professionals_completion.up.sql
-- Columnas faltantes en el squash 0001_professionals: is_favorite + tags
-- + payload (intakes) en las tablas que el código GORM asume.

ALTER TABLE professionals.professional_profiles
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[];

ALTER TABLE professionals.specialties
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[];

ALTER TABLE professionals.intakes
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS payload jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE professionals.sessions
    ADD COLUMN IF NOT EXISTS started_at timestamptz,
    ADD COLUMN IF NOT EXISTS ended_at timestamptz,
    ADD COLUMN IF NOT EXISTS summary text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_professional_profiles_is_favorite
    ON professionals.professional_profiles(org_id, is_favorite) WHERE is_favorite = true;
