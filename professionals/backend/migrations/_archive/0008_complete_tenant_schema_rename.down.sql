DROP INDEX IF EXISTS professionals.idx_sessions_deleted_at;
DROP INDEX IF EXISTS professionals.idx_intakes_deleted_at;
DROP INDEX IF EXISTS professionals.idx_specialties_deleted_at;
DROP INDEX IF EXISTS professionals.idx_professional_profiles_deleted_at;
DROP INDEX IF EXISTS professionals.idx_sessions_tenant_service_id;
DROP INDEX IF EXISTS professionals.idx_intakes_tenant_service_id;
DROP INDEX IF EXISTS professionals.idx_service_links_tenant_service_id;
DROP INDEX IF EXISTS professionals.idx_session_notes_tenant_id;
DROP INDEX IF EXISTS professionals.idx_sessions_tenant_status;
DROP INDEX IF EXISTS professionals.idx_sessions_tenant_id;
DROP INDEX IF EXISTS professionals.idx_intakes_tenant_status;
DROP INDEX IF EXISTS professionals.idx_intakes_tenant_id;
DROP INDEX IF EXISTS professionals.idx_service_links_tenant_id;
DROP INDEX IF EXISTS professionals.idx_specialties_tenant_id;
DROP INDEX IF EXISTS professionals.idx_professional_profiles_tenant_id;

ALTER TABLE professionals.professional_profiles
  DROP CONSTRAINT IF EXISTS professional_profiles_tenant_id_public_slug_key;
ALTER TABLE professionals.specialties
  DROP CONSTRAINT IF EXISTS specialties_tenant_id_code_key;
ALTER TABLE professionals.professional_specialties
  DROP CONSTRAINT IF EXISTS professional_specialties_tenant_id_profile_id_specialty_id_key;
ALTER TABLE professionals.sessions
  DROP CONSTRAINT IF EXISTS sessions_tenant_id_booking_id_key;

DO $$
DECLARE
  v_table text;
BEGIN
  FOREACH v_table IN ARRAY ARRAY[
    'professional_profiles',
    'specialties',
    'professional_specialties',
    'professional_service_links',
    'intakes',
    'sessions',
    'session_notes'
  ]
  LOOP
    IF EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'professionals'
        AND table_name = v_table
        AND column_name = 'tenant_id'
    ) AND NOT EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'professionals'
        AND table_name = v_table
        AND column_name = 'org_id'
    ) THEN
      EXECUTE format('ALTER TABLE professionals.%I RENAME COLUMN tenant_id TO org_id', v_table);
    END IF;
  END LOOP;
END $$;

ALTER TABLE professionals.professional_profiles
  ADD CONSTRAINT professional_profiles_org_id_public_slug_key UNIQUE (org_id, public_slug);
ALTER TABLE professionals.specialties
  ADD CONSTRAINT specialties_org_id_code_key UNIQUE (org_id, code);
ALTER TABLE professionals.professional_specialties
  ADD CONSTRAINT professional_specialties_org_id_profile_id_specialty_id_key UNIQUE (org_id, profile_id, specialty_id);
ALTER TABLE professionals.sessions
  ADD CONSTRAINT sessions_org_id_booking_id_key UNIQUE (org_id, booking_id);

CREATE INDEX IF NOT EXISTS idx_professional_profiles_org_id
  ON professionals.professional_profiles (org_id);
CREATE INDEX IF NOT EXISTS idx_specialties_org_id
  ON professionals.specialties (org_id);
CREATE INDEX IF NOT EXISTS idx_service_links_org_id
  ON professionals.professional_service_links (org_id);
CREATE INDEX IF NOT EXISTS idx_intakes_org_id
  ON professionals.intakes (org_id);
CREATE INDEX IF NOT EXISTS idx_intakes_org_status
  ON professionals.intakes (org_id, status);
CREATE INDEX IF NOT EXISTS idx_sessions_org_id
  ON professionals.sessions (org_id);
CREATE INDEX IF NOT EXISTS idx_sessions_org_status
  ON professionals.sessions (org_id, status);

CREATE INDEX IF NOT EXISTS idx_service_links_org_service_id
  ON professionals.professional_service_links (org_id, service_id)
  WHERE service_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_intakes_org_service_id
  ON professionals.intakes (org_id, service_id)
  WHERE service_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_org_service_id
  ON professionals.sessions (org_id, service_id)
  WHERE service_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_professional_profiles_deleted_at
  ON professionals.professional_profiles (org_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_specialties_deleted_at
  ON professionals.specialties (org_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_intakes_deleted_at
  ON professionals.intakes (org_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted_at
  ON professionals.sessions (org_id, deleted_at);
