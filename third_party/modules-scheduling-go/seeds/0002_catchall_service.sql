-- Catch-all service for ad-hoc owner-side bookings.
--
-- Provides a "Turno general" service that the SMB owner uses from the internal
-- calendar to anote any booking whose duration / customer / context does not
-- match a real catalog service. The actual duration is editable from the
-- calendar via the resize/drag flow (custom-duration reschedule).
--
-- Marked with metadata.catchall = true so the public catalog adapter
-- (pymes-core/internal/publicapi) filters it out and clients booking through
-- PublicSchedulingFlow never see it.
--
-- The service is linked to every active resource of the org so the owner can
-- pick any of them when creating an ad-hoc booking. New resources added later
-- will require a manual link if the owner wants the catch-all to cover them.

DO $$
DECLARE
    v_org uuid := '__SEED_ORG_ID__';
    v_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_appointment');
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orgs WHERE id = v_org) THEN
        RETURN;
    END IF;

    INSERT INTO scheduling_services (
        id, org_id, code, name, description, fulfillment_mode,
        default_duration_minutes, buffer_before_minutes, buffer_after_minutes,
        slot_granularity_minutes, max_concurrent_bookings, min_cancel_notice_minutes,
        allow_waitlist, active, metadata
    )
    VALUES (
        v_service, v_org, 'general_appointment', 'Turno general',
        'Servicio comodín para anotar turnos ad-hoc desde el calendario interno. La duración real se ajusta arrastrando el evento.',
        'schedule', 15, 0, 0, 15, 1, 0, false, true,
        '{"catchall": true}'::jsonb
    )
    ON CONFLICT (id) DO NOTHING;

    -- Link the catch-all to every active resource of the org so the owner can
    -- pick any of them. New resources added later need a manual link.
    INSERT INTO scheduling_service_resources (service_id, resource_id)
    SELECT v_service, r.id
    FROM scheduling_resources r
    WHERE r.org_id = v_org AND r.active = true
    ON CONFLICT (service_id, resource_id) DO NOTHING;
END $$;
