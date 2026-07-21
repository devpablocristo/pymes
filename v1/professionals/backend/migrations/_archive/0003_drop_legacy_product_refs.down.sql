ALTER TABLE professionals.professional_service_links
    ADD COLUMN IF NOT EXISTS product_id UUID;

UPDATE professionals.professional_service_links
SET product_id = service_id
WHERE product_id IS NULL
  AND service_id IS NOT NULL;

ALTER TABLE professionals.professional_service_links
    ALTER COLUMN product_id SET NOT NULL;

ALTER TABLE professionals.intakes
    ADD COLUMN IF NOT EXISTS product_id UUID;

UPDATE professionals.intakes
SET product_id = service_id
WHERE product_id IS NULL
  AND service_id IS NOT NULL;

ALTER TABLE professionals.sessions
    ADD COLUMN IF NOT EXISTS product_id UUID;

UPDATE professionals.sessions
SET product_id = service_id
WHERE product_id IS NULL
  AND service_id IS NOT NULL;

ALTER TABLE professionals.professional_service_links
    DROP CONSTRAINT IF EXISTS chk_professional_service_links_service_id_required;
