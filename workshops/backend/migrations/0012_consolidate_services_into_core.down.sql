-- Revertir: recrear workshops.services vacía con la estructura previa.
-- Atención: los datos migrados siguen viviendo en public.services; este down sólo
-- restaura el esquema para permitir rollback de la migración.

CREATE TABLE IF NOT EXISTS workshops.services (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    estimated_hours DOUBLE PRECISION NOT NULL DEFAULT 0,
    base_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'ARS',
    tax_rate DOUBLE PRECISION NOT NULL DEFAULT 21,
    is_active BOOLEAN NOT NULL DEFAULT true,
    segment TEXT NOT NULL DEFAULT 'auto_repair',
    archived_at TIMESTAMPTZ,
    linked_service_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE workshops.services
    ADD CONSTRAINT workshops_services_linked_service_fk
    FOREIGN KEY (linked_service_id) REFERENCES public.services(id) ON DELETE SET NULL;
