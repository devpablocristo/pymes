-- Demo restaurants: áreas y mesas del salón.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    area_main uuid;
    area_terrace uuid;
    tbl_m1 uuid;
    tbl_m2 uuid;
    tbl_t1 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    area_main := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/main');
    area_terrace := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/terrace');
    tbl_m1 := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-1');
    tbl_m2 := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-2');
    tbl_t1 := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/terrace-1');

    INSERT INTO restaurant.dining_areas (id, org_id, name, sort_order)
    VALUES
        (area_main, v_org, 'Salón principal', 0),
        (area_terrace, v_org, 'Terraza', 1)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO restaurant.dining_tables (id, org_id, area_id, code, label, capacity, status, notes)
    VALUES
        (tbl_m1, v_org, area_main, 'M-01', 'Ventana', 4, 'available', 'Mesa pegada a la ventana'),
        (tbl_m2, v_org, area_main, 'M-02', 'Centro', 6, 'occupied', 'Mesa grande del centro'),
        (tbl_t1, v_org, area_terrace, 'T-01', 'Sombra', 2, 'reserved', 'Mesa reservada al aire libre')
    ON CONFLICT (id) DO NOTHING;
END $$;
