BEGIN;

CREATE TABLE IF NOT EXISTS app.outbox_dead_letter_replays (
  -- Deliberately no FK to organizations: audit evidence must survive tenant
  -- suspension/removal and must not be truncated through an operational table.
  org_id text NOT NULL,
  event_id uuid NOT NULL,
  failed_at timestamptz NOT NULL,
  failure_code text NOT NULL,
  replayed_at timestamptz NOT NULL DEFAULT now(),
  actor_ref text NOT NULL,
  change_ref text NOT NULL,
  PRIMARY KEY (org_id, event_id, failed_at),
  CHECK (actor_ref ~ '^[A-Za-z0-9:_./-]{1,255}$'),
  CHECK (change_ref ~ '^[A-Za-z0-9:_./-]{1,255}$')
);

ALTER TABLE app.outbox_dead_letter_replays
  DROP CONSTRAINT IF EXISTS outbox_dead_letter_replays_org_id_fkey;

CREATE INDEX IF NOT EXISTS outbox_dead_letter_replays_replayed_at_idx
  ON app.outbox_dead_letter_replays (replayed_at DESC);

ALTER TABLE app.outbox_dead_letter_replays ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.outbox_dead_letter_replays FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS outbox_dead_letter_replays_org_isolation
  ON app.outbox_dead_letter_replays;
CREATE POLICY outbox_dead_letter_replays_org_isolation
  ON app.outbox_dead_letter_replays
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

CREATE OR REPLACE FUNCTION app.reject_dead_letter_replay_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'outbox dead-letter replay audit is immutable'
    USING ERRCODE = '55000';
END
$$;

DROP TRIGGER IF EXISTS outbox_dead_letter_replay_audit_no_update_delete
  ON app.outbox_dead_letter_replays;
CREATE TRIGGER outbox_dead_letter_replay_audit_no_update_delete
  BEFORE UPDATE OR DELETE ON app.outbox_dead_letter_replays
  FOR EACH ROW
  EXECUTE FUNCTION app.reject_dead_letter_replay_audit_mutation();

DROP TRIGGER IF EXISTS outbox_dead_letter_replay_audit_no_truncate
  ON app.outbox_dead_letter_replays;
CREATE TRIGGER outbox_dead_letter_replay_audit_no_truncate
  BEFORE TRUNCATE ON app.outbox_dead_letter_replays
  FOR EACH STATEMENT
  EXECUTE FUNCTION app.reject_dead_letter_replay_audit_mutation();

COMMIT;
