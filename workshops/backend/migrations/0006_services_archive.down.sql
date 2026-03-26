DROP INDEX IF EXISTS workshops.workshops_services_org_code_segment_active_idx;

ALTER TABLE workshops.services
    DROP COLUMN IF EXISTS archived_at;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_services_org_code_idx
    ON workshops.services (org_id, code);
