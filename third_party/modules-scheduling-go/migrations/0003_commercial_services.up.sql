ALTER TABLE scheduling_services
    ADD COLUMN IF NOT EXISTS commercial_service_id uuid REFERENCES services(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_scheduling_services_commercial_service
    ON scheduling_services(commercial_service_id)
    WHERE commercial_service_id IS NOT NULL;

UPDATE scheduling_services ss
SET commercial_service_id = s.id
FROM services s
WHERE ss.org_id = s.org_id
  AND ss.commercial_service_id IS NULL
  AND (
      (ss.code <> '' AND s.code <> '' AND LOWER(ss.code) = LOWER(s.code))
      OR LOWER(ss.name) = LOWER(s.name)
  );
