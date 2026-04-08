-- 0012: Consolidar workshops.services en public.services (catálogo único en core).
-- Migración:
--  1. Para cada workshops.services row, asegurar una public.services equivalente.
--     - Si linked_service_id apunta a public.services existente, reusar.
--     - Si no, crear nueva public.services con id = uuid_generate_v5(workshops.services.id namespace)
--       guardando estimated_hours, segment y vertical en metadata jsonb.
--  2. Reapuntar workshops.work_order_items.service_id y workshops.bike_work_order_items.service_id
--     desde el id de workshops.services al id de public.services.
--  3. Drop tabla workshops.services.

-- Asegurar extension uuid-ossp para uuid_generate_v5.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Namespace estable para derivar uuids de workshops services.
-- (uuid v5 con namespace fijo + workshops.services.id::text)
DO $$
DECLARE
    ns_workshops uuid := '6f6e8c2a-1234-5678-9abc-def012345678';
    rec record;
    new_id uuid;
BEGIN
    -- Paso 1: crear/reusar core.services para cada workshops.services.
    FOR rec IN
        SELECT id, org_id, code, name, description, category, estimated_hours,
               base_price, currency, tax_rate, is_active, segment, archived_at,
               linked_service_id, created_at, updated_at
        FROM workshops.services
    LOOP
        IF rec.linked_service_id IS NOT NULL THEN
            -- Ya tiene link al catálogo core: reutilizar.
            new_id := rec.linked_service_id;
        ELSE
            -- Derivar id determinístico para este workshops.services row.
            new_id := uuid_generate_v5(ns_workshops, rec.id::text);
            INSERT INTO public.services (
                id, org_id, code, name, description, category_code,
                sale_price, cost_price, tax_rate, currency,
                default_duration_minutes, tags, metadata,
                created_at, updated_at, deleted_at, is_active
            )
            VALUES (
                new_id,
                rec.org_id,
                NULLIF(rec.code, ''),
                rec.name,
                rec.description,
                rec.category,
                rec.base_price::numeric(15,2),
                0::numeric(15,2),
                rec.tax_rate::numeric(5,2),
                rec.currency,
                CASE WHEN rec.estimated_hours > 0
                     THEN GREATEST(1, (rec.estimated_hours * 60)::integer)
                     ELSE NULL END,
                ARRAY[]::text[],
                jsonb_build_object(
                    'vertical', 'workshops',
                    'segment', rec.segment,
                    'estimated_hours', rec.estimated_hours,
                    'migrated_from', 'workshops.services',
                    'legacy_id', rec.id::text
                ),
                rec.created_at,
                rec.updated_at,
                rec.archived_at,
                rec.is_active
            )
            ON CONFLICT (id) DO NOTHING;
        END IF;

        -- Paso 2: reapuntar items que referenciaban el id viejo.
        UPDATE workshops.work_order_items
            SET service_id = new_id
            WHERE service_id = rec.id;

        UPDATE workshops.bike_work_order_items
            SET service_id = new_id
            WHERE service_id = rec.id;
    END LOOP;
END $$;

-- Paso 3: drop FK al linked_service_id (ya no aplica) y la tabla legacy.
ALTER TABLE workshops.services DROP CONSTRAINT IF EXISTS workshops_services_linked_service_fk;
DROP TABLE IF EXISTS workshops.services CASCADE;
