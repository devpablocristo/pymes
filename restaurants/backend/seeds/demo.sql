-- Demo restaurants: 10 areas, 10 mesas y 10 sesiones del salon.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO restaurant.dining_areas (
        id, org_id, name, sort_order, is_favorite, tags, metadata, updated_at
    )
    SELECT
        CASE gs
            WHEN 1 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/main')
            WHEN 2 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/terrace')
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/' || gs::text)
        END,
        v_org,
        (ARRAY[
            'Salon principal', 'Terraza', 'Barra', 'Patio', 'VIP',
            'Vereda', 'Reservado', 'Salon alto', 'Deck', 'Eventos'
        ])[gs],
        gs - 1,
        gs IN (1, 5),
        ARRAY['seed', 'restaurant'],
        jsonb_build_object('source', 'seed'),
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            sort_order = EXCLUDED.sort_order,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now();

    INSERT INTO restaurant.dining_tables (
        id, org_id, area_id, code, label, capacity, status, notes,
        is_favorite, tags, metadata, updated_at
    )
    SELECT
        CASE gs
            WHEN 1 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-1')
            WHEN 2 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-2')
            WHEN 3 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/terrace-1')
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/' || gs::text)
        END,
        v_org,
        CASE (((gs - 1) % 10) + 1)
            WHEN 1 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/main')
            WHEN 2 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/terrace')
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/' || (((gs - 1) % 10) + 1)::text)
        END,
        (ARRAY['M-01','M-02','T-01','M-04','M-05','M-06','M-07','M-08','M-09','M-10'])[gs],
        (ARRAY['Ventana','Centro','Barra norte','Patio sombra','VIP 1','Vereda 1','Reservado A','Salon alto','Deck 1','Eventos A'])[gs],
        (ARRAY[4, 6, 2, 4, 8, 2, 6, 4, 4, 10])[gs],
        (ARRAY['available','occupied','reserved','cleaning','available','occupied','reserved','available','cleaning','available'])[gs],
        'Mesa seed ' || gs::text,
        gs IN (5, 10),
        ARRAY['seed', 'restaurant'],
        jsonb_build_object('source', 'seed'),
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (org_id, code) DO UPDATE
        SET area_id = EXCLUDED.area_id,
            label = EXCLUDED.label,
            capacity = EXCLUDED.capacity,
            status = EXCLUDED.status,
            notes = EXCLUDED.notes,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now();

    INSERT INTO restaurant.table_sessions (
        id, org_id, table_id, guest_count, party_label, notes, opened_at, closed_at, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/session/' || gs::text),
        v_org,
        CASE gs
            WHEN 1 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-1')
            WHEN 2 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-2')
            WHEN 3 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/terrace-1')
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/' || gs::text)
        END,
        (ARRAY[2, 4, 2, 3, 6, 2, 5, 4, 3, 8])[gs],
        (ARRAY['Mesa Lopez','Mesa Gomez','Walk-in barra','Reserva patio','Empresa Norte','Pareja vereda','Cumple pequeno','Familia Sur','After office','Evento demo'])[gs],
        'Sesion de mesa seed ' || gs::text,
        now() - ((11 - gs) || ' hours')::interval,
        CASE WHEN gs IN (2, 6) THEN NULL ELSE now() - ((10 - gs) || ' hours')::interval END,
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET table_id = EXCLUDED.table_id,
            guest_count = EXCLUDED.guest_count,
            party_label = EXCLUDED.party_label,
            notes = EXCLUDED.notes,
            opened_at = EXCLUDED.opened_at,
            closed_at = EXCLUDED.closed_at,
            updated_at = now();
END $$;
