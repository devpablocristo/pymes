ALTER TABLE scheduling_services
    ADD COLUMN IF NOT EXISTS min_cancel_notice_minutes integer NOT NULL DEFAULT 0 CHECK (min_cancel_notice_minutes >= 0),
    ADD COLUMN IF NOT EXISTS allow_waitlist boolean NOT NULL DEFAULT false;

ALTER TABLE scheduling_bookings
    ADD COLUMN IF NOT EXISTS customer_email text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reminder_sent_at timestamptz;

ALTER TABLE scheduling_queue_tickets
    ADD COLUMN IF NOT EXISTS customer_email text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS scheduling_booking_action_tokens (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    booking_id uuid NOT NULL REFERENCES scheduling_bookings(id) ON DELETE CASCADE,
    action text NOT NULL CHECK (action IN ('confirm', 'cancel')),
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    voided_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_scheduling_booking_action_tokens_hash ON scheduling_booking_action_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_scheduling_booking_action_tokens_booking ON scheduling_booking_action_tokens(org_id, booking_id, action);
CREATE INDEX IF NOT EXISTS idx_scheduling_booking_action_tokens_active ON scheduling_booking_action_tokens(expires_at) WHERE used_at IS NULL AND voided_at IS NULL;

CREATE TABLE IF NOT EXISTS scheduling_waitlist_entries (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid NOT NULL REFERENCES scheduling_branches(id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES scheduling_services(id) ON DELETE CASCADE,
    resource_id uuid REFERENCES scheduling_resources(id) ON DELETE SET NULL,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    booking_id uuid REFERENCES scheduling_bookings(id) ON DELETE SET NULL,
    customer_name text NOT NULL,
    customer_phone text NOT NULL DEFAULT '',
    customer_email text NOT NULL DEFAULT '',
    requested_start_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'notified', 'booked', 'cancelled', 'expired')),
    source text NOT NULL CHECK (source IN ('admin', 'public_web', 'whatsapp', 'api')),
    idempotency_key text,
    expires_at timestamptz,
    notified_at timestamptz,
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uidx_scheduling_waitlist_entries_idempotency ON scheduling_waitlist_entries(org_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_scheduling_waitlist_entries_scope ON scheduling_waitlist_entries(org_id, branch_id, service_id, requested_start_at, status);
CREATE INDEX IF NOT EXISTS idx_scheduling_waitlist_entries_pending ON scheduling_waitlist_entries(status, expires_at, requested_start_at) WHERE status IN ('pending', 'notified');
