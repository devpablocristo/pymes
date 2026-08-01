#!/usr/bin/env sh
set -eu
root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"
pymes_test_url=${PYMES_DATABASE_TEST_URL:-postgres://pymes:pymes@127.0.0.1:55434/pymes_v3?sslmode=disable}
fiscal_test_url=${FISCAL_DATABASE_TEST_URL:-postgres://fiscal:fiscal@127.0.0.1:55435/pymes_fiscal?sslmode=disable}
accounting_test_url=${ACCOUNTING_DATABASE_TEST_URL:-postgres://accounting:accounting@127.0.0.1:55436/pymes_accounting?sslmode=disable}
docker compose stop worker api fiscal-fake accounting accounting-admin
docker compose up -d --wait postgres fiscal-postgres accounting-postgres

# Immutable inbox/audit tables intentionally cannot be truncated. Recreate
# only the three disposable Docker databases so every integration run starts
# from authoritative empty state, then apply each migrator twice below to
# prove migration idempotence.
docker compose run --rm --no-deps -e PGPASSWORD=pymes postgres \
  psql -h postgres -U pymes -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS pymes_v3 WITH (FORCE)"
docker compose run --rm --no-deps -e PGPASSWORD=pymes postgres \
  psql -h postgres -U pymes -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_v3 OWNER pymes"
docker compose run --rm --no-deps -e PGPASSWORD=fiscal fiscal-postgres \
  psql -h fiscal-postgres -U fiscal -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS pymes_fiscal WITH (FORCE)"
docker compose run --rm --no-deps -e PGPASSWORD=fiscal fiscal-postgres \
  psql -h fiscal-postgres -U fiscal -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_fiscal OWNER fiscal"
docker compose run --rm --no-deps -e PGPASSWORD=accounting accounting-postgres \
  psql -h accounting-postgres -U accounting -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS pymes_accounting WITH (FORCE)"
docker compose run --rm --no-deps -e PGPASSWORD=accounting accounting-postgres \
  psql -h accounting-postgres -U accounting -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE DATABASE pymes_accounting OWNER accounting"

for file in db/migrations/*.sql; do
  docker compose exec -T postgres psql -U pymes -d pymes_v3 -v ON_ERROR_STOP=1 -f "/migrations/$(basename "$file")"
  docker compose exec -T postgres psql -U pymes -d pymes_v3 -v ON_ERROR_STOP=1 -f "/migrations/$(basename "$file")"
done
docker compose run --rm fiscal-migrate
docker compose run --rm fiscal-migrate
docker compose build accounting-migrate
docker compose run --rm accounting-roles
docker compose run --rm --no-deps accounting-migrate
docker compose run --rm --no-deps accounting-migrate
docker compose run --rm --no-deps accounting-grants
docker compose exec -T postgres \
  psql -U pymes -d pymes_v3 \
  <db/tests/rls-isolation.sql
cd backend
PYMES_DATABASE_TEST_URL="$pymes_test_url" go test ./internal/commerce
PYMES_DATABASE_TEST_URL="$pymes_test_url" go test ./internal/identity
PYMES_DATABASE_TEST_URL="$pymes_test_url" go test ./internal/postgres
PYMES_DATABASE_TEST_URL="$pymes_test_url" go test ./internal/organization
PYMES_DATABASE_TEST_URL="$pymes_test_url" go test ./internal/notifications
PYMES_DATABASE_TEST_URL="$pymes_test_url" go test ./internal/scheduling \
  -run TestBookingStatusCustomizationPreservesTenantAndLifecycleInvariants \
  -count=1
cd ../fiscal-adapter
FISCAL_DATABASE_TEST_URL="$fiscal_test_url" npm run test:postgres
accounting_dir=${ACCOUNTING_BUILD_CONTEXT:-../../open-accounting}
case "$accounting_dir" in
  /*) cd "$accounting_dir" ;;
  *) cd "$root_dir/$accounting_dir" ;;
esac
ACCOUNTING_DATABASE_TEST_URL="$accounting_test_url" \
  go test -race -tags=integration ./internal/pymesaccounting -count=1
