-- 0001_professionals.down.sql

DROP TRIGGER IF EXISTS trg_sessions_updated_at ON professionals.sessions;
DROP TRIGGER IF EXISTS trg_intakes_updated_at ON professionals.intakes;
DROP TRIGGER IF EXISTS trg_professional_service_links_updated_at ON professionals.professional_service_links;
DROP TRIGGER IF EXISTS trg_specialties_updated_at ON professionals.specialties;
DROP TRIGGER IF EXISTS trg_professional_profiles_updated_at ON professionals.professional_profiles;

DROP TABLE IF EXISTS professionals.session_notes;
DROP TABLE IF EXISTS professionals.sessions;
DROP TABLE IF EXISTS professionals.intakes;
DROP TABLE IF EXISTS professionals.professional_service_links;
DROP TABLE IF EXISTS professionals.professional_specialties;
DROP TABLE IF EXISTS professionals.specialties;
DROP TABLE IF EXISTS professionals.professional_profiles;

DROP SCHEMA IF EXISTS professionals;
