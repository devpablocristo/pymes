#!/usr/bin/env sh
set -eu

# This smoke test never writes to the active Pymes databases. It creates three
# isolated source databases, restores them into three new databases and removes
# only those six explicitly named databases when it finishes.
root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
temporary_dir=$(mktemp -d)
run_suffix="$(date -u +%Y%m%d%H%M%S)_$$"
container_suffix="$(date -u +%Y%m%d%H%M%S)-$$"

pymes_source_database="pymes_backup_source_$run_suffix"
pymes_restore_database="pymes_backup_restore_$run_suffix"
fiscal_source_database="fiscal_backup_source_$run_suffix"
fiscal_restore_database="fiscal_backup_restore_$run_suffix"
accounting_source_database="accounting_backup_source_$run_suffix"
accounting_restore_database="accounting_backup_restore_$run_suffix"

organization_id="org_backup_restore_$run_suffix"
purchase_id="purchase_backup_restore_$run_suffix"
fiscal_request_id="fiscal_backup_restore_$run_suffix"
source_accounting_container="pymes-v3-accounting-backup-source-$container_suffix"
source_fiscal_container="pymes-v3-fiscal-backup-source-$container_suffix"
restore_accounting_container="pymes-v3-accounting-backup-restore-$container_suffix"
restore_fiscal_container="pymes-v3-fiscal-backup-restore-$container_suffix"

pymes_user=
pymes_password=
fiscal_user=
fiscal_password=
accounting_admin_user=

for command in cut docker jq pg_dump pg_restore psql sha256sum; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done

assert_identifier() {
  value=$1
  label=$2
  case "$value" in
    ""|[0-9]*|*[!A-Za-z0-9_]*)
      echo "$label is not a safe PostgreSQL identifier" >&2
      exit 1
      ;;
  esac
}

assert_owned_database() {
  database=$1
  assert_identifier "$database" database
  case "$database" in
    pymes_backup_source_*|pymes_backup_restore_*|\
    fiscal_backup_source_*|fiscal_backup_restore_*|\
    accounting_backup_source_*|accounting_backup_restore_*) ;;
    *)
      echo "refusing to operate on non-smoke database $database" >&2
      exit 1
      ;;
  esac
}

assert_tenant_schema() {
  schema=$1
  suffix=${schema#tenant_pymes_}
  case "$schema" in
    tenant_pymes_????????????????????????????????) ;;
    *)
      echo "unsafe Accounting tenant schema $schema" >&2
      exit 1
      ;;
  esac
  case "$suffix" in
    *[!0-9a-f]*)
      echo "unsafe Accounting tenant schema $schema" >&2
      exit 1
      ;;
  esac
}

compose_value() {
  service=$1
  variable=$2
  value=$(docker compose config --format json |
    jq -er --arg service "$service" --arg variable "$variable" \
      '.services[$service].environment[$variable]
       | select(type == "string" and length > 0)')
  if test -z "$value"; then
    echo "$service does not expose required $variable" >&2
    exit 1
  fi
  printf '%s' "$value"
}

urlencode() {
  jq -nr --arg value "$1" '$value|@uri'
}

password_database_url() {
  user=$(urlencode "$1")
  password=$(urlencode "$2")
  host=$3
  port=$4
  database=$(urlencode "$5")
  printf 'postgres://%s:%s@%s:%s/%s?sslmode=disable' \
    "$user" "$password" "$host" "$port" "$database"
}

accounting_database_url() {
  role=$(urlencode "$1")
  host=$2
  port=$3
  database=$(urlencode "$4")
  options=${5:-}
  printf 'postgres://%s@%s:%s/%s?sslmode=disable%s' \
    "$role" "$host" "$port" "$database" "$options"
}

drop_database() {
  service=$1
  admin_user=$2
  database=$3
  assert_owned_database "$database"
  docker compose exec -T "$service" \
    psql -X -U "$admin_user" -d postgres -v ON_ERROR_STOP=1 \
      -c "DROP DATABASE IF EXISTS \"$database\" WITH (FORCE)" \
      >/dev/null
}

create_database() {
  service=$1
  admin_user=$2
  database=$3
  owner=$4
  assert_owned_database "$database"
  assert_identifier "$admin_user" admin_user
  assert_identifier "$owner" owner
  drop_database "$service" "$admin_user" "$database"
  docker compose exec -T "$service" \
    psql -X -U "$admin_user" -d postgres -v ON_ERROR_STOP=1 \
      -c "CREATE DATABASE \"$database\" OWNER \"$owner\"" \
      >/dev/null
}

configure_accounting_database() {
  database=$1
  assert_owned_database "$database"
  docker compose exec -T accounting-postgres \
    psql -X -U "$accounting_admin_user" -d postgres -v ON_ERROR_STOP=1 \
      -c "REVOKE ALL ON DATABASE \"$database\" FROM PUBLIC" \
      -c "GRANT CONNECT ON DATABASE \"$database\" TO accounting_runtime, accounting_control, accounting_migrate" \
      >/dev/null
}

apply_sql_migrations() {
  service=$1
  database_user=$2
  database=$3
  migrations_dir=$4
  for migration in "$migrations_dir"/*.sql; do
    test -f "$migration"
    docker compose exec -T "$service" \
      psql -X -q -U "$database_user" -d "$database" -v ON_ERROR_STOP=1 \
      <"$migration" >/dev/null
  done
}

apply_accounting_migrations() {
  database=$1
  database_url=$(accounting_database_url \
    accounting_migrate accounting-postgres 5432 "$database" \
    '&options=-c%20role%3Daccounting_owner')
  docker compose run --rm --no-deps \
    -e DATABASE_URL="$database_url" \
    accounting-migrate >/dev/null
}

sync_accounting_grants() {
  database=$1
  database_url=$(accounting_database_url \
    accounting_control accounting-postgres 5432 "$database" \
    '&options=-c%20role%3Daccounting_owner')
  docker compose run --rm --no-deps \
    -e ACCOUNTING_ADMIN_DATABASE_URL="$database_url" \
    -e ACCOUNTING_ADMIN_OPERATION=sync-runtime-grants \
    -e ACCOUNTING_RUNTIME_ROLE=accounting_runtime \
    -e ACCOUNTING_OWNER_ROLE=accounting_owner \
    accounting-admin >/dev/null
}

provision_accounting_organization() {
  database=$1
  database_url=$(accounting_database_url \
    accounting_control accounting-postgres 5432 "$database" \
    '&options=-c%20role%3Daccounting_owner')
  docker compose run --rm --no-deps \
    -e ACCOUNTING_ADMIN_DATABASE_URL="$database_url" \
    -e ACCOUNTING_ADMIN_OPERATION=provision \
    -e ACCOUNTING_ORGANIZATION_ID="$organization_id" \
    -e ACCOUNTING_ORGANIZATION_DISPLAY_NAME='Backup restore smoke' \
    -e ACCOUNTING_RUNTIME_ROLE=accounting_runtime \
    -e ACCOUNTING_OWNER_ROLE=accounting_owner \
    accounting-admin >/dev/null

  read_url=$(accounting_database_url \
    "$accounting_admin_user" 127.0.0.1 55436 "$database")
  provisioned_schema=$(psql "$read_url" -X -Atc \
    "SELECT schema_name
     FROM public.pymes_accounting_organizations
     WHERE organization_id='$organization_id'
       AND is_active=true")
  assert_tenant_schema "$provisioned_schema"
}

stop_container() {
  container=$1
  if docker inspect "$container" >/dev/null 2>&1; then
    docker stop --time 5 "$container" >/dev/null 2>&1 || true
  fi
}

wait_container_ready() {
  container=$1
  attempt=0
  while test "$attempt" -lt 60; do
    state=$(docker inspect --format '{{.State.Status}}' "$container" 2>/dev/null || true)
    health=$(docker inspect --format \
      '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' \
      "$container" 2>/dev/null || true)
    if test "$state" = running && {
      test "$health" = healthy || test "$health" = none
    }; then
      return 0
    fi
    if test "$state" = exited || test "$state" = dead; then
      break
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  echo "$container did not become ready" >&2
  docker logs --tail 80 "$container" >&2 || true
  return 1
}

launch_accounting() {
  container=$1
  database=$2
  database_url=$(accounting_database_url \
    accounting_runtime accounting-postgres 5432 "$database")
  docker compose run --rm --no-deps --name "$container" -d \
    -e ACCOUNTING_DATABASE_URL="$database_url" \
    -e PORT=8080 \
    accounting >/dev/null
  wait_container_ready "$container"
}

launch_fiscal() {
  container=$1
  database=$2
  database_url=$(password_database_url \
    "$fiscal_user" "$fiscal_password" fiscal-postgres 5432 "$database")
  docker compose run --rm --no-deps --name "$container" -d \
    -e FISCAL_DATABASE_URL="$database_url" \
    -e FISCAL_ADAPTER_MODE=mock \
    -e FISCAL_MOCK_SCENARIO=authorized \
    -e PORT=8080 \
    fiscal-fake >/dev/null
  wait_container_ready "$container"
}

run_worker_once() {
  database=$1
  accounting_container=$2
  fiscal_container=$3
  label=$4
  database_url=$(password_database_url \
    "$pymes_user" "$pymes_password" postgres 5432 "$database")
  if ! docker compose run --rm --no-deps \
    -e PYMES_DATABASE_URL="$database_url" \
    -e ACCOUNTING_URL="http://$accounting_container:8080" \
    -e FISCAL_ADAPTER_URL="http://$fiscal_container:8080" \
    -e PYMES_ALLOW_INSECURE_LOCAL_SERVICES=true \
    -e PYMES_WORKER_RUN_ONCE=true \
    -e PYMES_WORKER_HTTP_ADDR=:0 \
    worker >"$temporary_dir/worker-$label.log" 2>&1; then
    echo "one-shot worker failed during $label" >&2
    sed -n '1,120p' "$temporary_dir/worker-$label.log" >&2
    return 1
  fi
}

assert_relation() {
  database_url=$1
  relation=$2
  restored=$(psql "$database_url" -X -v ON_ERROR_STOP=1 -Atc \
    "SELECT to_regclass('$relation') IS NOT NULL")
  if test "$restored" != t; then
    echo "restored database is missing relation $relation" >&2
    exit 1
  fi
}

assert_equal() {
  label=$1
  expected=$2
  actual=$3
  if test "$actual" != "$expected"; then
    echo "$label mismatch: expected $expected, got $actual" >&2
    exit 1
  fi
}

stage() {
  printf 'backup/restore: %s\n' "$1"
}

cleanup() {
  trap - EXIT INT TERM
  stop_container "$source_accounting_container"
  stop_container "$source_fiscal_container"
  stop_container "$restore_accounting_container"
  stop_container "$restore_fiscal_container"

  if test -n "${pymes_user:-}" &&
    docker compose ps postgres >/dev/null 2>&1; then
    drop_database postgres "$pymes_user" "$pymes_source_database" \
      >/dev/null 2>&1 || true
    drop_database postgres "$pymes_user" "$pymes_restore_database" \
      >/dev/null 2>&1 || true
  fi
  if test -n "${fiscal_user:-}" &&
    docker compose ps fiscal-postgres >/dev/null 2>&1; then
    drop_database fiscal-postgres "$fiscal_user" "$fiscal_source_database" \
      >/dev/null 2>&1 || true
    drop_database fiscal-postgres "$fiscal_user" "$fiscal_restore_database" \
      >/dev/null 2>&1 || true
  fi
  if test -n "${accounting_admin_user:-}" &&
    docker compose ps accounting-postgres >/dev/null 2>&1; then
    drop_database accounting-postgres "$accounting_admin_user" \
      "$accounting_source_database" >/dev/null 2>&1 || true
    drop_database accounting-postgres "$accounting_admin_user" \
      "$accounting_restore_database" >/dev/null 2>&1 || true
  fi
  if test -d "$temporary_dir"; then
    find "$temporary_dir" -type f -exec chmod 600 {} \; 2>/dev/null || true
    rm -r "$temporary_dir"
  fi
}

cd "$root_dir"
trap cleanup EXIT INT TERM

# Initialize only the database engines and the local, passwordless Accounting
# roles. Application data lives exclusively in the smoke-owned databases.
stage 'initializing database engines and images'
docker compose up -d --wait postgres fiscal-postgres accounting-postgres
docker compose run --rm --no-deps accounting-roles >/dev/null
docker compose build worker fiscal-fake accounting accounting-admin \
  accounting-migrate >/dev/null

pymes_user=$(compose_value postgres POSTGRES_USER)
pymes_password=$(compose_value postgres POSTGRES_PASSWORD)
fiscal_user=$(compose_value fiscal-postgres POSTGRES_USER)
fiscal_password=$(compose_value fiscal-postgres POSTGRES_PASSWORD)
accounting_admin_user=$(compose_value accounting-postgres POSTGRES_USER)
assert_identifier "$pymes_user" pymes_user
assert_identifier "$fiscal_user" fiscal_user
assert_identifier "$accounting_admin_user" accounting_admin_user

stage 'creating isolated source databases'
create_database postgres "$pymes_user" "$pymes_source_database" "$pymes_user"
create_database fiscal-postgres "$fiscal_user" "$fiscal_source_database" "$fiscal_user"
create_database accounting-postgres "$accounting_admin_user" \
  "$accounting_source_database" accounting_owner
configure_accounting_database "$accounting_source_database"

# Every source migration is deliberately run twice. Pymes and Fiscal use
# intrinsically repeatable SQL; Accounting records applied versions and makes
# the second invocation a no-op.
stage 'applying every source migration twice'
apply_sql_migrations postgres "$pymes_user" "$pymes_source_database" \
  "$root_dir/db/migrations"
apply_sql_migrations postgres "$pymes_user" "$pymes_source_database" \
  "$root_dir/db/migrations"
apply_sql_migrations fiscal-postgres "$fiscal_user" "$fiscal_source_database" \
  "$root_dir/fiscal-adapter/migrations"
apply_sql_migrations fiscal-postgres "$fiscal_user" "$fiscal_source_database" \
  "$root_dir/fiscal-adapter/migrations"
apply_accounting_migrations "$accounting_source_database"
apply_accounting_migrations "$accounting_source_database"
stage 'provisioning a real Accounting tenant schema'
provision_accounting_organization "$accounting_source_database"

pymes_source_host_url=$(password_database_url \
  "$pymes_user" "$pymes_password" 127.0.0.1 55434 "$pymes_source_database")
fiscal_source_host_url=$(password_database_url \
  "$fiscal_user" "$fiscal_password" 127.0.0.1 55435 "$fiscal_source_database")
accounting_source_host_url=$(accounting_database_url \
  "$accounting_admin_user" 127.0.0.1 55436 "$accounting_source_database")

snapshot_digest=$(printf '%s' "$organization_id:$purchase_id:121.00:ARS" |
  sha256sum | cut -d ' ' -f 1)
outbox_payload=$(printf '{"purchase_id":"%s"}' "$purchase_id")
outbox_payload_hash=$(printf '%s' "$outbox_payload" |
  sha256sum | cut -d ' ' -f 1)
fiscal_payload_hash=$(printf '%s' "$organization_id:$fiscal_request_id" |
  sha256sum | cut -d ' ' -f 1)

stage 'seeding tenant-owned Pymes and Fiscal operational data'
psql "$pymes_source_host_url" -X -v ON_ERROR_STOP=1 \
  -v organization_id="$organization_id" \
  -v purchase_id="$purchase_id" \
  -v snapshot_digest="$snapshot_digest" \
  -v outbox_payload_hash="$outbox_payload_hash" \
  -v run_suffix="$run_suffix" >/dev/null <<'SQL'
BEGIN;
INSERT INTO app.organizations (id, name, slug, status)
VALUES (
  :'organization_id',
  'Backup restore smoke',
  'backup-restore-' || :'run_suffix',
  'ready'
);
INSERT INTO app.organization_provisioning (
  organization_id, accounting_status, fiscal_status
) VALUES (:'organization_id', 'ready', 'ready');
INSERT INTO app.purchases (
  id, org_id, supplier_ref, external_document_ref,
  amount, currency, status, snapshot_digest, correlation_id,
  issue_date, net_amount, exempt_amount, vat_breakdown, exchange_rate,
  request_id, actor_ref, source_version
) VALUES (
  :'purchase_id', :'organization_id', 'supplier:backup-restore',
  'supplier-document-' || :'run_suffix',
  121.00, 'ARS', 'confirmed', :'snapshot_digest',
  'correlation:backup-restore:' || :'run_suffix',
  current_date, 100.00, 0.00,
  '[{"rate":"21","base_amount":"100.00","tax_amount":"21.00"}]'::jsonb,
  NULL,
  'request:backup-restore:' || :'run_suffix',
  'worker:backup-restore',
  1
);
INSERT INTO app.outbox (
  id, org_id, topic, payload, payload_hash, idempotency_key,
  correlation_id, available_at, request_id, actor_ref,
  source_version, snapshot_digest
) VALUES (
  md5(:'organization_id' || ':purchase-posting')::uuid,
  :'organization_id',
  'PurchasePostingRequested',
  jsonb_build_object('purchase_id', :'purchase_id'),
  :'outbox_payload_hash',
  'backup-restore-outbox:' || :'run_suffix',
  'correlation:backup-restore:' || :'run_suffix',
  '2000-01-01T00:00:00Z',
  'request:backup-restore:' || :'run_suffix',
  'worker:backup-restore',
  1,
  :'snapshot_digest'
);
COMMIT;
SQL

psql "$fiscal_source_host_url" -X -v ON_ERROR_STOP=1 \
  -v organization_id="$organization_id" \
  -v request_id="$fiscal_request_id" \
  -v payload_hash="$fiscal_payload_hash" \
  -v run_suffix="$run_suffix" >/dev/null <<'SQL'
INSERT INTO fiscal.requests (
  organization_id, request_id, idempotency_key, payload_hash,
  request, result, correlation_id, actor_ref, delegated_actor_ref,
  workload_issuer, workload_subject, workload_request_id, workload_token_id
) VALUES (
  :'organization_id',
  :'request_id',
  'backup-restore-fiscal:' || :'run_suffix',
  :'payload_hash',
  jsonb_build_object(
    'request_id', :'request_id',
    'organization_id', :'organization_id',
    'document_type', 'FA',
    'voucher_number', 1
  ),
  jsonb_build_object(
    'request_id', :'request_id',
    'organization_id', :'organization_id',
    'status', 'authorized',
    'cae', 'backup-restore-cae',
    'correlation_id', 'correlation:backup-restore:' || :'run_suffix'
  ),
  'correlation:backup-restore:' || :'run_suffix',
  'worker:backup-restore',
  NULL,
  'pymes-v3',
  'worker:backup-restore',
  'request:backup-restore:' || :'run_suffix',
  'token:backup-restore:' || :'run_suffix'
);
SQL

# Pymes and Fiscal are captured before Accounting processes the command. The
# later Accounting backup represents the real "posted but response lost"
# recovery boundary across independently owned databases.
stage 'backing up Pymes and Fiscal before Accounting delivery'
PYMES_DATABASE_URL="$pymes_source_host_url" \
  SERVICE=pymes ./scripts/backup-postgres.sh "$temporary_dir/pymes.dump"
FISCAL_DATABASE_URL="$fiscal_source_host_url" \
  SERVICE=fiscal ./scripts/backup-postgres.sh "$temporary_dir/fiscal.dump"

launch_accounting "$source_accounting_container" "$accounting_source_database"
launch_fiscal "$source_fiscal_container" "$fiscal_source_database"
stage 'posting once so Accounting is ahead of the Pymes backup'
run_worker_once "$pymes_source_database" "$source_accounting_container" \
  "$source_fiscal_container" source

source_purchase_status=$(psql "$pymes_source_host_url" -X -Atc \
  "SELECT status FROM app.purchases WHERE org_id='$organization_id' AND id='$purchase_id'")
assert_equal 'source purchase status after one-shot worker' posted \
  "$source_purchase_status"

source_schema=$(psql "$accounting_source_host_url" -X -Atc \
  "SELECT schema_name FROM public.pymes_accounting_organizations WHERE organization_id='$organization_id'")
assert_tenant_schema "$source_schema"
source_journal_id=$(psql "$accounting_source_host_url" -X -Atc \
  "SELECT result->>'journal_entry_id' FROM public.pymes_accounting_commands WHERE organization_id='$organization_id' AND operation='posting'")
if test -z "$source_journal_id"; then
  echo "source accounting command did not create a journal entry" >&2
  exit 1
fi
source_command_witness=$(psql "$accounting_source_host_url" -X -Atc \
  "SELECT (result->'source'->>'id') || '|' || (result->>'journal_entry_id')
   FROM public.pymes_accounting_commands
   WHERE organization_id='$organization_id' AND operation='posting'")
assert_equal 'source Accounting command witness' \
  "$purchase_id|$source_journal_id" "$source_command_witness"
source_journal_count=$(psql "$accounting_source_host_url" -X -Atc \
  "SELECT count(*) FROM \"$source_schema\".journal_entries
   WHERE id='$source_journal_id' AND status='POSTED'")
assert_equal 'source journal count' 1 "$source_journal_count"

stop_container "$source_accounting_container"
stop_container "$source_fiscal_container"
stage 'backing up Accounting after the posting committed'
ACCOUNTING_DATABASE_URL="$accounting_source_host_url" \
  SERVICE=accounting ./scripts/backup-postgres.sh "$temporary_dir/accounting.dump"

for dump in "$temporary_dir/pymes.dump" "$temporary_dir/fiscal.dump" \
  "$temporary_dir/accounting.dump"; do
  test -s "$dump"
  sha256sum "$dump" >>"$temporary_dir/checksums.sha256"
done

stage 'creating and restoring three isolated destinations'
create_database postgres "$pymes_user" "$pymes_restore_database" "$pymes_user"
create_database fiscal-postgres "$fiscal_user" "$fiscal_restore_database" "$fiscal_user"
create_database accounting-postgres "$accounting_admin_user" \
  "$accounting_restore_database" accounting_owner
configure_accounting_database "$accounting_restore_database"

pymes_restore_host_url=$(password_database_url \
  "$pymes_user" "$pymes_password" 127.0.0.1 55434 "$pymes_restore_database")
fiscal_restore_host_url=$(password_database_url \
  "$fiscal_user" "$fiscal_password" 127.0.0.1 55435 "$fiscal_restore_database")
accounting_restore_host_url=$(accounting_database_url \
  accounting_control 127.0.0.1 55436 "$accounting_restore_database" \
  '&options=-c%20role%3Daccounting_owner')
accounting_restore_read_url=$(accounting_database_url \
  "$accounting_admin_user" 127.0.0.1 55436 "$accounting_restore_database")

PYMES_RESTORE_DATABASE_URL="$pymes_restore_host_url" \
  SERVICE=pymes ./scripts/restore-postgres.sh "$temporary_dir/pymes.dump"
FISCAL_RESTORE_DATABASE_URL="$fiscal_restore_host_url" \
  SERVICE=fiscal ./scripts/restore-postgres.sh "$temporary_dir/fiscal.dump"
ACCOUNTING_RESTORE_DATABASE_URL="$accounting_restore_host_url" \
  SERVICE=accounting ./scripts/restore-postgres.sh "$temporary_dir/accounting.dump"

stage 'reapplying every restored migration twice'
apply_sql_migrations postgres "$pymes_user" "$pymes_restore_database" \
  "$root_dir/db/migrations"
apply_sql_migrations postgres "$pymes_user" "$pymes_restore_database" \
  "$root_dir/db/migrations"
apply_sql_migrations fiscal-postgres "$fiscal_user" "$fiscal_restore_database" \
  "$root_dir/fiscal-adapter/migrations"
apply_sql_migrations fiscal-postgres "$fiscal_user" "$fiscal_restore_database" \
  "$root_dir/fiscal-adapter/migrations"
apply_accounting_migrations "$accounting_restore_database"
apply_accounting_migrations "$accounting_restore_database"
sync_accounting_grants "$accounting_restore_database"

stage 'validating restored tenant data and Accounting schemas'
assert_relation "$pymes_restore_host_url" app.organizations
assert_relation "$pymes_restore_host_url" app.accounting_failures
assert_relation "$fiscal_restore_host_url" fiscal.requests
assert_relation "$accounting_restore_read_url" \
  public.pymes_accounting_organizations
assert_relation "$accounting_restore_read_url" \
  public.pymes_headless_schema_migrations

restored_pymes_witness=$(psql "$pymes_restore_host_url" -X -Atc \
  "SELECT o.status || '|' || p.status || '|' || (p.journal_entry_id IS NULL)::text || '|' || (x.published_at IS NULL)::text
   FROM app.organizations o
   JOIN app.purchases p ON p.org_id=o.id
   JOIN app.outbox x ON x.org_id=o.id
   WHERE o.id='$organization_id' AND p.id='$purchase_id'
     AND x.topic='PurchasePostingRequested'")
assert_equal 'restored Pymes tenant witness before recovery' \
  'ready|confirmed|true|true' "$restored_pymes_witness"

restored_fiscal_witness=$(psql "$fiscal_restore_host_url" -X -Atc \
  "SELECT result->>'status' FROM fiscal.requests WHERE organization_id='$organization_id' AND request_id='$fiscal_request_id'")
assert_equal 'restored Fiscal tenant witness' authorized \
  "$restored_fiscal_witness"

restored_schema=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT schema_name FROM public.pymes_accounting_organizations WHERE organization_id='$organization_id'")
assert_tenant_schema "$restored_schema"
assert_equal 'restored Accounting tenant schema mapping' "$source_schema" \
  "$restored_schema"
assert_relation "$accounting_restore_read_url" "$restored_schema.accounts"
assert_relation "$accounting_restore_read_url" \
  "$restored_schema.journal_entries"
assert_relation "$accounting_restore_read_url" \
  "$restored_schema.pymes_open_items"

restored_accounts=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM \"$restored_schema\".accounts")
restored_periods=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM \"$restored_schema\".pymes_periods")
restored_commands=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM public.pymes_accounting_commands WHERE organization_id='$organization_id' AND operation='posting'")
restored_journals=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM \"$restored_schema\".journal_entries
   WHERE id='$source_journal_id' AND status='POSTED'")
test "$restored_accounts" -ge 12
test "$restored_periods" -ge 1
assert_equal 'restored Accounting command count before recovery' 1 \
  "$restored_commands"
assert_equal 'restored Accounting journal count before recovery' 1 \
  "$restored_journals"

expected_accounting_migrations=$(find \
  "${ACCOUNTING_BUILD_CONTEXT:-../../open-accounting}/migrations/pymes-accounting" \
  -type f -name '*.up.sql' | wc -l | tr -d ' ')
restored_accounting_migrations=$(psql "$accounting_restore_read_url" -X -Atc \
  'SELECT count(*) FROM public.pymes_headless_schema_migrations')
assert_equal 'restored Accounting migration count' \
  "$expected_accounting_migrations" "$restored_accounting_migrations"

stage 'recovering the lost response with a one-shot worker'
launch_accounting "$restore_accounting_container" "$accounting_restore_database"
launch_fiscal "$restore_fiscal_container" "$fiscal_restore_database"
run_worker_once "$pymes_restore_database" "$restore_accounting_container" \
  "$restore_fiscal_container" restore-first

restored_purchase=$(psql "$pymes_restore_host_url" -X -Atc \
  "SELECT status || '|' || journal_entry_id || '|' || (open_item_id IS NOT NULL)::text
   FROM app.purchases WHERE org_id='$organization_id' AND id='$purchase_id'")
assert_equal 'restored purchase after one-shot recovery' \
  "posted|$source_journal_id|true" "$restored_purchase"
restored_pending=$(psql "$pymes_restore_host_url" -X -Atc \
  "SELECT count(*) FROM app.outbox WHERE org_id='$organization_id' AND published_at IS NULL")
assert_equal 'restored unpublished outbox count' 0 "$restored_pending"
restored_inbox=$(psql "$pymes_restore_host_url" -X -Atc \
  "SELECT count(*) FROM app.service_response_inbox WHERE org_id='$organization_id' AND service='accounting'")
assert_equal 'restored accounting response inbox count' 1 "$restored_inbox"

commands_after_first=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM public.pymes_accounting_commands WHERE organization_id='$organization_id' AND operation='posting'")
journals_after_first=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM \"$restored_schema\".journal_entries
   WHERE id='$source_journal_id' AND status='POSTED'")
assert_equal 'Accounting command count after recovery' 1 \
  "$commands_after_first"
assert_equal 'Accounting journal count after recovery' 1 \
  "$journals_after_first"

# A second one-shot execution proves that recovery itself is replay-safe.
stage 'replaying the one-shot worker to prove idempotence'
run_worker_once "$pymes_restore_database" "$restore_accounting_container" \
  "$restore_fiscal_container" restore-second
commands_after_second=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM public.pymes_accounting_commands WHERE organization_id='$organization_id' AND operation='posting'")
journals_after_second=$(psql "$accounting_restore_read_url" -X -Atc \
  "SELECT count(*) FROM \"$restored_schema\".journal_entries
   WHERE id='$source_journal_id' AND status='POSTED'")
assert_equal 'Accounting command count after replay' \
  "$commands_after_first" "$commands_after_second"
assert_equal 'Accounting journal count after replay' \
  "$journals_after_first" "$journals_after_second"

echo "backup/restore smoke passed: three isolated databases, tenant schemas, repeatable migrations and idempotent one-shot worker recovery"
