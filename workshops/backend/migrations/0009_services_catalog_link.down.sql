DROP INDEX IF EXISTS idx_workshops_services_linked_service;

ALTER TABLE workshops.services
    DROP COLUMN IF EXISTS linked_service_id;
