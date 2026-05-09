ALTER TABLE professionals.specialties
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE professionals.professional_profiles
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE professionals.intakes
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';
