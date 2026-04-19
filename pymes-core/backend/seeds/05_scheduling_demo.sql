-- Seed scheduling local reutilizable sin depender del repo ../modules.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    v_branch uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/branch/central');
    v_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_consultation');
    v_catchall uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_appointment');
    v_resource uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/resource/professional-1');
    v_queue uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/queue/frontdesk');
    v_today date := CURRENT_DATE;
    v_rule_base text := 'modules-scheduling/v1/rule/';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO scheduling_branches (id, org_id, code, name, timezone, address, active)
    VALUES (v_branch, v_org, 'central', 'Sucursal Central', 'America/Argentina/Tucuman', 'Casa central demo', true)
    ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            timezone = EXCLUDED.timezone,
            address = EXCLUDED.address,
            active = EXCLUDED.active,
            updated_at = now();

    INSERT INTO scheduling_services (
        id, org_id, code, name, description, fulfillment_mode,
        default_duration_minutes, buffer_before_minutes, buffer_after_minutes,
        slot_granularity_minutes, max_concurrent_bookings, min_cancel_notice_minutes,
        allow_waitlist, active
    )
    VALUES (
        v_service, v_org, 'general_consultation', 'Consulta general', 'Servicio demo reutilizable para agenda',
        'hybrid', 30, 0, 10, 30, 1, 60, true, true
    )
    ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            fulfillment_mode = EXCLUDED.fulfillment_mode,
            default_duration_minutes = EXCLUDED.default_duration_minutes,
            buffer_before_minutes = EXCLUDED.buffer_before_minutes,
            buffer_after_minutes = EXCLUDED.buffer_after_minutes,
            slot_granularity_minutes = EXCLUDED.slot_granularity_minutes,
            max_concurrent_bookings = EXCLUDED.max_concurrent_bookings,
            min_cancel_notice_minutes = EXCLUDED.min_cancel_notice_minutes,
            allow_waitlist = EXCLUDED.allow_waitlist,
            active = EXCLUDED.active,
            updated_at = now();

    INSERT INTO scheduling_resources (id, org_id, branch_id, code, name, kind, capacity, timezone, active)
    VALUES (v_resource, v_org, v_branch, 'professional_1', 'Profesional Demo', 'professional', 1, 'America/Argentina/Tucuman', true)
    ON CONFLICT (id) DO UPDATE
        SET branch_id = EXCLUDED.branch_id,
            name = EXCLUDED.name,
            kind = EXCLUDED.kind,
            capacity = EXCLUDED.capacity,
            timezone = EXCLUDED.timezone,
            active = EXCLUDED.active,
            updated_at = now();

    INSERT INTO scheduling_service_resources (service_id, resource_id)
    VALUES (v_service, v_resource)
    ON CONFLICT (service_id, resource_id) DO NOTHING;

    INSERT INTO scheduling_queues (
        id, org_id, branch_id, service_id, code, name, status, strategy,
        ticket_prefix, last_issued_number, avg_service_seconds, allow_remote_join
    )
    VALUES (
        v_queue, v_org, v_branch, v_service, 'frontdesk', 'Front Desk',
        'active', 'fifo', 'FD', 0, 600, true
    )
    ON CONFLICT (id) DO UPDATE
        SET branch_id = EXCLUDED.branch_id,
            service_id = EXCLUDED.service_id,
            name = EXCLUDED.name,
            status = EXCLUDED.status,
            strategy = EXCLUDED.strategy,
            ticket_prefix = EXCLUDED.ticket_prefix,
            avg_service_seconds = EXCLUDED.avg_service_seconds,
            allow_remote_join = EXCLUDED.allow_remote_join,
            updated_at = now();

    DELETE FROM scheduling_availability_rules
    WHERE org_id = v_org
      AND (
        branch_id = v_branch
        OR resource_id = v_resource
      );

    INSERT INTO scheduling_availability_rules (
        id, org_id, branch_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active
    )
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'branch/weekday/' || gs::text || '/am'),
        v_org, v_branch, 'branch', gs, TIME '09:00', TIME '13:00', 30, true
    FROM generate_series(1, 5) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'branch/weekday/' || gs::text || '/pm'),
        v_org, v_branch, 'branch', gs, TIME '14:00', TIME '18:00', 30, true
    FROM generate_series(1, 5) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'branch/' || gs::text),
        v_org, v_branch, 'branch', gs, TIME '09:00', TIME '18:00', 30, true
    FROM unnest(ARRAY[0,6]) AS gs
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_availability_rules (
        id, org_id, branch_id, resource_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active
    )
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'resource/weekday/' || gs::text || '/am'),
        v_org, v_branch, v_resource, 'resource', gs, TIME '09:00', TIME '13:00', 30, true
    FROM generate_series(1, 5) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'resource/weekday/' || gs::text || '/pm'),
        v_org, v_branch, v_resource, 'resource', gs, TIME '14:00', TIME '18:00', 30, true
    FROM generate_series(1, 5) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'resource/' || gs::text),
        v_org, v_branch, v_resource, 'resource', gs, TIME '09:00', TIME '17:00', 30, true
    FROM unnest(ARRAY[0,6]) AS gs
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_services (
        id, org_id, code, name, description, fulfillment_mode,
        default_duration_minutes, buffer_before_minutes, buffer_after_minutes,
        slot_granularity_minutes, max_concurrent_bookings, min_cancel_notice_minutes,
        allow_waitlist, active, metadata
    )
    VALUES (
        v_catchall, v_org, 'general_appointment', 'Turno general',
        'Servicio comodín para anotar turnos ad-hoc desde el calendario interno.',
        'schedule', 15, 0, 0, 15, 1, 0, false, true, '{"catchall": true}'::jsonb
    )
    ON CONFLICT (id) DO UPDATE
        SET name = EXCLUDED.name,
            description = EXCLUDED.description,
            metadata = EXCLUDED.metadata,
            updated_at = now();

    INSERT INTO scheduling_service_resources (service_id, resource_id)
    VALUES (v_catchall, v_resource)
    ON CONFLICT (service_id, resource_id) DO NOTHING;

    INSERT INTO scheduling_bookings (
        id, org_id, branch_id, service_id, resource_id, party_id, reference,
        customer_name, customer_phone, customer_email, status, source,
        start_at, end_at, occupies_from, occupies_until, notes, metadata,
        created_by, confirmed_at, created_at, updated_at
    )
    VALUES
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-1'),
        v_org, v_branch, v_service, v_resource, uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1'),
        'DEMO-001', 'Juan Pérez', '+5491111111111', 'juan@local.dev', 'confirmed', 'admin',
        (v_today + time '10:00') AT TIME ZONE 'America/Argentina/Tucuman',
        (v_today + time '10:30') AT TIME ZONE 'America/Argentina/Tucuman',
        (v_today + time '10:00') AT TIME ZONE 'America/Argentina/Tucuman',
        (v_today + time '10:30') AT TIME ZONE 'America/Argentina/Tucuman',
        'Turno demo confirmado', '{}'::jsonb, 'seed', now() - interval '1 day', now(), now()
      ),
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-2'),
        v_org, v_branch, v_service, v_resource, NULL,
        'DEMO-002', 'María López', '+5491122222222', 'maria@local.dev', 'confirmed', 'admin',
        (v_today + time '14:00') AT TIME ZONE 'America/Argentina/Tucuman',
        (v_today + time '14:30') AT TIME ZONE 'America/Argentina/Tucuman',
        (v_today + time '14:00') AT TIME ZONE 'America/Argentina/Tucuman',
        (v_today + time '14:30') AT TIME ZONE 'America/Argentina/Tucuman',
        'Turno demo confirmado', '{}'::jsonb, 'seed', now() - interval '1 day', now(), now()
      ),
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-3'),
        v_org, v_branch, v_service, v_resource, NULL,
        'DEMO-003', 'Carlos Gómez', '+5491133333333', 'carlos@local.dev', 'pending_confirmation', 'admin',
        ((v_today + 1) + time '11:00') AT TIME ZONE 'America/Argentina/Tucuman',
        ((v_today + 1) + time '11:30') AT TIME ZONE 'America/Argentina/Tucuman',
        ((v_today + 1) + time '11:00') AT TIME ZONE 'America/Argentina/Tucuman',
        ((v_today + 1) + time '11:30') AT TIME ZONE 'America/Argentina/Tucuman',
        'Turno demo pendiente', '{}'::jsonb, 'seed', NULL, now(), now()
      ),
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/booking/demo-4'),
        v_org, v_branch, v_service, v_resource, NULL,
        'DEMO-004', 'Ana Martínez', '+5491144444444', 'ana@local.dev', 'confirmed', 'admin',
        ((v_today + 2) + time '15:30') AT TIME ZONE 'America/Argentina/Tucuman',
        ((v_today + 2) + time '16:00') AT TIME ZONE 'America/Argentina/Tucuman',
        ((v_today + 2) + time '15:30') AT TIME ZONE 'America/Argentina/Tucuman',
        ((v_today + 2) + time '16:00') AT TIME ZONE 'America/Argentina/Tucuman',
        'Turno demo futuro', '{}'::jsonb, 'seed', now(), now(), now()
      )
    ON CONFLICT (id) DO UPDATE
        SET party_id = EXCLUDED.party_id,
            customer_name = EXCLUDED.customer_name,
            customer_phone = EXCLUDED.customer_phone,
            customer_email = EXCLUDED.customer_email,
            status = EXCLUDED.status,
            start_at = EXCLUDED.start_at,
            end_at = EXCLUDED.end_at,
            occupies_from = EXCLUDED.occupies_from,
            occupies_until = EXCLUDED.occupies_until,
            notes = EXCLUDED.notes,
            metadata = EXCLUDED.metadata,
            confirmed_at = EXCLUDED.confirmed_at,
            updated_at = now();

    INSERT INTO scheduling_queue_tickets (
        id, org_id, queue_id, branch_id, service_id,
        customer_name, customer_phone, number, display_code, status, priority, source,
        requested_at, created_at, updated_at
    )
    VALUES
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-1'),
        v_org, v_queue, v_branch, v_service,
        'Pedro Sánchez', '+5491155555555', 1, 'FD-001', 'waiting', 0, 'reception',
        now() - interval '15 minutes', now() - interval '15 minutes', now() - interval '15 minutes'
      ),
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-2'),
        v_org, v_queue, v_branch, v_service,
        'Lucía Fernández', '+5491166666666', 2, 'FD-002', 'waiting', 0, 'reception',
        now() - interval '8 minutes', now() - interval '8 minutes', now() - interval '8 minutes'
      ),
      (
        uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-3'),
        v_org, v_queue, v_branch, v_service,
        'Roberto Díaz', '+5491177777777', 3, 'FD-003', 'waiting', 0, 'reception',
        now() - interval '3 minutes', now() - interval '3 minutes', now() - interval '3 minutes'
      )
    ON CONFLICT (id) DO UPDATE
        SET customer_name = EXCLUDED.customer_name,
            customer_phone = EXCLUDED.customer_phone,
            status = EXCLUDED.status,
            requested_at = EXCLUDED.requested_at,
            updated_at = EXCLUDED.updated_at;

    UPDATE scheduling_queues
       SET last_issued_number = GREATEST(last_issued_number, 3),
           updated_at = now()
     WHERE id = v_queue;
END $$;
