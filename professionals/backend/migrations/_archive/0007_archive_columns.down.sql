DROP INDEX IF EXISTS professionals.idx_sessions_deleted_at;
DROP INDEX IF EXISTS professionals.idx_intakes_deleted_at;
DROP INDEX IF EXISTS professionals.idx_specialties_deleted_at;
DROP INDEX IF EXISTS professionals.idx_professional_profiles_deleted_at;

ALTER TABLE professionals.sessions
  DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE professionals.intakes
  DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE professionals.specialties
  DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE professionals.professional_profiles
  DROP COLUMN IF EXISTS deleted_at;
