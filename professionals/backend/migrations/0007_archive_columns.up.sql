ALTER TABLE professionals.professional_profiles
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE professionals.specialties
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE professionals.intakes
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE professionals.sessions
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_professional_profiles_deleted_at
  ON professionals.professional_profiles (org_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_specialties_deleted_at
  ON professionals.specialties (org_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_intakes_deleted_at
  ON professionals.intakes (org_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_sessions_deleted_at
  ON professionals.sessions (org_id, deleted_at);
