ALTER TABLE professionals.professional_service_links
    ADD CONSTRAINT chk_professional_service_links_service_id_required
    CHECK (service_id IS NOT NULL) NOT VALID;

ALTER TABLE professionals.professional_service_links
    VALIDATE CONSTRAINT chk_professional_service_links_service_id_required;

ALTER TABLE professionals.professional_service_links
    DROP COLUMN IF EXISTS product_id;

ALTER TABLE professionals.intakes
    DROP COLUMN IF EXISTS product_id;

ALTER TABLE professionals.sessions
    DROP COLUMN IF EXISTS product_id;
