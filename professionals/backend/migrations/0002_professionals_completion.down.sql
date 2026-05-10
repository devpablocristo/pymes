DROP INDEX IF EXISTS professionals.idx_professional_profiles_is_favorite;

ALTER TABLE professionals.sessions
    DROP COLUMN IF EXISTS summary,
    DROP COLUMN IF EXISTS ended_at,
    DROP COLUMN IF EXISTS started_at;

ALTER TABLE professionals.intakes
    DROP COLUMN IF EXISTS payload,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE professionals.specialties
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE professionals.professional_profiles
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
