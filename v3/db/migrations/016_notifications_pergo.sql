BEGIN;

CREATE TABLE IF NOT EXISTS app.notification_settings (
  org_id text PRIMARY KEY REFERENCES app.organizations(id) ON DELETE CASCADE,
  whatsapp_enabled boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app.notifications (
  org_id text NOT NULL REFERENCES app.organizations(id) ON DELETE CASCADE,
  id text NOT NULL,
  kind text NOT NULL CHECK (
    kind IN ('confirmation','reminder','rescheduled','cancellation','waitlist')
  ),
  aggregate_type text NOT NULL,
  aggregate_id text NOT NULL,
  recipient_e164 text NOT NULL CHECK (
    recipient_e164 ~ '^\+[1-9][0-9]{7,14}$'
  ),
  template_name text NOT NULL,
  template_version integer NOT NULL CHECK (template_version > 0),
  locale text NOT NULL CHECK (locale ~ '^[a-z]{2}(_[A-Z]{2})?$'),
  variables jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (
    jsonb_typeof(variables) = 'object'
  ),
  body text NOT NULL CHECK (
    char_length(body) BETWEEN 1 AND 4096
  ),
  send_at timestamptz NOT NULL,
  status text NOT NULL CHECK (
    status IN ('pending','uncertain','queued','sent','delivered','read','failed')
  ),
  external_message_id text,
  idempotency_key text NOT NULL,
  correlation_id text NOT NULL,
  request_id text NOT NULL,
  actor_ref text NOT NULL,
  source_version integer NOT NULL CHECK (source_version > 0),
  snapshot_digest char(64) NOT NULL CHECK (
    snapshot_digest ~ '^[0-9a-f]{64}$'
  ),
  failure_code text CHECK (
    failure_code IS NULL OR failure_code ~ '^[A-Z0-9_]{1,80}$'
  ),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id,id),
  UNIQUE (org_id,idempotency_key),
  CHECK (char_length(id) BETWEEN 1 AND 255),
  CHECK (char_length(aggregate_type) BETWEEN 1 AND 80),
  CHECK (char_length(aggregate_id) BETWEEN 1 AND 255),
  CHECK (char_length(template_name) BETWEEN 1 AND 120),
  CHECK (char_length(idempotency_key) BETWEEN 1 AND 255),
  CHECK (char_length(correlation_id) BETWEEN 1 AND 255),
  CHECK (char_length(request_id) BETWEEN 1 AND 255),
  CHECK (char_length(actor_ref) BETWEEN 1 AND 255)
);

CREATE INDEX IF NOT EXISTS notifications_due_idx
  ON app.notifications (org_id,send_at)
  WHERE status IN ('pending','uncertain');
CREATE INDEX IF NOT EXISTS notifications_aggregate_idx
  ON app.notifications (org_id,aggregate_type,aggregate_id,created_at DESC);

CREATE TABLE IF NOT EXISTS app.notification_webhook_inbox (
  -- Deliberately no FK to organizations: this immutable delivery proof must
  -- not be truncated by organization test/setup cleanup or lifecycle cascades.
  org_id text NOT NULL,
  payload_hash char(64) NOT NULL CHECK (
    payload_hash ~ '^[0-9a-f]{64}$'
  ),
  event_type text NOT NULL,
  trace_id text NOT NULL,
  message_id text NOT NULL,
  workspace_id text NOT NULL,
  occurred_at timestamptz NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id,payload_hash)
);

ALTER TABLE app.notification_webhook_inbox
  DROP CONSTRAINT IF EXISTS notification_webhook_inbox_org_id_fkey;

CREATE INDEX IF NOT EXISTS notification_webhook_trace_idx
  ON app.notification_webhook_inbox (org_id,trace_id,occurred_at);

ALTER TABLE app.notification_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.notification_settings FORCE ROW LEVEL SECURITY;
ALTER TABLE app.notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.notifications FORCE ROW LEVEL SECURITY;
ALTER TABLE app.notification_webhook_inbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.notification_webhook_inbox FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS notification_settings_org_isolation
  ON app.notification_settings;
CREATE POLICY notification_settings_org_isolation
  ON app.notification_settings
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

DROP POLICY IF EXISTS notifications_org_isolation ON app.notifications;
CREATE POLICY notifications_org_isolation
  ON app.notifications
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

DROP POLICY IF EXISTS notification_webhook_inbox_org_isolation
  ON app.notification_webhook_inbox;
CREATE POLICY notification_webhook_inbox_org_isolation
  ON app.notification_webhook_inbox
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

CREATE OR REPLACE FUNCTION app.reject_notification_webhook_inbox_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'notification webhook inbox is immutable'
    USING ERRCODE = '55000';
END
$$;

DROP TRIGGER IF EXISTS notification_webhook_inbox_no_update_delete
  ON app.notification_webhook_inbox;
CREATE TRIGGER notification_webhook_inbox_no_update_delete
  BEFORE UPDATE OR DELETE ON app.notification_webhook_inbox
  FOR EACH ROW
  EXECUTE FUNCTION app.reject_notification_webhook_inbox_mutation();

DROP TRIGGER IF EXISTS notification_webhook_inbox_no_truncate
  ON app.notification_webhook_inbox;
CREATE TRIGGER notification_webhook_inbox_no_truncate
  BEFORE TRUNCATE ON app.notification_webhook_inbox
  FOR EACH STATEMENT
  EXECUTE FUNCTION app.reject_notification_webhook_inbox_mutation();

COMMIT;
