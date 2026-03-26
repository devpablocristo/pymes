DROP INDEX IF EXISTS workshops.workshops_vehicles_org_plate_active_idx;

ALTER TABLE workshops.vehicles
    DROP COLUMN IF EXISTS archived_at;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_vehicles_org_plate_idx
    ON workshops.vehicles (org_id, license_plate);
