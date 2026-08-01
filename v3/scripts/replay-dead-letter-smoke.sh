#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
admin_url=${PYMES_DATABASE_ADMIN_TEST_URL:-postgres://pymes:pymes@127.0.0.1:55434/postgres?sslmode=disable}
database_name=pymes_replay_smoke
database_url=postgres://pymes:pymes@127.0.0.1:55434/$database_name?sslmode=disable
organization_id=org_ops_replay_smoke
event_id=11111111-1111-4111-8111-111111111111

tenant_sql() {
  psql "$database_url" -X -v ON_ERROR_STOP=1 -v organization_id="$organization_id" -v event_id="$event_id" "$@"
}

cleanup() {
  psql "$admin_url" -X -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS pymes_replay_smoke WITH (FORCE)" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM
cleanup

psql "$admin_url" -X -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_replay_smoke" >/dev/null
for migration in "$root_dir"/db/migrations/*.sql; do
  psql "$database_url" -X -v ON_ERROR_STOP=1 -f "$migration" >/dev/null
done

tenant_sql >/dev/null <<'SQL'
INSERT INTO app.organizations (id, name, slug, status)
VALUES (:'organization_id', 'Operations replay smoke', 'operations-replay-smoke', 'ready');
BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
INSERT INTO app.outbox_dead_letters (
  id, org_id, topic, payload, payload_hash, idempotency_key,
  request_id, actor_ref, source_version, snapshot_digest,
  correlation_id, attempts, failure_code
) VALUES (
  :'event_id'::uuid, :'organization_id', 'OperationsReplaySmoke', '{}'::jsonb,
  repeat('0', 64), 'operations-replay-smoke',
  'request:operations-replay-smoke', 'system:smoke', 1, repeat('0', 64),
  'operations-replay-smoke',
  10, 'DELIVERY_FAILED'
);
UPDATE app.outbox_dead_letters
SET failed_at = '2026-01-01T00:00:00Z'
WHERE org_id = :'organization_id' AND id = :'event_id'::uuid;
COMMIT;
SQL

first_result=$(PYMES_DATABASE_URL="$database_url" \
PYMES_REPLAY_ORGANIZATION_ID="$organization_id" \
PYMES_REPLAY_EVENT_ID="$event_id" \
PYMES_REPLAY_FAILURE_CODE=DELIVERY_FAILED \
PYMES_REPLAY_ACTOR_REF=ops:smoke \
PYMES_REPLAY_CHANGE_REF=test:replay-smoke \
  "$root_dir/scripts/replay-dead-letter.sh")
if test "$first_result" != "replay-created"; then
  echo "unexpected first replay result: $first_result" >&2
  exit 1
fi

state=$(tenant_sql -At <<'SQL'
BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
SELECT
  (SELECT count(*) FROM app.outbox WHERE org_id = :'organization_id' AND id = :'event_id'::uuid)::text
  || ':' ||
  (SELECT count(*) FROM app.outbox_dead_letters WHERE org_id = :'organization_id' AND id = :'event_id'::uuid)::text
  || ':' ||
  COALESCE((SELECT attempts::text FROM app.outbox WHERE org_id = :'organization_id' AND id = :'event_id'::uuid), 'missing')
  || ':' ||
  (SELECT count(*) FROM app.outbox_dead_letter_replays WHERE org_id = :'organization_id' AND event_id = :'event_id'::uuid)::text
  || ':' ||
  COALESCE((
    SELECT request_id || ':' || actor_ref || ':' || source_version::text || ':' || snapshot_digest
    FROM app.outbox
    WHERE org_id = :'organization_id' AND id = :'event_id'::uuid
  ), 'missing');
COMMIT;
SQL
)
state=$(printf '%s\n' "$state" | awk '/^1:0:0:1:/ {print; exit}')
expected_state="1:0:0:1:request:operations-replay-smoke:system:smoke:1:$(printf '%064d' 0)"
if test "$state" != "$expected_state"; then
  echo "unexpected replay state: $state" >&2
  exit 1
fi

# Simulate normal outbox retention cleanup after processing. Audit alone must
# make a later operator retry an idempotent no-op.
tenant_sql >/dev/null <<'SQL'
BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
DELETE FROM app.outbox
WHERE org_id = :'organization_id' AND id = :'event_id'::uuid;
COMMIT;
SQL

second_result=$(PYMES_DATABASE_URL="$database_url" \
PYMES_REPLAY_ORGANIZATION_ID="$organization_id" \
PYMES_REPLAY_EVENT_ID="$event_id" \
PYMES_REPLAY_FAILURE_CODE=DELIVERY_FAILED \
PYMES_REPLAY_ACTOR_REF=ops:smoke \
PYMES_REPLAY_CHANGE_REF=test:replay-smoke \
  "$root_dir/scripts/replay-dead-letter.sh")
if test "$second_result" != "replay-noop"; then
  echo "unexpected idempotent replay result: $second_result" >&2
  exit 1
fi

# The audit key is tenant-composite and FORCE RLS prevents the first tenant
# from observing an identical event/failure timestamp owned by another one.
tenant_sql >/dev/null <<'SQL'
INSERT INTO app.organizations (id, name, slug, status)
VALUES ('org_ops_replay_other', 'Operations replay other', 'operations-replay-other', 'ready');
BEGIN;
SELECT set_config('app.org_id', 'org_ops_replay_other', true);
INSERT INTO app.outbox_dead_letter_replays (
  org_id, event_id, failed_at, failure_code, replayed_at, actor_ref, change_ref
) VALUES (
  'org_ops_replay_other', :'event_id'::uuid, '2026-01-01T00:00:00Z',
  'DELIVERY_FAILED', now(), 'ops:other', 'test:other'
);
COMMIT;
SQL

audit_count=$(tenant_sql -At <<'SQL'
BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
SELECT count(*) FROM app.outbox_dead_letter_replays
WHERE org_id = :'organization_id' AND event_id = :'event_id'::uuid;
COMMIT;
SQL
)
audit_count=$(printf '%s\n' "$audit_count" | awk '/^[0-9]+$/ {print; exit}')
if test "$audit_count" != "1"; then
  echo "replay audit is not idempotent or tenant-isolated: $audit_count rows" >&2
  exit 1
fi

rls_state=$(psql "$database_url" -X -At -v ON_ERROR_STOP=1 -c "
  SELECT relrowsecurity::text || ':' || relforcerowsecurity::text
  FROM pg_class
  WHERE oid = 'app.outbox_dead_letter_replays'::regclass
")
if test "$rls_state" != "true:true"; then
  echo "replay audit RLS is not forced: $rls_state" >&2
  exit 1
fi

if tenant_sql >/dev/null 2>&1 <<'SQL'
BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
UPDATE app.outbox_dead_letter_replays
SET change_ref = 'test:mutated'
WHERE org_id = :'organization_id' AND event_id = :'event_id'::uuid;
COMMIT;
SQL
then
  echo "immutable replay audit accepted an update" >&2
  exit 1
fi
