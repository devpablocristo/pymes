BEGIN;

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS app.scheduling_branches (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  code text NOT NULL,
  slug text NOT NULL,
  name text NOT NULL,
  timezone text NOT NULL,
  address text NOT NULL DEFAULT '',
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, code),
  UNIQUE (org_id, slug),
  CHECK (btrim(code) <> '' AND btrim(slug) <> '' AND btrim(name) <> '')
);

CREATE TABLE IF NOT EXISTS app.scheduling_services (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  code text NOT NULL,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 1440),
  buffer_before_minutes integer NOT NULL DEFAULT 0 CHECK (buffer_before_minutes >= 0),
  buffer_after_minutes integer NOT NULL DEFAULT 0 CHECK (buffer_after_minutes >= 0),
  slot_minutes integer NOT NULL CHECK (slot_minutes BETWEEN 1 AND 1440),
  price numeric(20,6) NOT NULL CHECK (price >= 0),
  currency char(3) NOT NULL,
  fulfillment_mode text NOT NULL CHECK (fulfillment_mode IN ('in_person','virtual','hybrid')),
  max_participants integer NOT NULL DEFAULT 1 CHECK (max_participants BETWEEN 1 AND 100000),
  allow_group boolean NOT NULL DEFAULT false,
  allow_waitlist boolean NOT NULL DEFAULT false,
  confirmation_required boolean NOT NULL DEFAULT false,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, code),
  CHECK (allow_group OR max_participants = 1)
);

CREATE TABLE IF NOT EXISTS app.scheduling_resources (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  branch_id uuid NOT NULL,
  code text NOT NULL,
  name text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('professional','room','machine','vehicle','equipment','generic')),
  capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 100000),
  timezone text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, branch_id, code),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS app.scheduling_service_resource_requirements (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  service_id uuid NOT NULL,
  resource_id uuid,
  resource_kind text NOT NULL CHECK (resource_kind IN ('professional','room','machine','vehicle','equipment','generic')),
  allocation_mode text NOT NULL CHECK (allocation_mode IN ('capacity','exclusive')),
  units integer NOT NULL CHECK (units BETWEEN 1 AND 100000),
  optional boolean NOT NULL DEFAULT false,
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, service_id)
    REFERENCES app.scheduling_services(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, resource_id)
    REFERENCES app.scheduling_resources(org_id, id)
);

CREATE TABLE IF NOT EXISTS app.scheduling_availability_rules (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  branch_id uuid NOT NULL,
  resource_id uuid,
  kind text NOT NULL CHECK (kind IN ('branch','resource')),
  weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
  start_minute integer NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
  end_minute integer NOT NULL CHECK (end_minute BETWEEN 0 AND 2879),
  valid_from date,
  valid_until date,
  timezone text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, resource_id)
    REFERENCES app.scheduling_resources(org_id, id) ON DELETE CASCADE,
  CHECK (
    (kind = 'branch' AND resource_id IS NULL)
    OR (kind = 'resource' AND resource_id IS NOT NULL)
  ),
  CHECK (start_minute <> end_minute),
  CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until >= valid_from)
);

CREATE TABLE IF NOT EXISTS app.scheduling_exceptions (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  branch_id uuid NOT NULL,
  resource_id uuid,
  kind text NOT NULL CHECK (kind IN ('holiday','vacation','absence','manual','maintenance','availability')),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  reason text NOT NULL DEFAULT '',
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, resource_id)
    REFERENCES app.scheduling_resources(org_id, id) ON DELETE CASCADE,
  CHECK (ends_at > starts_at)
);
CREATE INDEX IF NOT EXISTS scheduling_exceptions_range_idx
  ON app.scheduling_exceptions USING gist (org_id, branch_id, tstzrange(starts_at, ends_at, '[)'));

CREATE TABLE IF NOT EXISTS app.scheduling_recurrence_series (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  frequency text NOT NULL CHECK (frequency IN ('daily','weekly')),
  interval_count integer NOT NULL CHECK (interval_count BETWEEN 1 AND 365),
  occurrence_count integer NOT NULL DEFAULT 0 CHECK (occurrence_count BETWEEN 0 AND 500),
  until_at timestamptz,
  by_weekdays smallint[] NOT NULL DEFAULT '{}',
  timezone text NOT NULL,
  status text NOT NULL CHECK (status IN ('active','cancelled','completed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id)
);

CREATE TABLE IF NOT EXISTS app.scheduling_group_sessions (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  branch_id uuid NOT NULL,
  service_id uuid NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 100000),
  booked integer NOT NULL DEFAULT 0 CHECK (booked >= 0),
  version integer NOT NULL DEFAULT 1 CHECK (version > 0),
  status text NOT NULL CHECK (status IN ('open','closed','cancelled','completed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id),
  FOREIGN KEY (org_id, service_id)
    REFERENCES app.scheduling_services(org_id, id),
  CHECK (ends_at > starts_at),
  CHECK (booked <= capacity)
);

CREATE TABLE IF NOT EXISTS app.scheduling_session_resource_allocations (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  session_id uuid NOT NULL,
  resource_id uuid NOT NULL,
  allocation_mode text NOT NULL CHECK (allocation_mode IN ('capacity','exclusive')),
  units integer NOT NULL CHECK (units BETWEEN 1 AND 100000),
  occupies_from timestamptz NOT NULL,
  occupies_until timestamptz NOT NULL,
  active boolean NOT NULL DEFAULT true,
  occupation tstzrange GENERATED ALWAYS AS (tstzrange(occupies_from, occupies_until, '[)')) STORED,
  PRIMARY KEY (org_id, session_id, resource_id),
  FOREIGN KEY (org_id, session_id)
    REFERENCES app.scheduling_group_sessions(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, resource_id)
    REFERENCES app.scheduling_resources(org_id, id),
  CHECK (occupies_until > occupies_from)
);
CREATE INDEX IF NOT EXISTS scheduling_session_allocations_capacity_idx
  ON app.scheduling_session_resource_allocations USING gist (org_id, resource_id, occupation)
  WHERE active;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'scheduling_exclusive_session_resource_no_overlap'
      AND conrelid = 'app.scheduling_session_resource_allocations'::regclass
  ) THEN
    ALTER TABLE app.scheduling_session_resource_allocations
      ADD CONSTRAINT scheduling_exclusive_session_resource_no_overlap
      EXCLUDE USING gist (
        org_id WITH =,
        resource_id WITH =,
        occupation WITH &&
      )
      WHERE (active AND allocation_mode = 'exclusive');
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS app.scheduling_bookings (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  series_id uuid,
  session_id uuid,
  supersedes_id uuid,
  occurrence integer NOT NULL DEFAULT 0 CHECK (occurrence >= 0),
  branch_id uuid NOT NULL,
  service_id uuid NOT NULL,
  party_id text NOT NULL,
  status text NOT NULL CHECK (status IN (
    'held','pending_confirmation','confirmed','checked_in','completed',
    'cancelled','rescheduled','no_show'
  )),
  participants integer NOT NULL CHECK (participants BETWEEN 1 AND 100000),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  occupies_from timestamptz NOT NULL,
  occupies_until timestamptz NOT NULL,
  hold_expires_at timestamptz,
  version integer NOT NULL DEFAULT 1 CHECK (version > 0),
  service_name_snapshot text NOT NULL,
  price_snapshot numeric(20,6) NOT NULL CHECK (price_snapshot >= 0),
  currency_snapshot char(3) NOT NULL,
  duration_minutes_snapshot integer NOT NULL CHECK (duration_minutes_snapshot > 0),
  timezone_snapshot text NOT NULL,
  customer_name_snapshot text NOT NULL DEFAULT '',
  customer_email_snapshot text NOT NULL DEFAULT '',
  customer_phone_snapshot text NOT NULL DEFAULT '',
  notes text NOT NULL DEFAULT '',
  cancellation_reason text NOT NULL DEFAULT '',
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id),
  FOREIGN KEY (org_id, service_id)
    REFERENCES app.scheduling_services(org_id, id),
  FOREIGN KEY (org_id, party_id)
    REFERENCES app.parties(org_id, id),
  FOREIGN KEY (org_id, series_id)
    REFERENCES app.scheduling_recurrence_series(org_id, id),
  FOREIGN KEY (org_id, session_id)
    REFERENCES app.scheduling_group_sessions(org_id, id),
  FOREIGN KEY (org_id, supersedes_id)
    REFERENCES app.scheduling_bookings(org_id, id),
  CHECK (ends_at > starts_at),
  CHECK (occupies_until > occupies_from),
  CHECK (
    (status = 'held' AND hold_expires_at IS NOT NULL)
    OR (status <> 'held')
  )
);
ALTER TABLE app.scheduling_bookings
  ADD COLUMN IF NOT EXISTS cancellation_reason text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS scheduling_bookings_range_idx
  ON app.scheduling_bookings USING gist (org_id, branch_id, tstzrange(occupies_from, occupies_until, '[)'));
CREATE INDEX IF NOT EXISTS scheduling_bookings_party_idx
  ON app.scheduling_bookings (org_id, party_id, starts_at DESC);

CREATE TABLE IF NOT EXISTS app.scheduling_booking_resource_allocations (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  booking_id uuid NOT NULL,
  resource_id uuid NOT NULL,
  allocation_mode text NOT NULL CHECK (allocation_mode IN ('capacity','exclusive')),
  units integer NOT NULL CHECK (units BETWEEN 1 AND 100000),
  occupies_from timestamptz NOT NULL,
  occupies_until timestamptz NOT NULL,
  active boolean NOT NULL DEFAULT true,
  occupation tstzrange GENERATED ALWAYS AS (tstzrange(occupies_from, occupies_until, '[)')) STORED,
  PRIMARY KEY (org_id, booking_id, resource_id),
  FOREIGN KEY (org_id, booking_id)
    REFERENCES app.scheduling_bookings(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, resource_id)
    REFERENCES app.scheduling_resources(org_id, id),
  CHECK (occupies_until > occupies_from)
);
CREATE INDEX IF NOT EXISTS scheduling_allocations_capacity_idx
  ON app.scheduling_booking_resource_allocations USING gist (org_id, resource_id, occupation)
  WHERE active;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'scheduling_exclusive_resource_no_overlap'
      AND conrelid = 'app.scheduling_booking_resource_allocations'::regclass
  ) THEN
    ALTER TABLE app.scheduling_booking_resource_allocations
      ADD CONSTRAINT scheduling_exclusive_resource_no_overlap
      EXCLUDE USING gist (
        org_id WITH =,
        resource_id WITH =,
        occupation WITH &&
      )
      WHERE (active AND allocation_mode = 'exclusive');
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS app.scheduling_group_participants (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  session_id uuid NOT NULL,
  booking_id uuid NOT NULL,
  party_id text NOT NULL,
  seats integer NOT NULL CHECK (seats BETWEEN 1 AND 100000),
  status text NOT NULL CHECK (status IN ('reserved','checked_in','cancelled','no_show')),
  PRIMARY KEY (org_id, session_id, booking_id, party_id),
  FOREIGN KEY (org_id, session_id)
    REFERENCES app.scheduling_group_sessions(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, booking_id)
    REFERENCES app.scheduling_bookings(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, party_id)
    REFERENCES app.parties(org_id, id)
);

CREATE TABLE IF NOT EXISTS app.scheduling_holds (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  booking_id uuid NOT NULL,
  expires_at timestamptz NOT NULL,
  released_at timestamptz,
  release_reason text,
  PRIMARY KEY (org_id, booking_id),
  FOREIGN KEY (org_id, booking_id)
    REFERENCES app.scheduling_bookings(org_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS scheduling_holds_expiry_idx
  ON app.scheduling_holds (expires_at) WHERE released_at IS NULL;

CREATE TABLE IF NOT EXISTS app.scheduling_waitlist (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  branch_id uuid NOT NULL,
  service_id uuid NOT NULL,
  party_id text NOT NULL,
  customer_name_snapshot text NOT NULL DEFAULT '',
  customer_email_snapshot text NOT NULL DEFAULT '',
  customer_phone_snapshot text NOT NULL DEFAULT '',
  preferred_from timestamptz NOT NULL,
  preferred_until timestamptz NOT NULL,
  participants integer NOT NULL CHECK (participants BETWEEN 1 AND 100000),
  status text NOT NULL CHECK (status IN ('pending','offered','accepted','cancelled','expired')),
  offer_expires_at timestamptz,
  offered_starts_at timestamptz,
  offered_ends_at timestamptz,
  offered_allocations jsonb NOT NULL DEFAULT '[]'::jsonb,
  accepted_booking_id uuid,
  lease_token uuid,
  lease_expires_at timestamptz,
  version integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id),
  FOREIGN KEY (org_id, service_id)
    REFERENCES app.scheduling_services(org_id, id),
  FOREIGN KEY (org_id, party_id)
    REFERENCES app.parties(org_id, id),
  CONSTRAINT scheduling_waitlist_accepted_booking_fk FOREIGN KEY (org_id, accepted_booking_id)
    REFERENCES app.scheduling_bookings(org_id, id),
  CHECK (preferred_until > preferred_from)
);
ALTER TABLE app.scheduling_waitlist
  ADD COLUMN IF NOT EXISTS customer_name_snapshot text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS customer_email_snapshot text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS customer_phone_snapshot text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS offered_starts_at timestamptz,
  ADD COLUMN IF NOT EXISTS offered_ends_at timestamptz,
  ADD COLUMN IF NOT EXISTS offered_allocations jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS accepted_booking_id uuid;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'scheduling_waitlist_accepted_booking_fk'
      AND conrelid = 'app.scheduling_waitlist'::regclass
  ) THEN
    ALTER TABLE app.scheduling_waitlist
      ADD CONSTRAINT scheduling_waitlist_accepted_booking_fk
      FOREIGN KEY (org_id, accepted_booking_id)
      REFERENCES app.scheduling_bookings(org_id, id);
  END IF;
END
$$;
CREATE INDEX IF NOT EXISTS scheduling_waitlist_pending_idx
  ON app.scheduling_waitlist (org_id, branch_id, service_id, created_at)
  WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS app.scheduling_reminders (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  booking_id uuid NOT NULL,
  reminder_at timestamptz NOT NULL,
  claimed_at timestamptz,
  event_id uuid,
  PRIMARY KEY (org_id, booking_id, reminder_at),
  FOREIGN KEY (org_id, booking_id)
    REFERENCES app.scheduling_bookings(org_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS scheduling_reminders_due_idx
  ON app.scheduling_reminders (reminder_at)
  WHERE claimed_at IS NULL;

CREATE TABLE IF NOT EXISTS app.scheduling_action_tokens (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  booking_id uuid,
  waitlist_id uuid,
  result_booking_id uuid,
  purpose text NOT NULL CHECK (purpose IN ('confirm','cancel','reschedule','accept_waitlist')),
  token_hash char(64) NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (token_hash),
  FOREIGN KEY (org_id, booking_id)
    REFERENCES app.scheduling_bookings(org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, waitlist_id)
    REFERENCES app.scheduling_waitlist(org_id, id) ON DELETE CASCADE,
  CONSTRAINT scheduling_action_tokens_result_booking_fk FOREIGN KEY (org_id, result_booking_id)
    REFERENCES app.scheduling_bookings(org_id, id),
  CHECK (
    (booking_id IS NOT NULL AND waitlist_id IS NULL)
    OR (booking_id IS NULL AND waitlist_id IS NOT NULL)
  )
);
ALTER TABLE app.scheduling_action_tokens
  ADD COLUMN IF NOT EXISTS result_booking_id uuid;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'scheduling_action_tokens_result_booking_fk'
      AND conrelid = 'app.scheduling_action_tokens'::regclass
  ) THEN
    ALTER TABLE app.scheduling_action_tokens
      ADD CONSTRAINT scheduling_action_tokens_result_booking_fk
      FOREIGN KEY (org_id, result_booking_id)
      REFERENCES app.scheduling_bookings(org_id, id);
  END IF;
END
$$;
CREATE INDEX IF NOT EXISTS scheduling_action_tokens_expiry_idx
  ON app.scheduling_action_tokens (expires_at) WHERE consumed_at IS NULL;

-- Exact-hash routing index for opaque public action tokens. It contains no PII
-- or action payload and is not tenant-owned; after resolving the organization,
-- every token read/update occurs under that tenant's forced RLS context.
CREATE TABLE IF NOT EXISTS app.scheduling_public_token_directory (
  token_hash char(64) PRIMARY KEY,
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS app.scheduling_queue_counters (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  branch_id uuid NOT NULL,
  service_id uuid NOT NULL,
  next_number bigint NOT NULL DEFAULT 1 CHECK (next_number > 0),
  PRIMARY KEY (org_id, branch_id, service_id),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id),
  FOREIGN KEY (org_id, service_id)
    REFERENCES app.scheduling_services(org_id, id)
);

CREATE TABLE IF NOT EXISTS app.scheduling_queue_tickets (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  branch_id uuid NOT NULL,
  service_id uuid NOT NULL,
  party_id text NOT NULL,
  number bigint NOT NULL,
  priority integer NOT NULL DEFAULT 0,
  status text NOT NULL CHECK (status IN ('waiting','called','serving','completed','no_show','cancelled')),
  called_at timestamptz,
  started_at timestamptz,
  completed_at timestamptz,
  version integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, branch_id, service_id, number),
  FOREIGN KEY (org_id, branch_id)
    REFERENCES app.scheduling_branches(org_id, id),
  FOREIGN KEY (org_id, service_id)
    REFERENCES app.scheduling_services(org_id, id),
  FOREIGN KEY (org_id, party_id)
    REFERENCES app.parties(org_id, id)
);
CREATE INDEX IF NOT EXISTS scheduling_queue_order_idx
  ON app.scheduling_queue_tickets (org_id, branch_id, status, priority DESC, number)
  WHERE status IN ('waiting','called','serving');

CREATE TABLE IF NOT EXISTS app.scheduling_audit (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  actor_id text NOT NULL,
  action text NOT NULL,
  aggregate_id text NOT NULL,
  before_state jsonb,
  after_state jsonb,
  request_id text NOT NULL,
  correlation_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, id)
);

-- Append-only lifecycle catalog. Integration commands are additionally
-- projected to app.outbox, whose topics are leased by an explicit owner.
CREATE TABLE IF NOT EXISTS app.scheduling_events (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  event_type text NOT NULL,
  aggregate_id text NOT NULL,
  payload jsonb NOT NULL,
  payload_hash char(64) NOT NULL,
  idempotency_key text NOT NULL,
  request_id text NOT NULL,
  actor_ref text NOT NULL,
  source_version integer NOT NULL CHECK (source_version > 0),
  correlation_id text NOT NULL,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (org_id, id),
  UNIQUE (org_id, event_type, idempotency_key)
);
CREATE INDEX IF NOT EXISTS scheduling_events_aggregate_idx
  ON app.scheduling_events (org_id, aggregate_id, occurred_at, id);

CREATE INDEX IF NOT EXISTS outbox_topic_available_idx
  ON app.outbox (topic, available_at, created_at)
  WHERE published_at IS NULL;

DO $$
DECLARE
  table_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'scheduling_branches',
    'scheduling_services',
    'scheduling_resources',
    'scheduling_service_resource_requirements',
    'scheduling_availability_rules',
    'scheduling_exceptions',
    'scheduling_recurrence_series',
    'scheduling_group_sessions',
    'scheduling_session_resource_allocations',
    'scheduling_bookings',
    'scheduling_booking_resource_allocations',
    'scheduling_group_participants',
    'scheduling_holds',
    'scheduling_waitlist',
    'scheduling_reminders',
    'scheduling_action_tokens',
    'scheduling_queue_counters',
    'scheduling_queue_tickets',
    'scheduling_audit',
    'scheduling_events'
  ]
  LOOP
    EXECUTE format('ALTER TABLE app.%I ENABLE ROW LEVEL SECURITY', table_name);
    EXECUTE format('ALTER TABLE app.%I FORCE ROW LEVEL SECURITY', table_name);
    EXECUTE format('DROP POLICY IF EXISTS %I ON app.%I', table_name || '_org_isolation', table_name);
    EXECUTE format(
      'CREATE POLICY %I ON app.%I USING (org_id = current_setting(''app.org_id'', true)) WITH CHECK (org_id = current_setting(''app.org_id'', true))',
      table_name || '_org_isolation',
      table_name
    );
  END LOOP;
END
$$;

COMMIT;
