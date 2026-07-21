ALTER TABLE beauty.salon_services
    ADD COLUMN IF NOT EXISTS linked_product_id uuid NULL;

UPDATE beauty.salon_services
SET linked_product_id = linked_service_id
WHERE linked_service_id IS NOT NULL
  AND linked_product_id IS NULL;
