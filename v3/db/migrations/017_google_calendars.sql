CREATE TABLE IF NOT EXISTS app.calendar_connections (
    id text NOT NULL,
    org_id text NOT NULL,
    actor_id text NOT NULL,
    provider text NOT NULL CHECK (provider = 'google'),
    status text NOT NULL CHECK (
        status IN ('pending','active','reauth_required','revoked')
    ),
    calendar_id text NOT NULL DEFAULT '',
    time_zone text NOT NULL,
    scopes text[] NOT NULL CHECK (cardinality(scopes) > 0),
    free_busy_enabled boolean NOT NULL DEFAULT false,
    meet_enabled boolean NOT NULL DEFAULT false,
    token_envelope bytea,
    access_token_expires_at timestamptz,
    version integer NOT NULL CHECK (version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (org_id,id),
    FOREIGN KEY (org_id) REFERENCES app.organizations(id) ON DELETE CASCADE,
    CHECK (
        (status IN ('pending','revoked')) OR
        (calendar_id <> '' AND token_envelope IS NOT NULL)
    )
);

CREATE TABLE IF NOT EXISTS app.calendar_oauth_states (
    state_hash bytea NOT NULL CHECK (octet_length(state_hash) = 32),
    org_id text NOT NULL,
    actor_id text NOT NULL,
    connection_id text NOT NULL,
    session_binding text NOT NULL,
    time_zone text NOT NULL,
    free_busy_enabled boolean NOT NULL DEFAULT false,
    meet_enabled boolean NOT NULL DEFAULT false,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (org_id,state_hash),
    FOREIGN KEY (org_id,connection_id)
        REFERENCES app.calendar_connections(org_id,id) ON DELETE CASCADE,
    CHECK (expires_at > created_at),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);

CREATE INDEX IF NOT EXISTS calendar_oauth_states_expiry_idx
    ON app.calendar_oauth_states (expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS app.external_calendar_events (
    org_id text NOT NULL,
    connection_id text NOT NULL,
    booking_id text NOT NULL,
    google_event_id text NOT NULL,
    etag text NOT NULL DEFAULT '',
    meet_request_id text NOT NULL DEFAULT '',
    meet_status text NOT NULL DEFAULT '',
    meet_uri text NOT NULL DEFAULT '',
    source_version integer NOT NULL CHECK (source_version > 0),
    snapshot_digest text NOT NULL CHECK (
        snapshot_digest ~ '^[0-9a-f]{64}$'
    ),
    status text NOT NULL CHECK (
        status IN ('pending','synced','deleting','deleted','uncertain','reconcile')
    ),
    last_error_code text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (org_id,connection_id,booking_id),
    UNIQUE (org_id,connection_id,google_event_id),
    FOREIGN KEY (org_id,connection_id)
        REFERENCES app.calendar_connections(org_id,id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS app.calendar_sync_attempts (
    id uuid PRIMARY KEY,
    org_id text NOT NULL,
    connection_id text NOT NULL,
    booking_id text NOT NULL,
    operation text NOT NULL CHECK (operation IN ('upsert','delete','reconcile')),
    source_version integer NOT NULL CHECK (source_version > 0),
    snapshot_digest text NOT NULL CHECK (
        snapshot_digest ~ '^[0-9a-f]{64}$'
    ),
    outcome text NOT NULL CHECK (
        outcome IN ('synced','duplicate','uncertain','retry','failed')
    ),
    error_code text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    FOREIGN KEY (org_id,connection_id)
        REFERENCES app.calendar_connections(org_id,id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS calendar_sync_attempts_lookup_idx
    ON app.calendar_sync_attempts (org_id,connection_id,booking_id,occurred_at DESC);

ALTER TABLE app.calendar_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.calendar_connections FORCE ROW LEVEL SECURITY;
ALTER TABLE app.calendar_oauth_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.calendar_oauth_states FORCE ROW LEVEL SECURITY;
ALTER TABLE app.external_calendar_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.external_calendar_events FORCE ROW LEVEL SECURITY;
ALTER TABLE app.calendar_sync_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.calendar_sync_attempts FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS calendar_connections_tenant ON app.calendar_connections;
CREATE POLICY calendar_connections_tenant ON app.calendar_connections
    USING (org_id = NULLIF(current_setting('app.org_id',true),''))
    WITH CHECK (org_id = NULLIF(current_setting('app.org_id',true),''));

DROP POLICY IF EXISTS calendar_oauth_states_tenant ON app.calendar_oauth_states;
CREATE POLICY calendar_oauth_states_tenant ON app.calendar_oauth_states
    USING (org_id = NULLIF(current_setting('app.org_id',true),''))
    WITH CHECK (org_id = NULLIF(current_setting('app.org_id',true),''));

DROP POLICY IF EXISTS external_calendar_events_tenant ON app.external_calendar_events;
CREATE POLICY external_calendar_events_tenant ON app.external_calendar_events
    USING (org_id = NULLIF(current_setting('app.org_id',true),''))
    WITH CHECK (org_id = NULLIF(current_setting('app.org_id',true),''));

DROP POLICY IF EXISTS calendar_sync_attempts_tenant ON app.calendar_sync_attempts;
CREATE POLICY calendar_sync_attempts_tenant ON app.calendar_sync_attempts
    USING (org_id = NULLIF(current_setting('app.org_id',true),''))
    WITH CHECK (org_id = NULLIF(current_setting('app.org_id',true),''));
