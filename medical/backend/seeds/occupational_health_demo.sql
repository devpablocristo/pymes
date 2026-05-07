-- Demo medical: 10 examenes laborales visibles para la subvertical medicina laboral.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tenants WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO medical.occupational_health_exams (
        id,
        tenant_id,
        patient_name,
        patient_document,
        employer_name,
        client_name,
        payment_method,
        exam_type,
        status,
        scheduled_at,
        completed_at,
        result,
        notes,
        is_favorite,
        tags,
        image_urls,
        created_by,
        updated_by,
        created_at,
        updated_at,
        deleted_at
    )
    SELECT
        uuid_generate_v5(v_org, 'pymes-seed/v1/medical/occupational-health/exam/' || gs::text),
        v_org,
        CASE gs
            WHEN 1 THEN 'Carla Benitez'
            WHEN 2 THEN 'Martin Peralta'
            WHEN 3 THEN 'Sofia Ibarra'
            WHEN 4 THEN 'Diego Herrera'
            WHEN 5 THEN 'Paula Medina'
            WHEN 6 THEN 'Luciano Rojas'
            WHEN 7 THEN 'Valentina Castro'
            WHEN 8 THEN 'Nicolas Pereyra'
            WHEN 9 THEN 'Camila Suarez'
            ELSE 'Federico Molina'
        END,
        '20' || lpad((34000000 + gs * 137)::text, 8, '0'),
        CASE ((gs - 1) % 5) + 1
            WHEN 1 THEN 'Transportes Norte SRL'
            WHEN 2 THEN 'Metalurgica Avenida'
            WHEN 3 THEN 'Clinica San Martin'
            WHEN 4 THEN 'Logistica del Sur'
            ELSE 'Supermercados Plaza'
        END,
        CASE ((gs - 1) % 5) + 1
            WHEN 1 THEN 'Transportes Norte SRL'
            WHEN 2 THEN 'Metalurgica Avenida'
            WHEN 3 THEN 'Clinica San Martin'
            WHEN 4 THEN 'Logistica del Sur'
            ELSE 'Supermercados Plaza'
        END,
        CASE ((gs - 1) % 4) + 1
            WHEN 1 THEN 'cash'
            WHEN 2 THEN 'transfer'
            WHEN 3 THEN 'card'
            ELSE 'mixed'
        END,
        CASE ((gs - 1) % 5) + 1
            WHEN 1 THEN 'pre_employment'
            WHEN 2 THEN 'periodic'
            WHEN 3 THEN 'return_to_work'
            WHEN 4 THEN 'exit'
            ELSE 'other'
        END,
        CASE
            WHEN gs IN (1, 6) THEN 'pending'
            WHEN gs IN (2, 3, 7, 8) THEN 'scheduled'
            WHEN gs IN (4, 5, 9) THEN 'completed'
            ELSE 'cancelled'
        END,
        CASE
            WHEN gs IN (1, 6) THEN NULL
            ELSE (CURRENT_DATE + ((gs - 3) * interval '1 day') + make_interval(hours => 8 + gs)) AT TIME ZONE 'America/Argentina/Buenos_Aires'
        END,
        CASE
            WHEN gs IN (4, 5, 9) THEN (CURRENT_DATE - ((10 - gs) * interval '1 day') + time '17:00') AT TIME ZONE 'America/Argentina/Buenos_Aires'
            ELSE NULL
        END,
        CASE
            WHEN gs IN (4, 5, 9) THEN CASE WHEN gs = 9 THEN 'Observado' ELSE 'Apto' END
            ELSE ''
        END,
        CASE
            WHEN gs IN (1, 6) THEN 'Pendiente de coordinar turno con la empresa.'
            WHEN gs IN (2, 3, 7, 8) THEN 'Turno programado por medicina laboral.'
            WHEN gs IN (4, 5) THEN 'Examen finalizado sin observaciones.'
            WHEN gs = 9 THEN 'Requiere control complementario.'
            ELSE 'Cancelado por reprogramacion de la empresa.'
        END,
        gs IN (1, 4, 7),
        ARRAY[
            'medicina-laboral',
            CASE
                WHEN gs IN (1, 6) THEN 'pendiente'
                WHEN gs IN (2, 3, 7, 8) THEN 'agendado'
                WHEN gs IN (4, 5, 9) THEN 'completo'
                ELSE 'cancelado'
            END,
            CASE ((gs - 1) % 5) + 1
                WHEN 1 THEN 'preocupacional'
                WHEN 2 THEN 'periodico'
                WHEN 3 THEN 'reintegro'
                WHEN 4 THEN 'egreso'
                ELSE 'otro'
            END
        ]::text[],
        ARRAY[
            'https://picsum.photos/seed/pymes-medical-exam-' || gs::text || '/900/600'
        ]::text[],
        'seed',
        'seed',
        now() - (gs || ' days')::interval,
        now(),
        NULL
    FROM generate_series(1, 10) AS gs
    ON CONFLICT (id) DO UPDATE
        SET patient_name = EXCLUDED.patient_name,
            patient_document = EXCLUDED.patient_document,
            employer_name = EXCLUDED.employer_name,
            client_name = EXCLUDED.client_name,
            payment_method = EXCLUDED.payment_method,
            exam_type = EXCLUDED.exam_type,
            status = EXCLUDED.status,
            scheduled_at = EXCLUDED.scheduled_at,
            completed_at = EXCLUDED.completed_at,
            result = EXCLUDED.result,
            notes = EXCLUDED.notes,
            is_favorite = EXCLUDED.is_favorite,
            tags = EXCLUDED.tags,
            image_urls = EXCLUDED.image_urls,
            updated_by = EXCLUDED.updated_by,
            updated_at = now(),
            deleted_at = NULL;
END $$;
