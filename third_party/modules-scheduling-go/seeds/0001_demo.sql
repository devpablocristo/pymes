DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    v_branch uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/branch/central');
    v_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_consultation');
    v_resource uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/resource/professional-1');
    v_queue uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/queue/frontdesk');
    v_rule_base text := 'modules-scheduling/v1/rule/';
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO scheduling_branches (id, org_id, code, name, timezone, address, active)
    VALUES (v_branch, v_org, 'central', 'Central Branch', 'America/Argentina/Tucuman', 'Main office', true)
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_services (
        id, org_id, code, name, description, fulfillment_mode,
        default_duration_minutes, buffer_before_minutes, buffer_after_minutes,
        slot_granularity_minutes, max_concurrent_bookings, min_cancel_notice_minutes,
        allow_waitlist, active
    )
    VALUES (
        v_service, v_org, 'general_consultation', 'General Consultation', 'Reusable demo service for scheduling',
        'hybrid', 30, 0, 10, 30, 1, 60, true, true
    )
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_resources (id, org_id, branch_id, code, name, kind, capacity, timezone, active)
    VALUES (v_resource, v_org, v_branch, 'professional_1', 'Demo Professional', 'professional', 1, 'America/Argentina/Tucuman', true)
    ON CONFLICT (id) DO NOTHING;

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
    ON CONFLICT (id) DO NOTHING;

    -- Lunes a viernes (1..5): 09:00–13:00 y 14:00–18:00 (almuerzo cerrado).
    -- Borra IDs del seed anterior (una ventana 09–18 / 09–17) para no duplicar al re-ejecutar.
    DELETE FROM scheduling_availability_rules
    WHERE org_id = v_org
      AND branch_id = v_branch
      AND id IN (
          SELECT uuid_generate_v5(v_org, v_rule_base || 'branch/' || gs::text)
          FROM generate_series(1, 5) AS gs
          UNION ALL
          SELECT uuid_generate_v5(v_org, v_rule_base || 'resource/' || gs::text)
          FROM generate_series(1, 5) AS gs
      );

    INSERT INTO scheduling_availability_rules (
        id, org_id, branch_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active
    )
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'branch/weekday/' || gs::text || '/am'),
        v_org,
        v_branch,
        'branch',
        gs,
        TIME '09:00',
        TIME '13:00',
        30,
        true
    FROM generate_series(1, 5) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'branch/weekday/' || gs::text || '/pm'),
        v_org,
        v_branch,
        'branch',
        gs,
        TIME '14:00',
        TIME '18:00',
        30,
        true
    FROM generate_series(1, 5) AS gs
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO scheduling_availability_rules (
        id, org_id, branch_id, resource_id, kind, weekday, start_time, end_time, slot_granularity_minutes, active
    )
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'resource/weekday/' || gs::text || '/am'),
        v_org,
        v_branch,
        v_resource,
        'resource',
        gs,
        TIME '09:00',
        TIME '13:00',
        30,
        true
    FROM generate_series(1, 5) AS gs
    UNION ALL
    SELECT
        uuid_generate_v5(v_org, v_rule_base || 'resource/weekday/' || gs::text || '/pm'),
        v_org,
        v_branch,
        v_resource,
        'resource',
        gs,
        TIME '14:00',
        TIME '18:00',
        30,
        true
    FROM generate_series(1, 5) AS gs
    ON CONFLICT (id) DO NOTHING;
END $$;
