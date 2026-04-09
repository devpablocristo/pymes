-- Eventos internos de agenda: reuniones, tareas personales, capacitaciones,
-- bloqueos con título, etc. NO son turnos de cliente (eso vive en
-- scheduling_bookings). Sólo se manipulan desde la consola interna; nunca se
-- exponen en la surface pública /v1/public/...
CREATE TABLE IF NOT EXISTS scheduling_calendar_events (
    id           uuid PRIMARY KEY,
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id    uuid REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    resource_id  uuid REFERENCES scheduling_resources(id) ON DELETE SET NULL,
    title        text NOT NULL,
    description  text NOT NULL DEFAULT '',
    start_at     timestamptz NOT NULL,
    end_at       timestamptz NOT NULL,
    all_day      boolean NOT NULL DEFAULT false,
    -- 'scheduled' | 'done' | 'cancelled'
    status       text NOT NULL DEFAULT 'scheduled',
    -- 'private' (solo creador) | 'team' (toda la org)
    visibility   text NOT NULL DEFAULT 'team',
    created_by   text NOT NULL DEFAULT '',
    metadata     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scheduling_calendar_events_range_check CHECK (start_at < end_at),
    CONSTRAINT scheduling_calendar_events_status_check CHECK (status IN ('scheduled', 'done', 'cancelled')),
    CONSTRAINT scheduling_calendar_events_visibility_check CHECK (visibility IN ('private', 'team'))
);

CREATE INDEX IF NOT EXISTS idx_scheduling_calendar_events_org_start
    ON scheduling_calendar_events (org_id, start_at);

CREATE INDEX IF NOT EXISTS idx_scheduling_calendar_events_branch_start
    ON scheduling_calendar_events (org_id, branch_id, start_at)
    WHERE branch_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_scheduling_calendar_events_resource_window
    ON scheduling_calendar_events (org_id, resource_id, start_at, end_at)
    WHERE resource_id IS NOT NULL;
