-- Demo professionals: perfil, especialidades, intake y sesión.
-- Depende de 02_core_business (cliente c1 como customer_party).

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    c1 uuid;
    prof_party uuid;
    prof_profile uuid;
    spec1 uuid;
    spec2 uuid;
    intake1 uuid;
    sess1 uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    c1 := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    prof_party := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/party/1');
    prof_profile := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile/1');
    spec1 := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/clinical');
    spec2 := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/pediatrics');
    intake1 := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/intake/1');
    sess1 := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/session/1');

    -- Party del profesional
    INSERT INTO parties (id, org_id, party_type, display_name, email, phone, address, tax_id, notes, tags, metadata, created_at, updated_at, deleted_at)
    VALUES (prof_party, v_org, 'person', 'Dra. Demo Profesional', 'profesional@local.dev', '+54-11-3000-0001', '{}'::jsonb, NULL, 'seed', ARRAY['demo'], jsonb_build_object('vertical', 'professionals'), now(), now(), NULL)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO party_persons (party_id, first_name, last_name)
    VALUES (prof_party, 'Demo', 'Profesional')
    ON CONFLICT (party_id) DO NOTHING;

    INSERT INTO party_roles (id, party_id, org_id, role, is_active, price_list_id, metadata, created_at)
    VALUES (gen_random_uuid(), prof_party, v_org, 'professional', true, NULL::uuid, '{}'::jsonb, now())
    ON CONFLICT (party_id, org_id, role) DO UPDATE SET is_active = EXCLUDED.is_active;

    -- Perfil profesional
    INSERT INTO professionals.professional_profiles (id, org_id, party_id, public_slug, bio, headline, is_public, is_bookable, accepts_new_clients, metadata)
    VALUES (prof_profile, v_org, prof_party, 'demo-profesional', 'Perfil semilla', 'Profesional demo', true, true, true, '{}'::jsonb)
    ON CONFLICT (id) DO NOTHING;

    -- Especialidades
    INSERT INTO professionals.specialties (id, org_id, code, name, description, is_active)
    VALUES
        (spec1, v_org, 'CLINICAL', 'Clínica general', 'Consultas clínicas', true),
        (spec2, v_org, 'PEDIATRICS', 'Pediatría', 'Atención infantil', true)
    ON CONFLICT (org_id, code) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, is_active = EXCLUDED.is_active;

    INSERT INTO professionals.professional_specialties (id, org_id, profile_id, specialty_id)
    VALUES
        (uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile-specialty/1'), v_org, prof_profile, spec1),
        (uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile-specialty/2'), v_org, prof_profile, spec2)
    ON CONFLICT (org_id, profile_id, specialty_id) DO NOTHING;

    -- Intake (formulario previo a la sesión)
    INSERT INTO professionals.intakes (id, org_id, booking_id, profile_id, customer_party_id, service_id, status, payload)
    VALUES (intake1, v_org, NULL, prof_profile, c1, NULL, 'submitted', jsonb_build_object('reason', 'Chequeo anual', 'allergies', 'Ninguna'))
    ON CONFLICT (id) DO NOTHING;

    -- Sesión
    INSERT INTO professionals.sessions (id, org_id, booking_id, profile_id, customer_party_id, service_id, status, started_at, ended_at, summary, metadata)
    VALUES (
        sess1, v_org, uuid_generate_v5(v_org, 'pymes-seed/v1/professional/booking/1'),
        prof_profile, c1, NULL, 'completed',
        now() - interval '1 day', now() - interval '1 day' + interval '45 minutes',
        'Sesión demo finalizada', '{}'::jsonb
    )
    ON CONFLICT (org_id, booking_id) DO NOTHING;
END $$;
