DROP INDEX IF EXISTS idx_scheduling_services_commercial_service;

ALTER TABLE scheduling_services
    DROP COLUMN IF EXISTS commercial_service_id;
