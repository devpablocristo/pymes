DROP INDEX IF EXISTS idx_sessions_org_service_id;
ALTER TABLE professionals.sessions
    DROP COLUMN IF EXISTS service_id;

DROP INDEX IF EXISTS idx_intakes_org_service_id;
ALTER TABLE professionals.intakes
    DROP COLUMN IF EXISTS service_id;

DROP INDEX IF EXISTS idx_service_links_org_service_id;
ALTER TABLE professionals.professional_service_links
    DROP COLUMN IF EXISTS service_id;
