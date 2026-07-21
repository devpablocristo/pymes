DROP INDEX IF EXISTS idx_beauty_salon_services_linked_service;

ALTER TABLE beauty.salon_services
    DROP COLUMN IF EXISTS linked_service_id;
