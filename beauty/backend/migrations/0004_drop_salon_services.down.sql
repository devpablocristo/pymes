CREATE TABLE IF NOT EXISTS beauty.salon_services (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    duration_minutes INTEGER NOT NULL DEFAULT 30,
    base_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'ARS',
    tax_rate DOUBLE PRECISION NOT NULL DEFAULT 21,
    is_active BOOLEAN NOT NULL DEFAULT true,
    linked_service_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS beauty_salon_services_org_code_idx
    ON beauty.salon_services (org_id, code);

ALTER TABLE beauty.salon_services
    ADD CONSTRAINT beauty_salon_services_linked_service_fk
    FOREIGN KEY (linked_service_id) REFERENCES public.services(id) ON DELETE SET NULL;
