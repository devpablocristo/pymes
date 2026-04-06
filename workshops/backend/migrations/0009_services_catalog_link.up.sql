ALTER TABLE workshops.services
    ADD COLUMN IF NOT EXISTS linked_service_id uuid;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'catalog_services'
          AND column_name = 'org_id'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'workshops_services_linked_service_fk'
        ) THEN
            ALTER TABLE workshops.services
                ADD CONSTRAINT workshops_services_linked_service_fk
                FOREIGN KEY (linked_service_id) REFERENCES public.catalog_services(id) ON DELETE SET NULL;
        END IF;

        UPDATE workshops.services ws
        SET linked_service_id = s.id
        FROM public.catalog_services s
        WHERE ws.linked_product_id = s.id
          AND ws.org_id = s.org_id
          AND ws.linked_service_id IS NULL;
    ELSIF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'services'
          AND column_name = 'org_id'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'workshops_services_linked_service_fk'
        ) THEN
            ALTER TABLE workshops.services
                ADD CONSTRAINT workshops_services_linked_service_fk
                FOREIGN KEY (linked_service_id) REFERENCES public.services(id) ON DELETE SET NULL;
        END IF;

        UPDATE workshops.services ws
        SET linked_service_id = s.id
        FROM public.services s
        WHERE ws.linked_product_id = s.id
          AND ws.org_id = s.org_id
          AND ws.linked_service_id IS NULL;
    ELSE
        RAISE EXCEPTION 'commercial services catalog not found';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_workshops_services_linked_service
    ON workshops.services(org_id, linked_service_id)
    WHERE linked_service_id IS NOT NULL;
