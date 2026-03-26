-- Soft archive para vehículos (paridad con consola / CRUD canónico).
DROP INDEX IF EXISTS workshops.workshops_vehicles_org_plate_idx;

ALTER TABLE workshops.vehicles
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_vehicles_org_plate_active_idx
    ON workshops.vehicles (org_id, license_plate)
    WHERE archived_at IS NULL;
