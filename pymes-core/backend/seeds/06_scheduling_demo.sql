DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    v_branch uuid := uuid_generate_v5(v_org, 'pymes-scheduling/v1/branch/central');
    v_service uuid := uuid_generate_v5(v_org, 'pymes-scheduling/v1/service/consulta');
    v_resource uuid := uuid_generate_v5(v_org, 'pymes-scheduling/v1/resource/profesional-1');
    v_queue uuid := uuid_generate_v5(v_org, 'pymes-scheduling/v1/queue/recepcion');
    v_rule_base text := 'pymes-scheduling/v1/rule/';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO scheduling_branches (id, org_id, code, name, timezone, address, active)
    VALUES (v_branch, v_org, 'central', 'Sucursal Central', 'America/Argentina/Tucuman', 'Casa central', true)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_services (
        id, org_id, code, name, description, fulfillment_mode,
        default_duration_minutes, buffer_before_minutes, buffer_after_minutes,
        slot_granularity_minutes, max_concurrent_bookings, active
    )
    VALUES (
        v_service, v_org, 'consulta', 'Consulta General', 'Servicio demo para agenda',
        'hybrid', 30, 0, 10, 30, 1, true
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_resources (id, org_id, branch_id, code, name, kind, capacity, timezone, active)
    VALUES (v_resource, v_org, v_branch, 'profesional_1', 'Profesional Demo', 'professional', 1, 'America/Argentina/Tucuman', true)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_service_resources (service_id, resource_id)
    VALUES (v_service, v_resource)
    ON CONFLICT (service_id, resource_id) DO NOTHING;

    INSERT INTO scheduling_queues (
        id, org_id, branch_id, service_id, code, name, status, strategy,
        ticket_prefix, last_issued_number, avg_service_seconds, allow_remote_join
    )
    VALUES (
        v_queue, v_org, v_branch, v_service, 'recepcion', 'Recepción General',
        'active', 'fifo', 'R', 0, 600, true
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_availability_rules (
        id, org_id, branch_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active
    )
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'branch/' || gs::text),
        v_org,
        v_branch,
        'branch',
        gs,
        '09:00',
        '18:00',
        30,
        true
    FROM generate_series(1, 5) AS gs
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_availability_rules (
        id, org_id, branch_id, resource_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active
    )
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'resource/' || gs::text),
        v_org,
        v_branch,
        v_resource,
        'resource',
        gs,
        '09:00',
        '17:00',
        30,
        true
    FROM generate_series(1, 5) AS gs
    ON CONFLICT (id) DO NOTHING;
END $$;
