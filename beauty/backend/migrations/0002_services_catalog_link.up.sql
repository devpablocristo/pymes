ALTER TABLE beauty.salon_services
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
            WHERE conname = 'beauty_salon_services_linked_service_fk'
        ) THEN
            ALTER TABLE beauty.salon_services
                ADD CONSTRAINT beauty_salon_services_linked_service_fk
                FOREIGN KEY (linked_service_id) REFERENCES public.catalog_services(id) ON DELETE SET NULL;
        END IF;

        UPDATE beauty.salon_services bs
        SET linked_service_id = s.id
        FROM public.catalog_services s
        WHERE bs.linked_product_id = s.id
          AND bs.org_id = s.org_id
          AND bs.linked_service_id IS NULL;
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
            WHERE conname = 'beauty_salon_services_linked_service_fk'
        ) THEN
            ALTER TABLE beauty.salon_services
                ADD CONSTRAINT beauty_salon_services_linked_service_fk
                FOREIGN KEY (linked_service_id) REFERENCES public.services(id) ON DELETE SET NULL;
        END IF;

        UPDATE beauty.salon_services bs
        SET linked_service_id = s.id
        FROM public.services s
        WHERE bs.linked_product_id = s.id
          AND bs.org_id = s.org_id
          AND bs.linked_service_id IS NULL;
    ELSE
        RAISE EXCEPTION 'commercial services catalog not found';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_beauty_salon_services_linked_service
    ON beauty.salon_services(org_id, linked_service_id)
    WHERE linked_service_id IS NOT NULL;
