#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"

organization_id=${PYMES_NOTIFICATIONS_E2E_ORGANIZATION:-notifications-e2e-$$}
api_port=${PYMES_API_PORT:-18080}
pergo_port=${PERGO_FAKE_PORT:-18086}
database_url=${PYMES_DATABASE_URL:-}
default_database_url='postgresql://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable'

psql_app() {
  if [ -n "$database_url" ]; then
    psql "$database_url" "$@"
    return
  fi
  if command -v psql >/dev/null 2>&1; then
    psql "$default_database_url" "$@"
    return
  fi
  docker compose exec -T postgres psql -U pymes -d pymes_v3 "$@"
}

set_scenario() {
  curl -fsS \
    -H 'Content-Type: application/json' \
    -d "{\"scenario\":\"$1\"}" \
    "http://127.0.0.1:$pergo_port/__test/scenario" >/dev/null
}

queue_notification() {
  notification_id=$1
  psql_app \
    -v ON_ERROR_STOP=1 \
    -v organization_id="$organization_id" \
    -v notification_id="$notification_id" <<'SQL' >/dev/null
INSERT INTO app.organizations(id,name,slug,status)
VALUES(:'organization_id','Notifications E2E',:'organization_id','ready')
ON CONFLICT (id) DO UPDATE SET status='ready';

BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
INSERT INTO app.notification_settings(
  org_id,whatsapp_enabled,pergo_channel,pergo_sender_identity
)
VALUES(
  :'organization_id',true,'whatsapp_mock','mock:' || :'organization_id'
)
ON CONFLICT (org_id) DO UPDATE
SET whatsapp_enabled=true,
    pergo_channel=EXCLUDED.pergo_channel,
    pergo_sender_identity=EXCLUDED.pergo_sender_identity,
    updated_at=now();
INSERT INTO app.organization_feature_flags(
  org_id,whatsapp_enabled,updated_by
)
VALUES(:'organization_id',true,'e2e')
ON CONFLICT (org_id) DO UPDATE
SET whatsapp_enabled=true,updated_at=now(),updated_by='e2e';
INSERT INTO app.notifications(
  org_id,id,kind,aggregate_type,aggregate_id,recipient_e164,template_name,
  template_version,locale,variables,body,delivery_channel,sender_identity,
  send_at,status,idempotency_key,correlation_id,request_id,actor_ref,
  source_version,snapshot_digest
)
VALUES(
  :'organization_id',:'notification_id','reminder','booking',
  'booking-e2e','+5491112345678','booking.reminder',1,'es_AR',
  '{"customer":"E2E"}'::jsonb,'Recordatorio de turno',
  'whatsapp_mock','mock:' || :'organization_id',now(),'pending',
  'notification:' || :'notification_id',
  'correlation:' || :'notification_id',
  'request:' || :'notification_id','system:scheduling',1,repeat('b',64)
)
ON CONFLICT (org_id,id) DO NOTHING;
INSERT INTO app.outbox(
  id,org_id,topic,payload,payload_hash,idempotency_key,request_id,actor_ref,
  source_version,snapshot_digest,correlation_id,available_at
)
VALUES(
  (
    substr(md5(:'organization_id' || ':' || :'notification_id'),1,8) || '-' ||
    substr(md5(:'organization_id' || ':' || :'notification_id'),9,4) || '-' ||
    substr(md5(:'organization_id' || ':' || :'notification_id'),13,4) || '-' ||
    substr(md5(:'organization_id' || ':' || :'notification_id'),17,4) || '-' ||
    substr(md5(:'organization_id' || ':' || :'notification_id'),21,12)
  )::uuid,
  :'organization_id','NotificationRequested',
  jsonb_build_object('notification_id',:'notification_id'),repeat('a',64),
  'notification:' || :'notification_id',
  'request:' || :'notification_id','system:scheduling',1,repeat('b',64),
  'correlation:' || :'notification_id',now()
)
ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING;
COMMIT;
SQL
}

queue_scheduling_projection() {
  event_id=$1
  booking_id=$2
  psql_app \
    -v ON_ERROR_STOP=1 \
    -v organization_id="$organization_id" \
    -v event_id="$event_id" \
    -v booking_id="$booking_id" <<'SQL' >/dev/null
INSERT INTO app.organizations(id,name,slug,status)
VALUES(:'organization_id','Notifications E2E',:'organization_id','ready')
ON CONFLICT (id) DO UPDATE SET status='ready';

BEGIN;
SELECT set_config('app.org_id', :'organization_id', true);
INSERT INTO app.notification_settings(
  org_id,whatsapp_enabled,pergo_channel,pergo_sender_identity
)
VALUES(
  :'organization_id',true,'whatsapp_mock','mock:' || :'organization_id'
)
ON CONFLICT (org_id) DO UPDATE
SET whatsapp_enabled=true,
    pergo_channel=EXCLUDED.pergo_channel,
    pergo_sender_identity=EXCLUDED.pergo_sender_identity,
    updated_at=now();
INSERT INTO app.organization_feature_flags(
  org_id,whatsapp_enabled,updated_by
)
VALUES(:'organization_id',true,'e2e')
ON CONFLICT (org_id) DO UPDATE
SET whatsapp_enabled=true,updated_at=now(),updated_by='e2e';
INSERT INTO app.outbox(
  id,org_id,topic,payload,payload_hash,idempotency_key,request_id,actor_ref,
  source_version,snapshot_digest,correlation_id,available_at
)
VALUES(
  :'event_id'::uuid,
  :'organization_id',
  'NotificationRequested',
  jsonb_build_object(
    'trigger','BookingConfirmed',
    'aggregate_type','booking',
    'aggregate_id',:'booking_id',
    'booking_id',:'booking_id',
    'recipient_e164','+5491112345678',
    'customer_name','Ada',
    'service_name','Consulta',
    'start_at','2026-08-03T15:00:00Z',
    'end_at','2026-08-03T16:00:00Z',
    'timezone','America/Argentina/Buenos_Aires'
  ),
  repeat('c',64),
  'scheduling:NotificationRequested:' || :'booking_id' || ':source:1',
  'request:projection',
  'system:scheduling',
  1,
  repeat('d',64),
  'correlation:projection',
  now()
)
ON CONFLICT (org_id,topic,idempotency_key) DO NOTHING;
COMMIT;
SQL
}

notification_status() {
  notification_id=$1
  psql_app \
    -qAt -v ON_ERROR_STOP=1 \
    -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT status FROM app.notifications WHERE org_id='$organization_id' AND id='$notification_id';" |
    tail -n 1
}

wait_status() {
  notification_id=$1
  expected=$2
  attempts=0
  while [ "$attempts" -lt 80 ]; do
    actual=$(notification_status "$notification_id")
    if [ "$actual" = "$expected" ]; then
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 0.25
  done
  echo "notification $notification_id did not reach $expected; status=$actual" >&2
  return 1
}

wait_published() {
  notification_id=$1
  attempts=0
  while [ "$attempts" -lt 80 ]; do
    published=$(psql_app \
      -qAt -v ON_ERROR_STOP=1 \
      -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT (published_at IS NOT NULL)::text FROM app.outbox WHERE org_id='$organization_id' AND topic='NotificationRequested' AND idempotency_key='notification:$notification_id';" |
      tail -n 1)
    if [ "$published" = "true" ]; then
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 0.25
  done
  echo "notification outbox $notification_id was not published" >&2
  return 1
}

set_scenario success
queue_notification notification-success
wait_status notification-success sent
wait_published notification-success

projection_event_id=$(
  psql_app -qAt -v ON_ERROR_STOP=1 -v organization_id="$organization_id" <<'SQL'
SELECT (
  substr(md5(:'organization_id' || ':scheduling-projection'),1,8) || '-' ||
  substr(md5(:'organization_id' || ':scheduling-projection'),9,4) || '-' ||
  '4' || substr(md5(:'organization_id' || ':scheduling-projection'),14,3) || '-' ||
  '8' || substr(md5(:'organization_id' || ':scheduling-projection'),18,3) || '-' ||
  substr(md5(:'organization_id' || ':scheduling-projection'),21,12)
)::uuid;
SQL
)
projection_notification_id="scheduling:$projection_event_id"
queue_scheduling_projection "$projection_event_id" booking-projection-e2e
wait_status "$projection_notification_id" sent
wait_published_idempotency=$(
  psql_app \
    -qAt -v ON_ERROR_STOP=1 \
    -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT count(*) || ':' || count(*) FILTER (WHERE published_at IS NOT NULL) FROM app.outbox WHERE org_id='$organization_id' AND topic='NotificationRequested' AND idempotency_key='scheduling:NotificationRequested:booking-projection-e2e:source:1';" |
    tail -n 1
)
projection_count=$(
  psql_app \
    -qAt -v ON_ERROR_STOP=1 \
    -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT count(*) FROM app.notifications WHERE org_id='$organization_id' AND idempotency_key='scheduling:NotificationRequested:booking-projection-e2e:source:1';" |
    tail -n 1
)
projection_route=$(
  psql_app \
    -qAt -v ON_ERROR_STOP=1 \
    -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT delivery_channel || ':' || sender_identity FROM app.notifications WHERE org_id='$organization_id' AND idempotency_key='scheduling:NotificationRequested:booking-projection-e2e:source:1';" |
    tail -n 1
)
if [ "$wait_published_idempotency" != "1:1" ] ||
  [ "$projection_count" != "1" ] ||
  [ "$projection_route" != "whatsapp_mock:mock:$organization_id" ]; then
  echo "scheduling projection did not converge: outbox=$wait_published_idempotency notifications=$projection_count route=$projection_route" >&2
  exit 1
fi
projection_stats=$(curl -fsS \
  "http://127.0.0.1:$pergo_port/__test/messages/$organization_id/$projection_notification_id")
case "$projection_stats" in
  *"\"channel\":\"whatsapp_mock\""*"\"sender_identity\":\"mock:$organization_id\""*) ;;
  *)
    echo "PerGo did not receive the tenant route: $projection_stats" >&2
    exit 1
    ;;
esac

set_scenario timeout_after
queue_notification notification-timeout-after
wait_status notification-timeout-after sent
wait_published notification-timeout-after
stats=$(curl -fsS "http://127.0.0.1:$pergo_port/__test/messages/$organization_id/notification-timeout-after")
case "$stats" in
  *'"requests":1'*) ;;
  *)
    echo "response-loss retry duplicated PerGo request: $stats" >&2
    exit 1
    ;;
esac

set_scenario unavailable
queue_notification notification-recovery
wait_status notification-recovery uncertain
set_scenario success
wait_status notification-recovery sent
wait_published notification-recovery

set_scenario timeout_before
queue_notification notification-timeout-before
wait_status notification-timeout-before uncertain
set_scenario success
wait_status notification-timeout-before sent
wait_published notification-timeout-before

before=$(psql_app \
  -qAt -v ON_ERROR_STOP=1 \
  -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT count(*) FROM app.notification_webhook_inbox inbox JOIN app.notifications notification ON notification.org_id=inbox.org_id AND notification.external_message_id=inbox.message_id WHERE notification.org_id='$organization_id' AND notification.id='notification-success';" |
  tail -n 1)
curl -fsS -X POST \
  "http://127.0.0.1:$pergo_port/__test/replay/$organization_id/notification-success" >/dev/null
after=$(psql_app \
  -qAt -v ON_ERROR_STOP=1 \
  -c "SELECT set_config('app.org_id', '$organization_id', false); SELECT count(*) FROM app.notification_webhook_inbox inbox JOIN app.notifications notification ON notification.org_id=inbox.org_id AND notification.external_message_id=inbox.message_id WHERE notification.org_id='$organization_id' AND notification.id='notification-success';" |
  tail -n 1)
if [ "$before" != "1" ] || [ "$after" != "1" ]; then
  echo "duplicate webhook was not deduplicated: before=$before after=$after" >&2
  exit 1
fi

invalid_status=$(curl -sS -o /dev/null -w '%{http_code}' \
  -X POST \
  -H 'Content-Type: application/json' \
  -H 'X-PerGo-Signature: t=1,v1=bad' \
  -d '{"event":"sent","trace_id":"pymes.v1.invalid.invalid","message_id":"invalid","channel":"whatsapp_mock","timestamp":"2026-08-01T00:00:00Z","workspace_id":"pymes-local"}' \
  "http://127.0.0.1:$api_port/api/v1/webhooks/pergo")
if [ "$invalid_status" != "401" ]; then
  echo "invalid PerGo webhook status=$invalid_status, want 401" >&2
  exit 1
fi

echo "notifications e2e: scheduling projection, success, response loss, outage recovery and webhook dedup verified"
