-- Demo professionals: 10 perfiles, especialidades, intakes y sesiones.
-- Depende de los clientes de pymes-core/seeds.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tenants WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO parties (
        id, tenant_id, party_type, display_name, email, phone, address,
        tax_id, notes, tags, metadata, created_at, updated_at, deleted_at, is_favorite
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/party/' || gs::text),
        v_org,
        'person',
        (ARRAY[
            'Dra. Demo Profesional', 'Dr. Martin Ruiz', 'Lic. Ana Torres', 'Dra. Paula Rivas',
            'Lic. Diego Molina', 'Dra. Sofia Castro', 'Lic. Laura Perez', 'Dr. Nicolas Vera',
            'Dra. Camila Ortiz', 'Lic. Tomas Silva'
        ])[gs],
        'profesional' || gs::text || '@local.dev',
        '+54-11-3000-' || lpad(gs::text, 4, '0'),
        '{}'::jsonb,
        NULL,
        'seed professional',
        ARRAY['demo', 'professional'],
        jsonb_build_object('vertical', 'professionals', 'source', 'seed'),
        now() - make_interval(days => 20 - gs),
        now(),
        NULL,
        gs IN (1, 6)
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET display_name = EXCLUDED.display_name,
            email = EXCLUDED.email,
            phone = EXCLUDED.phone,
            notes = EXCLUDED.notes,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now(),
            deleted_at = NULL,
            is_favorite = EXCLUDED.is_favorite;

    INSERT INTO party_persons (party_id, first_name, last_name)
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/party/' || gs::text),
        split_part(full_name, ' ', 1),
        substring(full_name from position(' ' in full_name) + 1)
    FROM (
        SELECT gs, (ARRAY[
            'Demo Profesional', 'Martin Ruiz', 'Ana Torres', 'Paula Rivas',
            'Diego Molina', 'Sofia Castro', 'Laura Perez', 'Nicolas Vera',
            'Camila Ortiz', 'Tomas Silva'
        ])[gs] AS full_name
        FROM generate_series(1, 10) AS gs
    ) src
    ON CONFLICT (party_id) DO UPDATE
        SET first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name;

    INSERT INTO party_roles (id, party_id, tenant_id, role, is_active, price_list_id, metadata, created_at)
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/role/' || gs::text),
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/party/' || gs::text),
        v_org,
        'professional',
        true,
        NULL::uuid,
        jsonb_build_object('source', 'seed'),
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (party_id, tenant_id, role) DO UPDATE
        SET is_active = EXCLUDED.is_active,
            metadata = EXCLUDED.metadata;

    INSERT INTO professionals.professional_profiles (
        id, tenant_id, party_id, public_slug, bio, headline,
        is_public, is_bookable, accepts_new_clients, is_favorite, tags, metadata, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile/' || gs::text),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/party/' || gs::text),
        'demo-profesional-' || lpad(gs::text, 2, '0'),
        'Perfil profesional seed ' || gs::text,
        (ARRAY[
            'Clinica general', 'Traumatologia', 'Psicologia adultos', 'Pediatria',
            'Nutricion', 'Kinesiologia', 'Fonoaudiologia', 'Odontologia',
            'Dermatologia', 'Coaching ejecutivo'
        ])[gs],
        true,
        gs <> 9,
        gs <> 8,
        gs IN (1, 4),
        ARRAY['demo', 'professional'],
        jsonb_build_object('source', 'seed'),
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            public_slug = EXCLUDED.public_slug,
            bio = EXCLUDED.bio,
            headline = EXCLUDED.headline,
            is_public = EXCLUDED.is_public,
            is_bookable = EXCLUDED.is_bookable,
            accepts_new_clients = EXCLUDED.accepts_new_clients,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now();

    INSERT INTO professionals.specialties (
        id, tenant_id, code, name, description, is_active, is_favorite, tags, metadata, updated_at
    )
    SELECT
        CASE gs
            WHEN 1 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/clinical')
            WHEN 2 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/pediatrics')
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/' || gs::text)
        END,
        v_org,
        (ARRAY['CLINICAL','PEDIATRICS','SPEC-003','SPEC-004','SPEC-005','SPEC-006','SPEC-007','SPEC-008','SPEC-009','SPEC-010'])[gs],
        (ARRAY[
            'Clinica general', 'Traumatologia', 'Psicologia', 'Pediatria', 'Nutricion',
            'Kinesiologia', 'Fonoaudiologia', 'Odontologia', 'Dermatologia', 'Coaching'
        ])[gs],
        'Especialidad seed ' || gs::text,
        gs <> 10,
        gs IN (1, 3),
        ARRAY['demo', 'specialty'],
        jsonb_build_object('source', 'seed'),
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (tenant_id, code) DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            is_active = EXCLUDED.is_active,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            metadata = EXCLUDED.metadata,
            updated_at = now();

    INSERT INTO professionals.professional_specialties (id, tenant_id, profile_id, specialty_id)
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile-specialty/' || gs::text),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile/' || gs::text),
        CASE gs
            WHEN 1 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/clinical')
            WHEN 2 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/pediatrics')
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/' || gs::text)
        END
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET tenant_id = EXCLUDED.tenant_id,
            profile_id = EXCLUDED.profile_id,
            specialty_id = EXCLUDED.specialty_id;

    INSERT INTO professionals.intakes (
        id, tenant_id, booking_id, profile_id, customer_party_id, service_id,
        status, payload, is_favorite, tags, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/intake/' || gs::text),
        v_org,
        NULL::uuid,
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile/' || (((gs - 1) % 10) + 1)::text),
        CASE
            WHEN gs <= 3 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/customer/' || gs::text)
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v2/customer/' || gs::text)
        END,
        NULL::uuid,
        (ARRAY['draft','submitted','reviewed','submitted','draft','submitted','reviewed','submitted','draft','submitted'])[gs],
        jsonb_build_object('reason', 'Consulta seed ' || gs::text, 'source', 'seed'),
        gs IN (2, 6),
        ARRAY['demo', 'intake'],
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET profile_id = EXCLUDED.profile_id,
            customer_party_id = EXCLUDED.customer_party_id,
            service_id = EXCLUDED.service_id,
            status = EXCLUDED.status,
            payload = EXCLUDED.payload,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            updated_at = now();

    INSERT INTO professionals.sessions (
        id, tenant_id, booking_id, profile_id, customer_party_id, service_id,
        status, started_at, ended_at, summary, metadata, updated_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/session/' || gs::text),
        v_org,
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/booking/' || gs::text),
        uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile/' || (((gs - 1) % 10) + 1)::text),
        CASE
            WHEN gs <= 3 THEN uuid_generate_v5(v_org, 'pymes-seed/v1/customer/' || gs::text)
            ELSE uuid_generate_v5(v_org, 'pymes-seed/v2/customer/' || gs::text)
        END,
        NULL::uuid,
        (ARRAY['completed','scheduled','completed','cancelled','completed','scheduled','completed','completed','scheduled','completed'])[gs],
        now() - ((11 - gs) || ' days')::interval,
        CASE WHEN gs IN (2, 6, 9) THEN NULL ELSE now() - ((11 - gs) || ' days')::interval + '45 minutes'::interval END,
        'Sesion profesional seed ' || gs::text,
        jsonb_build_object('source', 'seed'),
        now()
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (tenant_id, booking_id) DO UPDATE
        SET profile_id = EXCLUDED.profile_id,
            customer_party_id = EXCLUDED.customer_party_id,
            service_id = EXCLUDED.service_id,
            status = EXCLUDED.status,
            started_at = EXCLUDED.started_at,
            ended_at = EXCLUDED.ended_at,
            summary = EXCLUDED.summary,
            metadata = EXCLUDED.metadata,
            updated_at = now();
END $$;
