#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
temporary_dir=$(mktemp -d)
pymes_admin='postgres://pymes:pymes@127.0.0.1:55434/postgres?sslmode=disable'
fiscal_admin='postgres://fiscal:fiscal@127.0.0.1:55435/postgres?sslmode=disable'
accounting_admin='postgres://accounting:accounting@127.0.0.1:55436/postgres?sslmode=disable'

drop_database() {
  admin_url=$1
  database=$2
  psql "$admin_url" -v ON_ERROR_STOP=1 -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$database' AND pid <> pg_backend_pid()" >/dev/null
  psql "$admin_url" -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS $database" >/dev/null
}

create_database() {
  admin_url=$1
  database=$2
  drop_database "$admin_url" "$database"
  psql "$admin_url" -v ON_ERROR_STOP=1 -c "CREATE DATABASE $database" >/dev/null
}

assert_relation() {
  database_url=$1
  relation=$2
  restored=$(psql "$database_url" -v ON_ERROR_STOP=1 -Atc "SELECT to_regclass('$relation') IS NOT NULL")
  if test "$restored" != "t"; then
    echo "restored database is missing relation $relation" >&2
    exit 1
  fi
}

cleanup() {
  drop_database "$pymes_admin" pymes_restore_smoke
  drop_database "$fiscal_admin" fiscal_restore_smoke
  drop_database "$accounting_admin" accounting_restore_smoke
  rm -rf "$temporary_dir"
}
trap cleanup EXIT INT TERM

cd "$root_dir"
docker compose up -d --wait postgres fiscal-postgres accounting-postgres

PYMES_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable' \
SERVICE=pymes ./scripts/backup-postgres.sh "$temporary_dir/pymes.dump"
FISCAL_DATABASE_URL='postgres://fiscal:fiscal@127.0.0.1:55435/pymes_fiscal?sslmode=disable' \
SERVICE=fiscal ./scripts/backup-postgres.sh "$temporary_dir/fiscal.dump"
ACCOUNTING_DATABASE_URL='postgres://accounting:accounting@127.0.0.1:55436/pymes_accounting?sslmode=disable' \
SERVICE=accounting ./scripts/backup-postgres.sh "$temporary_dir/accounting.dump"

create_database "$pymes_admin" pymes_restore_smoke
create_database "$fiscal_admin" fiscal_restore_smoke
create_database "$accounting_admin" accounting_restore_smoke

PYMES_RESTORE_DATABASE_URL='postgres://pymes:pymes@127.0.0.1:55434/pymes_restore_smoke?sslmode=disable' \
SERVICE=pymes ./scripts/restore-postgres.sh "$temporary_dir/pymes.dump"
FISCAL_RESTORE_DATABASE_URL='postgres://fiscal:fiscal@127.0.0.1:55435/fiscal_restore_smoke?sslmode=disable' \
SERVICE=fiscal ./scripts/restore-postgres.sh "$temporary_dir/fiscal.dump"
ACCOUNTING_RESTORE_DATABASE_URL='postgres://accounting:accounting@127.0.0.1:55436/accounting_restore_smoke?sslmode=disable' \
SERVICE=accounting ./scripts/restore-postgres.sh "$temporary_dir/accounting.dump"

assert_relation 'postgres://pymes:pymes@127.0.0.1:55434/pymes_restore_smoke?sslmode=disable' app.organizations
assert_relation 'postgres://fiscal:fiscal@127.0.0.1:55435/fiscal_restore_smoke?sslmode=disable' fiscal.requests
assert_relation 'postgres://accounting:accounting@127.0.0.1:55436/accounting_restore_smoke?sslmode=disable' public.tenants
