-- Archivo lógico de servicios de taller (paridad CRUD / consola con clientes y vehículos).
DROP INDEX IF EXISTS workshops.workshops_services_org_code_idx;

ALTER TABLE workshops.services
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_services_org_code_segment_active_idx
    ON workshops.services (org_id, code, segment)
    WHERE archived_at IS NULL;
