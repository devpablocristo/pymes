DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    v_branch uuid;
    v_service uuid;
    v_resource uuid;
    v_queue uuid;
    v_today date := CURRENT_DATE;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    SELECT id INTO v_branch FROM scheduling_branches WHERE org_id = v_org AND code = 'central' LIMIT 1;
    SELECT id INTO v_service FROM scheduling_services WHERE org_id = v_org AND code = 'general_consultation' LIMIT 1;
    SELECT id INTO v_resource FROM scheduling_resources WHERE org_id = v_org AND code = 'professional_1' LIMIT 1;
    SELECT id INTO v_queue FROM scheduling_queues WHERE org_id = v_org AND code = 'frontdesk' LIMIT 1;

    IF v_branch IS NULL OR v_service IS NULL OR v_resource IS NULL THEN
        RETURN;
    END IF;

    -- Weekend availability rules (sun=0, sat=6) so the calendar is usable any day for demo.
    INSERT INTO scheduling_availability_rules (id, org_id, branch_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active)
    SELECT uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/' || gs::text), v_org, v_branch, 'branch', gs, '09:00', '18:00', 30, true
    FROM unnest(ARRAY[0,6]) AS gs
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_availability_rules (id, org_id, branch_id, resource_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active)
    SELECT uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/' || gs::text), v_org, v_branch, v_resource, 'resource', gs, '09:00', '17:00', 30, true
    FROM unnest(ARRAY[0,6]) AS gs
    ON CONFLICT (id) DO NOTHING;

    -- Sample bookings: today + next 2 days at varied hours.
    INSERT INTO scheduling_bookings (
        id, org_id, branch_id, service_id, resource_id, reference,
        customer_name, customer_phone, status, source,
        start_at, end_at, occupies_from, occupies_until, created_by, created_at, updated_at
    )
    VALUES
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-1'), v_org, v_branch, v_service, v_resource, 'DEMO-001',
       'Juan Pérez', '+5491111111111', 'confirmed', 'admin',
       (v_today + time '10:00') AT TIME ZONE 'America/Argentina/Tucuman',
       (v_today + time '10:30') AT TIME ZONE 'America/Argentina/Tucuman',
       (v_today + time '10:00') AT TIME ZONE 'America/Argentina/Tucuman',
       (v_today + time '10:30') AT TIME ZONE 'America/Argentina/Tucuman',
       'seed', NOW(), NOW()),
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-2'), v_org, v_branch, v_service, v_resource, 'DEMO-002',
       'María López', '+5491122222222', 'confirmed', 'admin',
       (v_today + time '14:00') AT TIME ZONE 'America/Argentina/Tucuman',
       (v_today + time '14:30') AT TIME ZONE 'America/Argentina/Tucuman',
       (v_today + time '14:00') AT TIME ZONE 'America/Argentina/Tucuman',
       (v_today + time '14:30') AT TIME ZONE 'America/Argentina/Tucuman',
       'seed', NOW(), NOW()),
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-3'), v_org, v_branch, v_service, v_resource, 'DEMO-003',
       'Carlos Gómez', '+5491133333333', 'pending_confirmation', 'admin',
       ((v_today + 1) + time '11:00') AT TIME ZONE 'America/Argentina/Tucuman',
       ((v_today + 1) + time '11:30') AT TIME ZONE 'America/Argentina/Tucuman',
       ((v_today + 1) + time '11:00') AT TIME ZONE 'America/Argentina/Tucuman',
       ((v_today + 1) + time '11:30') AT TIME ZONE 'America/Argentina/Tucuman',
       'seed', NOW(), NOW()),
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-4'), v_org, v_branch, v_service, v_resource, 'DEMO-004',
       'Ana Martínez', '+5491144444444', 'confirmed', 'admin',
       ((v_today + 2) + time '15:30') AT TIME ZONE 'America/Argentina/Tucuman',
       ((v_today + 2) + time '16:00') AT TIME ZONE 'America/Argentina/Tucuman',
       ((v_today + 2) + time '15:30') AT TIME ZONE 'America/Argentina/Tucuman',
       ((v_today + 2) + time '16:00') AT TIME ZONE 'America/Argentina/Tucuman',
       'seed', NOW(), NOW())
    ON CONFLICT (id) DO NOTHING;

    -- Make sure a queue exists; create one if missing (some envs were seeded before queue insert was added).
    IF v_queue IS NULL THEN
        v_queue := uuid_generate_v5(v_org, 'modules-scheduling/v1/queue/frontdesk');
        INSERT INTO scheduling_queues (
            id, org_id, branch_id, service_id, code, name, status, strategy,
            ticket_prefix, last_issued_number, avg_service_seconds, allow_remote_join
        )
        VALUES (
            v_queue, v_org, v_branch, v_service, 'frontdesk', 'Front Desk',
            'active', 'fifo', 'FD', 0, 600, true
        )
        ON CONFLICT (id) DO NOTHING;
    END IF;

    -- Sample queue tickets (waiting). Sources allowed: reception, web, whatsapp, api.
    INSERT INTO scheduling_queue_tickets (
        id, org_id, queue_id, branch_id, service_id,
        customer_name, customer_phone, number, display_code, status, priority, source,
        requested_at, created_at, updated_at
    )
    VALUES
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-1'), v_org, v_queue, v_branch, v_service,
       'Pedro Sánchez', '+5491155555555', 1, 'FD-001', 'waiting', 0, 'reception',
       NOW() - interval '15 min', NOW() - interval '15 min', NOW() - interval '15 min'),
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-2'), v_org, v_queue, v_branch, v_service,
       'Lucía Fernández', '+5491166666666', 2, 'FD-002', 'waiting', 0, 'reception',
       NOW() - interval '8 min', NOW() - interval '8 min', NOW() - interval '8 min'),
      (uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-3'), v_org, v_queue, v_branch, v_service,
       'Roberto Díaz', '+5491177777777', 3, 'FD-003', 'waiting', 0, 'reception',
       NOW() - interval '3 min', NOW() - interval '3 min', NOW() - interval '3 min')
    ON CONFLICT (id) DO NOTHING;

    UPDATE scheduling_queues SET last_issued_number = GREATEST(last_issued_number, 3) WHERE id = v_queue;
END $$;
