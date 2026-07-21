ALTER TABLE professionals.specialties
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE professionals.professional_profiles
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE professionals.intakes
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite;
