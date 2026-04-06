ALTER TABLE professionals.professional_service_links
    ADD COLUMN IF NOT EXISTS service_id UUID;

CREATE INDEX IF NOT EXISTS idx_service_links_org_service_id
    ON professionals.professional_service_links (org_id, service_id)
    WHERE service_id IS NOT NULL;

UPDATE professionals.professional_service_links
SET service_id = product_id
WHERE service_id IS NULL
  AND product_id IS NOT NULL;

ALTER TABLE professionals.intakes
    ADD COLUMN IF NOT EXISTS service_id UUID;

CREATE INDEX IF NOT EXISTS idx_intakes_org_service_id
    ON professionals.intakes (org_id, service_id)
    WHERE service_id IS NOT NULL;

UPDATE professionals.intakes
SET service_id = product_id
WHERE service_id IS NULL
  AND product_id IS NOT NULL;

ALTER TABLE professionals.sessions
    ADD COLUMN IF NOT EXISTS service_id UUID;

CREATE INDEX IF NOT EXISTS idx_sessions_org_service_id
    ON professionals.sessions (org_id, service_id)
    WHERE service_id IS NOT NULL;

UPDATE professionals.sessions
SET service_id = product_id
WHERE service_id IS NULL
  AND product_id IS NOT NULL;
