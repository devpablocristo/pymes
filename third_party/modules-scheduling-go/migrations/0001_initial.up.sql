CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS scheduling_branches (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    timezone text NOT NULL,
    address text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, code)
);

CREATE INDEX IF NOT EXISTS idx_scheduling_branches_org_active ON scheduling_branches(org_id, active);

CREATE TABLE IF NOT EXISTS scheduling_services (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    fulfillment_mode text NOT NULL CHECK (fulfillment_mode IN ('schedule', 'queue', 'hybrid')),
    default_duration_minutes integer NOT NULL CHECK (default_duration_minutes > 0),
    buffer_before_minutes integer NOT NULL DEFAULT 0 CHECK (buffer_before_minutes >= 0),
    buffer_after_minutes integer NOT NULL DEFAULT 0 CHECK (buffer_after_minutes >= 0),
    slot_granularity_minutes integer NOT NULL DEFAULT 15 CHECK (slot_granularity_minutes > 0),
    max_concurrent_bookings integer NOT NULL DEFAULT 1 CHECK (max_concurrent_bookings > 0),
    active boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, code)
);

CREATE INDEX IF NOT EXISTS idx_scheduling_services_org_active ON scheduling_services(org_id, active);

CREATE TABLE IF NOT EXISTS scheduling_resources (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('professional', 'desk', 'counter', 'box', 'room', 'generic')),
    capacity integer NOT NULL DEFAULT 1 CHECK (capacity > 0),
    timezone text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, code)
);

CREATE INDEX IF NOT EXISTS idx_scheduling_resources_branch_active ON scheduling_resources(org_id, branch_id, active);

CREATE TABLE IF NOT EXISTS scheduling_service_resources (
    service_id uuid NOT NULL REFERENCES scheduling_services(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES scheduling_resources(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id, resource_id)
);

CREATE INDEX IF NOT EXISTS idx_scheduling_service_resources_resource ON scheduling_service_resources(resource_id);

CREATE TABLE IF NOT EXISTS scheduling_availability_rules (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    resource_id uuid REFERENCES scheduling_resources(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('branch', 'resource')),
    weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_time time NOT NULL,
    end_time time NOT NULL,
    slot_granularity_minutes integer CHECK (slot_granularity_minutes > 0),
    valid_from date,
    valid_until date,
    active boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (start_time < end_time),
    CHECK (
        (kind = 'branch' AND resource_id IS NULL) OR
        (kind = 'resource' AND resource_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_scheduling_availability_rules_scope ON scheduling_availability_rules(org_id, branch_id, weekday, active);
CREATE INDEX IF NOT EXISTS idx_scheduling_availability_rules_resource ON scheduling_availability_rules(resource_id, weekday) WHERE resource_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS scheduling_blocked_ranges (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    resource_id uuid REFERENCES scheduling_resources(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('holiday', 'manual', 'maintenance', 'leave')),
    reason text NOT NULL DEFAULT '',
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    all_day boolean NOT NULL DEFAULT false,
    created_by text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (start_at < end_at)
);

CREATE INDEX IF NOT EXISTS idx_scheduling_blocked_ranges_branch ON scheduling_blocked_ranges(org_id, branch_id, start_at, end_at);
CREATE INDEX IF NOT EXISTS idx_scheduling_blocked_ranges_resource ON scheduling_blocked_ranges(org_id, resource_id, start_at, end_at) WHERE resource_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS scheduling_bookings (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES scheduling_services(id) ON DELETE RESTRICT,
    resource_id uuid NOT NULL REFERENCES scheduling_resources(id) ON DELETE RESTRICT,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    reference text NOT NULL,
    customer_name text NOT NULL,
    customer_phone text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('hold', 'pending_confirmation', 'confirmed', 'checked_in', 'in_service', 'completed', 'cancelled', 'no_show', 'expired')),
    source text NOT NULL CHECK (source IN ('admin', 'public_web', 'whatsapp', 'api')),
    idempotency_key text,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    occupies_from timestamptz NOT NULL,
    occupies_until timestamptz NOT NULL,
    hold_expires_at timestamptz,
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by text NOT NULL DEFAULT '',
    confirmed_at timestamptz,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (start_at < end_at),
    CHECK (occupies_from < occupies_until)
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_scheduling_bookings_idempotency ON scheduling_bookings(org_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_scheduling_bookings_org_start ON scheduling_bookings(org_id, start_at);
CREATE INDEX IF NOT EXISTS idx_scheduling_bookings_branch_start ON scheduling_bookings(org_id, branch_id, start_at);
CREATE INDEX IF NOT EXISTS idx_scheduling_bookings_resource_window ON scheduling_bookings(org_id, resource_id, occupies_from, occupies_until);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'scheduling_bookings_no_overlap'
    ) THEN
        ALTER TABLE scheduling_bookings
            ADD CONSTRAINT scheduling_bookings_no_overlap
            EXCLUDE USING gist (
                org_id WITH =,
                resource_id WITH =,
                tstzrange(occupies_from, occupies_until, '[)') WITH &&
            )
            WHERE (status IN ('hold', 'pending_confirmation', 'confirmed', 'checked_in', 'in_service'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS scheduling_queues (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    service_id uuid REFERENCES scheduling_services(id) ON DELETE SET NULL,
    code text NOT NULL,
    name text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'paused', 'closed')),
    strategy text NOT NULL CHECK (strategy IN ('fifo', 'priority')),
    ticket_prefix text NOT NULL DEFAULT 'T',
    last_issued_number bigint NOT NULL DEFAULT 0,
    avg_service_seconds integer NOT NULL DEFAULT 600 CHECK (avg_service_seconds > 0),
    allow_remote_join boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, code)
);

CREATE INDEX IF NOT EXISTS idx_scheduling_queues_branch_status ON scheduling_queues(org_id, branch_id, status);

CREATE TABLE IF NOT EXISTS scheduling_queue_tickets (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    queue_id uuid NOT NULL REFERENCES scheduling_queues(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    service_id uuid REFERENCES scheduling_services(id) ON DELETE SET NULL,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    customer_name text NOT NULL,
    customer_phone text NOT NULL DEFAULT '',
    number bigint NOT NULL,
    display_code text NOT NULL,
    status text NOT NULL CHECK (status IN ('waiting', 'called', 'serving', 'completed', 'no_show', 'cancelled')),
    priority integer NOT NULL DEFAULT 100,
    source text NOT NULL CHECK (source IN ('reception', 'web', 'whatsapp', 'api')),
    idempotency_key text,
    serving_resource_id uuid REFERENCES scheduling_resources(id) ON DELETE SET NULL,
    operator_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    called_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz,
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (queue_id, number)
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_scheduling_queue_tickets_idempotency ON scheduling_queue_tickets(org_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_scheduling_queue_tickets_queue_waiting ON scheduling_queue_tickets(queue_id, status, priority, requested_at, number);
CREATE INDEX IF NOT EXISTS idx_scheduling_queue_tickets_branch_requested ON scheduling_queue_tickets(org_id, branch_id, requested_at);
