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
        AND column_name = 'org_id'
    ) AND NOT EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'professionals'
        AND table_name = v_table
        AND column_name = 'tenant_id'
    ) THEN
      EXECUTE format('ALTER TABLE professionals.%I RENAME COLUMN org_id TO tenant_id', v_table);
    ELSIF EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'professionals'
        AND table_name = v_table
        AND column_name = 'org_id'
    ) AND EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'professionals'
        AND table_name = v_table
        AND column_name = 'tenant_id'
    ) THEN
      EXECUTE format('UPDATE professionals.%I SET tenant_id = org_id WHERE tenant_id IS NULL', v_table);
      EXECUTE format('ALTER TABLE professionals.%I DROP COLUMN org_id', v_table);
    END IF;
  END LOOP;
END $$;

DROP INDEX IF EXISTS professionals.idx_professional_profiles_org_id;
DROP INDEX IF EXISTS professionals.idx_specialties_org_id;
DROP INDEX IF EXISTS professionals.idx_service_links_org_id;
DROP INDEX IF EXISTS professionals.idx_intakes_org_id;
DROP INDEX IF EXISTS professionals.idx_intakes_org_status;
DROP INDEX IF EXISTS professionals.idx_sessions_org_id;
DROP INDEX IF EXISTS professionals.idx_sessions_org_status;
DROP INDEX IF EXISTS professionals.idx_service_links_org_service_id;
DROP INDEX IF EXISTS professionals.idx_intakes_org_service_id;
DROP INDEX IF EXISTS professionals.idx_sessions_org_service_id;

ALTER TABLE professionals.professional_profiles
  DROP CONSTRAINT IF EXISTS professional_profiles_org_id_public_slug_key;
ALTER TABLE professionals.specialties
  DROP CONSTRAINT IF EXISTS specialties_org_id_code_key;
ALTER TABLE professionals.professional_specialties
  DROP CONSTRAINT IF EXISTS professional_specialties_org_id_profile_id_specialty_id_key;
ALTER TABLE professionals.sessions
  DROP CONSTRAINT IF EXISTS sessions_org_id_appointment_id_key,
  DROP CONSTRAINT IF EXISTS sessions_org_id_booking_id_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'professional_profiles_tenant_id_public_slug_key'
      AND conrelid = 'professionals.professional_profiles'::regclass
  ) THEN
    ALTER TABLE professionals.professional_profiles
      ADD CONSTRAINT professional_profiles_tenant_id_public_slug_key UNIQUE (tenant_id, public_slug);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'specialties_tenant_id_code_key'
      AND conrelid = 'professionals.specialties'::regclass
  ) THEN
    ALTER TABLE professionals.specialties
      ADD CONSTRAINT specialties_tenant_id_code_key UNIQUE (tenant_id, code);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'professional_specialties_tenant_id_profile_id_specialty_id_key'
      AND conrelid = 'professionals.professional_specialties'::regclass
  ) THEN
    ALTER TABLE professionals.professional_specialties
      ADD CONSTRAINT professional_specialties_tenant_id_profile_id_specialty_id_key UNIQUE (tenant_id, profile_id, specialty_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'sessions_tenant_id_booking_id_key'
      AND conrelid = 'professionals.sessions'::regclass
  ) THEN
    ALTER TABLE professionals.sessions
      ADD CONSTRAINT sessions_tenant_id_booking_id_key UNIQUE (tenant_id, booking_id);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_professional_profiles_tenant_id
  ON professionals.professional_profiles (tenant_id);
CREATE INDEX IF NOT EXISTS idx_specialties_tenant_id
  ON professionals.specialties (tenant_id);
CREATE INDEX IF NOT EXISTS idx_service_links_tenant_id
  ON professionals.professional_service_links (tenant_id);
CREATE INDEX IF NOT EXISTS idx_intakes_tenant_id
  ON professionals.intakes (tenant_id);
CREATE INDEX IF NOT EXISTS idx_intakes_tenant_status
  ON professionals.intakes (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id
  ON professionals.sessions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_status
  ON professionals.sessions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_session_notes_tenant_id
  ON professionals.session_notes (tenant_id);

CREATE INDEX IF NOT EXISTS idx_service_links_tenant_service_id
  ON professionals.professional_service_links (tenant_id, service_id)
  WHERE service_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_intakes_tenant_service_id
  ON professionals.intakes (tenant_id, service_id)
  WHERE service_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_service_id
  ON professionals.sessions (tenant_id, service_id)
  WHERE service_id IS NOT NULL;

DROP INDEX IF EXISTS professionals.idx_professional_profiles_deleted_at;
DROP INDEX IF EXISTS professionals.idx_specialties_deleted_at;
DROP INDEX IF EXISTS professionals.idx_intakes_deleted_at;
DROP INDEX IF EXISTS professionals.idx_sessions_deleted_at;

CREATE INDEX IF NOT EXISTS idx_professional_profiles_deleted_at
  ON professionals.professional_profiles (tenant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_specialties_deleted_at
  ON professionals.specialties (tenant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_intakes_deleted_at
  ON professionals.intakes (tenant_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted_at
  ON professionals.sessions (tenant_id, deleted_at);
